// vm-web creates exe.dev VMs that run claude with Remote Control, sends them
// prompts, backs up their transcripts and deletes them. It does what the vm
// script does, from a VM rather than a laptop, and with as little access as it
// can get by with:
//
//   - It reaches exe.dev through an HTTP-proxy integration (-exe-api) that
//     injects a token allowed only ls, new and rm. The token never touches
//     this VM.
//   - It has no SSH access to the VMs it makes. Each one runs vm-child as a
//     service that polls this app for jobs through a peer integration
//     (-ctl-url). exe.dev stamps those requests with the calling VM's name in
//     X-Exedev-Source-Vm, which can't be forged, and they arrive on their own
//     port (-agent-addr).
//
// The UI port trusts the X-ExeDev-Email header set by exe.dev's HTTPS proxy,
// so serve it only through that proxy, with the VM's sharing kept private.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

//go:embed index.html
var static embed.FS

var (
	addr      = flag.String("addr", ":8000", "address for the UI, behind exe.dev's proxy")
	agentAddr = flag.String("agent-addr", ":8001", "address for the VMs' agents, behind the peer integration")
	exeAPI    = flag.String("exe-api", "https://spawn-api.int.exe.xyz", "HTTP-proxy integration in front of https://exe.dev")
	ctlURL    = flag.String("ctl-url", "https://spawn.int.exe.xyz", "peer integration the VMs reach this app's -agent-addr through")
	tag       = flag.String("tag", "spawn", "tag for new VMs; the peer integration is attached to it")
	repoDir   = flag.String("repo", "", "the dream checkout with vm-child and the notes (default: .. from the working directory)")
	backupDir = flag.String("backups", "", "where transcripts go (default ~/vm-transcripts)")
	stateFile = flag.String("state", "", "the list of VMs this app manages (default ~/.config/vm/state.json)")
	allowed   = flag.String("allow", "dan@buildbuddy.io,dstowell@gmail.com", "comma-separated emails allowed to use the app")
)

// exe.dev's naming rule, as the vm script checks it.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

var loginURLRE = regexp.MustCompile(`(?m)^vm: sign in at (\S+)$`)

const (
	maxLog         = 256 << 10
	pickupTimeout  = 15 * time.Minute // for a job to reach its VM's agent
	runTimeout     = 45 * time.Minute // for an agent to finish a job
	agentFresh     = 2 * time.Minute  // an agent polled within this is online
	backupInterval = time.Hour
)

// The files vm-child setup needs, besides the generated config and secrets.
var bundleFiles = []string{
	"vm-CLAUDE.md", "vm-CLAUDE-buildbuddy.md", "vm-coordinator.md",
	"vm-coordinator-shelley.md", "vm-dispatcher.md", "vm-agents/*.md",
}

// managed is a VM this app made or adopted. Only these get jobs, secrets and
// backups.
type managed struct {
	Kind       string    `json:"kind"` // classic or dispatcher
	Created    time.Time `json:"created"`
	LastBackup time.Time `json:"last_backup,omitzero"`
}

type job struct {
	ID       int64     `json:"id"`
	Kind     string    `json:"kind"` // start, prompt, setup, backup, rm
	VM       string    `json:"vm"`
	State    string    `json:"state"` // queued, running, login, ok, failed
	Auto     bool      `json:"auto,omitempty"`
	LoginURL string    `json:"login_url,omitempty"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended,omitzero"`
	Prompt   string    `json:"prompt,omitempty"` // so a login or retry can resume a start
	Log      string    `json:"log"`

	ctlLog   bytes.Buffer // this app's part of the log
	agentLog string       // the agent's latest report
	steps    [][]string   // vm-child command lines for the agent
	queued   time.Time
	picked   time.Time
	reported bool // the agent said it's done with the steps
}

// storedJob is a job as jobs.json keeps it, so a restart of this app
// doesn't lose queued work, prompts or logs.
type storedJob struct {
	ID       int64      `json:"id"`
	Kind     string     `json:"kind"`
	VM       string     `json:"vm"`
	State    string     `json:"state"`
	Auto     bool       `json:"auto,omitempty"`
	LoginURL string     `json:"login_url,omitempty"`
	Started  time.Time  `json:"started"`
	Ended    time.Time  `json:"ended,omitzero"`
	Prompt   string     `json:"prompt,omitempty"`
	CtlLog   string     `json:"ctl_log,omitempty"`
	AgentLog string     `json:"agent_log,omitempty"`
	Steps    [][]string `json:"steps,omitempty"`
	Queued   time.Time  `json:"queued,omitzero"`
	Picked   time.Time  `json:"picked,omitzero"`
	Reported bool       `json:"reported,omitempty"`
}

type server struct {
	repo, backups, state string
	jobsFile             string
	emails               map[string]bool
	self                 string
	client               *http.Client
	gh, bb               string

	mu       sync.Mutex
	vms      map[string]*managed
	jobs     []*job
	nextID   int64
	lastPoll map[string]time.Time
	wake     chan struct{} // closed and replaced whenever a job is queued
}

func main() {
	flag.Parse()
	home, _ := os.UserHomeDir()
	s := &server{
		repo: *repoDir, backups: *backupDir, state: *stateFile,
		emails: map[string]bool{}, vms: map[string]*managed{}, lastPoll: map[string]time.Time{},
		client: &http.Client{Timeout: 2 * time.Minute}, wake: make(chan struct{}),
		gh: os.Getenv("VM_GITHUB_TOKEN"), bb: os.Getenv("VM_BUILDBUDDY_API_KEY"),
		nextID: time.Now().Unix(), // so an agent reporting on a job from before a restart can't hit a new one
	}
	if s.repo == "" {
		s.repo = ".."
	}
	if s.backups == "" {
		s.backups = filepath.Join(home, "vm-transcripts")
	}
	if s.state == "" {
		s.state = filepath.Join(home, ".config", "vm", "state.json")
	}
	if _, err := os.Stat(filepath.Join(s.repo, "vm-child")); err != nil {
		log.Fatalf("-repo: %v", err)
	}
	s.self, _ = os.Hostname()
	for _, e := range strings.Split(*allowed, ",") {
		if e = strings.TrimSpace(e); e != "" {
			s.emails[strings.ToLower(e)] = true
		}
	}
	if b, err := os.ReadFile(s.state); err == nil {
		if err := json.Unmarshal(b, &s.vms); err != nil {
			log.Fatalf("%s: %v", s.state, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		log.Fatal(err)
	}

	s.jobsFile = filepath.Join(filepath.Dir(s.state), "jobs.json")
	if err := s.loadJobs(); err != nil {
		log.Fatalf("%s: %v", s.jobsFile, err)
	}
	go s.saveJobsEvery(2 * time.Second)
	go func() {
		// Save on the way out, so a restart loses nothing.
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM, os.Interrupt)
		<-ch
		s.saveJobs()
		os.Exit(0)
	}()

	ui := http.NewServeMux()
	ui.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "index.html")
	})
	ui.HandleFunc("GET /api/vms", s.listVMs)
	ui.HandleFunc("POST /api/vms", s.startVM)
	ui.HandleFunc("POST /api/vms/{name}/prompt", s.promptVM)
	ui.HandleFunc("POST /api/vms/{name}/setup", s.setupVM)
	ui.HandleFunc("POST /api/vms/{name}/backup", s.backupVM)
	ui.HandleFunc("POST /api/vms/{name}/rm", s.rmVM)
	ui.HandleFunc("POST /api/vms/{name}/adopt", s.adoptVM)
	ui.HandleFunc("POST /api/vms/{name}/forget", s.forgetVM)
	ui.HandleFunc("GET /api/jobs", s.listJobs)
	ui.HandleFunc("POST /api/jobs/{id}/login", s.login)
	ui.HandleFunc("GET /api/backups", s.listBackups)
	ui.HandleFunc("GET /api/backups/{name}", s.downloadBackup)

	agent := http.NewServeMux()
	agent.HandleFunc("GET /agent/vm-child", s.agentScript)
	agent.HandleFunc("POST /agent/poll", s.agentPoll)
	agent.HandleFunc("POST /agent/jobs/{id}", s.agentReport)
	agent.HandleFunc("GET /agent/bundle", s.agentBundle)
	agent.HandleFunc("POST /agent/backup", s.agentBackup)

	go s.janitor()
	go func() {
		log.Printf("agents on %s", *agentAddr)
		log.Fatal(http.ListenAndServe(*agentAddr, s.agentAuth(agent)))
	}()
	log.Printf("vm-web on %s; exe.dev via %s; VMs reach this app via %s", *addr, *exeAPI, *ctlURL)
	log.Fatal(http.ListenAndServe(*addr, s.auth(ui)))
}

// auth admits only allowed exe.dev users, and refuses cross-site writes. A
// request from one of the VMs never gets in, even if exe.dev vouches for its
// owner's email on it.
func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Exedev-Source-Vm") != "" {
			http.Error(w, "VMs use the agent port", http.StatusForbidden)
			return
		}
		email := strings.ToLower(r.Header.Get("X-ExeDev-Email"))
		if !s.emails[email] {
			if email == "" {
				http.Redirect(w, r, "/__exe.dev/login?redirect="+r.URL.Path, http.StatusFound)
				return
			}
			http.Error(w, email+" isn't allowed here", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" || r.Header.Get("Content-Type") != "application/json" {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// agentAuth admits only managed VMs, as named by the peer integration.
func (s *server) agentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vm := r.Header.Get("X-Exedev-Source-Vm")
		s.mu.Lock()
		ok := s.vms[vm] != nil
		s.mu.Unlock()
		if !ok {
			var hs []string
			for k := range r.Header {
				hs = append(hs, k)
			}
			sort.Strings(hs)
			log.Printf("refused agent request from VM %q: %s %s (headers: %s)", vm, r.Method, r.URL.Path, strings.Join(hs, " "))
			http.Error(w, "not a VM this app manages", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// exe runs an exe.dev command through the API integration.
func (s *server) exe(ctx context.Context, args ...string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", *exeAPI+"/exec", strings.NewReader(shellQuote(args)))
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// exe.dev answers with the command's JSON output, and reports failure as
	// a JSON object with an "error" field, sometimes followed by more output
	// and often with status 200. The exit code, when there is one, is a
	// trailer.
	fail := func(msg string) error { return fmt.Errorf("exe.dev %s: %s", args[0], msg) }
	if resp.StatusCode/100 != 2 {
		return nil, fail(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(body)))
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	for {
		var v struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&v); err != nil {
			break
		}
		if v.Error != "" {
			return nil, fail(v.Error)
		}
	}
	if x := resp.Trailer.Get("X-Exe-Exit"); x != "" && x != "0" {
		return nil, fail(fmt.Sprintf("exit %s: %s", x, bytes.TrimSpace(body)))
	}
	return body, nil
}

func (s *server) exeVMs(ctx context.Context) ([]map[string]any, error) {
	var out []byte
	var err error
	// exe.dev occasionally drops a connection, so try twice.
	for range 2 {
		if out, err = s.exe(ctx, "ls", "--json"); err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	var ls struct {
		VMs []map[string]any `json:"vms"`
	}
	if err := json.Unmarshal(out, &ls); err != nil {
		return nil, fmt.Errorf("exe.dev ls: %v", err)
	}
	return ls.VMs, nil
}

func (s *server) saveLocked() error {
	b, err := json.MarshalIndent(s.vms, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.state), 0o755); err != nil {
		return err
	}
	tmp := s.state + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.state)
}

// --- UI ---

func (s *server) listVMs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	vms, err := s.exeVMs(ctx)
	if err != nil {
		httpError(w, http.StatusBadGateway, "%v", err)
		return
	}
	sort.Slice(vms, func(i, j int) bool { return fmt.Sprint(vms[i]["vm_name"]) < fmt.Sprint(vms[j]["vm_name"]) })
	s.mu.Lock()
	for _, v := range vms {
		name := fmt.Sprint(v["vm_name"])
		if m := s.vms[name]; m != nil {
			v["managed"] = m
			if t, ok := s.lastPoll[name]; ok {
				v["agent_seen"] = t
			}
		}
	}
	s.mu.Unlock()
	writeJSON(w, map[string]any{"vms": vms, "self": s.self, "ctl_url": *ctlURL, "tag": *tag})
}

func (s *server) startVM(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name, Prompt, Kind string
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return
	}
	if len(req.Name) < 5 || len(req.Name) > 52 || !nameRE.MatchString(req.Name) {
		httpError(w, http.StatusBadRequest, "use 5-52 lowercase letters, digits and single hyphens, starting with a letter")
		return
	}
	if req.Kind != "dispatcher" {
		req.Kind = "classic"
	}
	if s.gh == "" || s.bb == "" {
		httpError(w, http.StatusFailedDependency, "set VM_GITHUB_TOKEN and VM_BUILDBUDDY_API_KEY in ~/.config/vm/secrets and restart vm-web")
		return
	}
	s.mu.Lock()
	m := s.vms[req.Name]
	s.mu.Unlock()
	if m != nil {
		// Like vm NAME on an existing VM: set it up again and start claude.
		// Asking for a dispatcher turns a classic VM into one; the reverse
		// takes an explicit choice in the UI, as with the vm script's marker.
		if req.Kind == "dispatcher" && m.Kind != "dispatcher" {
			s.mu.Lock()
			m.Kind = "dispatcher"
			s.saveLocked()
			s.mu.Unlock()
		}
		s.queue(w, "start", req.Name, req.Prompt, false, [][]string{{"setup"}, startStep(req.Prompt)})
		return
	}
	j, err := s.newJob("start", req.Name, req.Prompt, false)
	if err != nil {
		httpError(w, http.StatusConflict, "%v", err)
		return
	}
	go s.create(j, req.Kind)
	writeJSON(w, map[string]any{"id": j.ID})
}

// create makes the VM, whose first boot installs vm-child, which then polls
// for the setup and start steps queued here.
func (s *server) create(j *job, kind string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	vms, err := s.exeVMs(ctx)
	if err != nil {
		s.fail(j, err)
		return
	}
	for _, v := range vms {
		if v["vm_name"] == j.VM {
			s.fail(j, fmt.Errorf("%s already exists, and this app didn't make it; adopt it from the list instead", j.VM))
			return
		}
	}
	// Record the VM first, so its agent is let in as soon as it boots.
	s.mu.Lock()
	s.vms[j.VM] = &managed{Kind: kind, Created: time.Now()}
	err = s.saveLocked()
	s.mu.Unlock()
	if err != nil {
		s.fail(j, err)
		return
	}
	bootstrap := fmt.Sprintf("curl -fsS --retry 60 --retry-delay 5 --retry-all-errors -o /tmp/vm-child %s/agent/vm-child && bash /tmp/vm-child install %s", *ctlURL, *ctlURL)
	args := []string{"new", "--name", j.VM, "--cpu", "8", "--memory", "32GB", "--disk", "100GB",
		"--no-email", "--tag", *tag, "--setup-script", bootstrap, "--json"}
	s.appendLog(j, fmt.Sprintf("Creating %s (%s)...\n$ ssh exe.dev %s\n", j.VM, kind, shellQuote(args[:len(args)-3])+" --setup-script … --json"))
	out, err := s.exe(ctx, args...)
	if err != nil {
		// A timeout may still have made the VM; only forget it if it isn't there.
		if vms, lerr := s.exeVMs(ctx); lerr == nil && !hasVM(vms, j.VM) {
			s.mu.Lock()
			delete(s.vms, j.VM)
			s.saveLocked()
			s.mu.Unlock()
		}
		s.fail(j, err)
		return
	}
	s.appendLog(j, string(out)+"\nWaiting for the VM to boot and its agent to check in...\n")
	s.mu.Lock()
	j.steps = [][]string{{"setup"}, startStep(j.Prompt)}
	j.State, j.queued = "queued", time.Now()
	s.wakeLocked()
	s.mu.Unlock()
}

func hasVM(vms []map[string]any, name string) bool {
	for _, v := range vms {
		if v["vm_name"] == name {
			return true
		}
	}
	return false
}

func startStep(prompt string) []string {
	if prompt == "" {
		return []string{"start"}
	}
	return []string{"start", prompt}
}

func (s *server) promptVM(w http.ResponseWriter, r *http.Request) {
	var req struct{ Prompt string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		httpError(w, http.StatusBadRequest, "a prompt is required")
		return
	}
	s.queue(w, "prompt", r.PathValue("name"), req.Prompt, false, [][]string{startStep(req.Prompt)})
}

// setupVM reruns setup, like vm NAME: refreshes notes, agents and settings,
// and switches between classic and dispatcher if asked. A running claude
// keeps its old system prompt until it's restarted.
func (s *server) setupVM(w http.ResponseWriter, r *http.Request) {
	var req struct{ Kind string }
	json.NewDecoder(r.Body).Decode(&req)
	name := r.PathValue("name")
	s.mu.Lock()
	if m := s.vms[name]; m != nil && (req.Kind == "classic" || req.Kind == "dispatcher") && m.Kind != req.Kind {
		m.Kind = req.Kind
		s.saveLocked()
	}
	s.mu.Unlock()
	s.queue(w, "setup", name, "", false, [][]string{{"setup"}, {"start"}})
}

func (s *server) backupVM(w http.ResponseWriter, r *http.Request) {
	s.queue(w, "backup", r.PathValue("name"), "", false, [][]string{{"backup"}})
}

// rmVM backs up the VM's transcripts one last time, then deletes it. A VM
// whose backup fails is kept.
func (s *server) rmVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == s.self {
		httpError(w, http.StatusBadRequest, "%s is running this app", name)
		return
	}
	s.queue(w, "rm", name, "", false, [][]string{{"backup"}})
}

// deleteVM finishes an rm job once the agent's final backup succeeds.
func (s *server) deleteVM(j *job) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	s.appendLog(j, "$ ssh exe.dev rm "+j.VM+"\n")
	if _, err := s.exe(ctx, "rm", j.VM); err != nil {
		s.fail(j, err)
		return
	}
	s.mu.Lock()
	delete(s.vms, j.VM)
	delete(s.lastPoll, j.VM)
	s.saveLocked()
	s.mu.Unlock()
	s.appendLog(j, "Deleted.\n")
	s.finish(j, 0)
}

// adoptVM takes on a VM this app didn't make, such as one from the vm
// script. It gets an agent once someone with SSH access installs vm-child
// on it (the UI shows how).
func (s *server) adoptVM(w http.ResponseWriter, r *http.Request) {
	var req struct{ Kind string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.Kind != "dispatcher" {
		req.Kind = "classic"
	}
	name := r.PathValue("name")
	if name == s.self || !nameRE.MatchString(name) {
		httpError(w, http.StatusBadRequest, "can't adopt %s", name)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vms[name] == nil {
		s.vms[name] = &managed{Kind: req.Kind, Created: time.Now()}
		if err := s.saveLocked(); err != nil {
			httpError(w, http.StatusInternalServerError, "%v", err)
			return
		}
	}
	writeJSON(w, map[string]any{"ok": true})
}

// forgetVM stops managing a VM without touching it.
func (s *server) forgetVM(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.vms, r.PathValue("name"))
	delete(s.lastPoll, r.PathValue("name"))
	if err := s.saveLocked(); err != nil {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Code string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		httpError(w, http.StatusBadRequest, "a code is required")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var j *job
	for _, jj := range s.jobs {
		if fmt.Sprint(jj.ID) == r.PathValue("id") {
			j = jj
		}
	}
	if j == nil || j.State != "login" {
		httpError(w, http.StatusConflict, "that job isn't waiting for a login")
		return
	}
	// The same job carries on: type the code into the VM's waiting login,
	// then rerun the start it interrupted.
	j.steps = [][]string{{"login", strings.TrimSpace(req.Code)}, startStep(j.Prompt)}
	j.State, j.LoginURL, j.Ended, j.queued, j.picked, j.reported = "queued", "", time.Time{}, time.Now(), time.Time{}, false
	j.ctlLog.WriteString(j.agentLog)
	j.agentLog = ""
	s.wakeLocked()
	writeJSON(w, map[string]any{"id": j.ID})
}

func (s *server) listJobs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*job, 0, len(s.jobs))
	for i := len(s.jobs) - 1; i >= 0; i-- {
		j := s.jobs[i]
		if j.Auto && j.State == "ok" {
			continue
		}
		j.Log = j.ctlLog.String() + j.agentLog
		out = append(out, j)
	}
	writeJSON(w, map[string]any{"jobs": out})
}

// newJob registers a job for vm, unless one is already active for it.
func (s *server) newJob(kind, vm, prompt string, auto bool) (*job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.newJobLocked(kind, vm, prompt, auto)
}

func (s *server) newJobLocked(kind, vm, prompt string, auto bool) (*job, error) {
	for _, j := range s.jobs {
		if j.VM == vm && (j.State == "running" || j.State == "queued") {
			return nil, fmt.Errorf("%s is busy with job %d (%s)", vm, j.ID, j.Kind)
		}
		// A new attempt supersedes a login nobody finished.
		if j.VM == vm && j.State == "login" {
			j.State, j.Ended = "failed", time.Now()
			fmt.Fprintf(&j.ctlLog, "%s\nSuperseded by a newer job.\n", j.agentLog)
			j.agentLog = ""
		}
	}
	s.nextID++
	j := &job{ID: s.nextID, Kind: kind, VM: vm, State: "running", Started: time.Now(), Prompt: prompt, Auto: auto}
	s.jobs = append(s.jobs, j)
	if len(s.jobs) > 100 {
		s.jobs = s.jobs[len(s.jobs)-100:]
	}
	return j, nil
}

// queue hands steps to vm's agent as a new job, and responds with its id.
func (s *server) queue(w http.ResponseWriter, kind, vm, prompt string, auto bool, steps [][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.vms[vm] == nil {
		httpError(w, http.StatusBadRequest, "%s isn't managed by this app; adopt it first", vm)
		return
	}
	j, err := s.newJobLocked(kind, vm, prompt, auto)
	if err != nil {
		httpError(w, http.StatusConflict, "%v", err)
		return
	}
	j.steps, j.State, j.queued = steps, "queued", time.Now()
	s.wakeLocked()
	writeJSON(w, map[string]any{"id": j.ID})
}

func (s *server) wakeLocked() {
	close(s.wake)
	s.wake = make(chan struct{})
}

func (s *server) fail(j *job, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.State, j.Ended = "failed", time.Now()
	fmt.Fprintf(&j.ctlLog, "\n%v\n", err)
}

func (s *server) finish(j *job, exit int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finishLocked(j, exit)
}

func (s *server) finishLocked(j *job, exit int) {
	j.Ended = time.Now()
	switch {
	case exit == 0:
		j.State = "ok"
	case exit == 3 && loginURLRE.MatchString(j.agentLog):
		j.State = "login"
		j.LoginURL = loginURLRE.FindStringSubmatch(j.agentLog)[1]
	default:
		j.State = "failed"
		fmt.Fprintf(&j.ctlLog, "%s\nexit status %d\n", j.agentLog, exit)
		j.agentLog = ""
	}
}

func (s *server) appendLog(j *job, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.ctlLog.WriteString(text)
	if j.ctlLog.Len() > maxLog {
		b := j.ctlLog.Bytes()
		j.ctlLog = *bytes.NewBuffer(append([]byte("…\n"), b[len(b)-maxLog/2:]...))
	}
}

// janitor fails jobs no agent picks up or finishes, and queues the hourly
// backups.
func (s *server) janitor() {
	lastBackup := map[string]time.Time{}
	for range time.Tick(30 * time.Second) {
		s.mu.Lock()
		now := time.Now()
		for _, j := range s.jobs {
			switch {
			case j.State == "queued" && now.Sub(j.queued) > pickupTimeout:
				j.State, j.Ended = "failed", now
				fmt.Fprintf(&j.ctlLog, "\n%s's agent didn't pick this up within %v. Is the VM running? A VM made by the vm script needs vm-child installed first.\n", j.VM, pickupTimeout)
			case j.State == "running" && !j.picked.IsZero() && now.Sub(j.picked) > runTimeout:
				j.State, j.Ended = "failed", now
				fmt.Fprintf(&j.ctlLog, "%s\nThe agent didn't finish within %v.\n", j.agentLog, runTimeout)
				j.agentLog = ""
			}
		}
		for name := range s.vms {
			if now.Sub(s.lastPoll[name]) > agentFresh || now.Sub(lastBackup[name]) < backupInterval {
				continue
			}
			if j, err := s.newJobLocked("backup", name, "", true); err == nil {
				j.steps, j.State, j.queued = [][]string{{"backup"}}, "queued", now
				lastBackup[name] = now
				s.wakeLocked()
			}
		}
		s.mu.Unlock()
	}
}

// --- backups ---

func (s *server) listBackups(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(s.backups)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		httpError(w, http.StatusInternalServerError, "%v", err)
		return
	}
	type backup struct {
		Name     string    `json:"name"`
		Files    int       `json:"files"`
		Bytes    int64     `json:"bytes"`
		Sessions int       `json:"sessions"`
		Latest   time.Time `json:"latest"`
	}
	out := []backup{}
	for _, e := range entries {
		if !e.IsDir() || !nameRE.MatchString(e.Name()) {
			continue
		}
		b := backup{Name: e.Name()}
		filepath.WalkDir(filepath.Join(s.backups, e.Name()), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			b.Files++
			b.Bytes += info.Size()
			if info.ModTime().After(b.Latest) {
				b.Latest = info.ModTime()
			}
			// A main claude session, not a subagent's.
			if strings.HasSuffix(p, ".jsonl") && !strings.Contains(p, "/subagents/") && strings.Contains(p, "/claude/projects/") {
				b.Sessions++
			}
			return nil
		})
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Latest.After(out[j].Latest) })
	writeJSON(w, map[string]any{"backups": out, "dir": s.backups})
}

// downloadBackup streams NAME's backup as a .tar.gz.
func (s *server) downloadBackup(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(r.PathValue("name"), ".tar.gz")
	root := filepath.Join(s.backups, name)
	if !nameRE.MatchString(name) {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(root); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-transcripts.tar.gz"`, name))
	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)
	if err := tw.AddFS(os.DirFS(root)); err != nil {
		log.Printf("download %s: %v", name, err)
	}
	tw.Close()
	gz.Close()
}

// Where each path in an agent's backup goes under ~/vm-transcripts/NAME:
// the same layout the vm-backup script makes.
var backupPaths = []struct{ from, to string }{
	{".claude/projects/", "claude/projects/"},
	{".claude/history.jsonl", "claude/history.jsonl"},
	{".codex/sessions/", "codex/sessions/"},
	{".cache/vm-backup/shelley.db", "shelley/shelley.db"},
}

func backupPath(name string) (string, bool) {
	name = path.Clean(strings.TrimPrefix(name, "./"))
	if strings.HasPrefix(name, "/") || name == ".." || strings.HasPrefix(name, "../") {
		return "", false
	}
	for _, p := range backupPaths {
		if strings.HasSuffix(p.from, "/") && strings.HasPrefix(name, p.from) {
			return p.to + strings.TrimPrefix(name, p.from), true
		}
		if name == p.from {
			return p.to, true
		}
	}
	return "", false
}

// agentBackup stores the files in an agent's .tar.gz under its own VM's
// directory. Only regular files on the known paths are kept, and nothing is
// ever deleted.
func (s *server) agentBackup(w http.ResponseWriter, r *http.Request) {
	vm := r.Header.Get("X-Exedev-Source-Vm")
	root := filepath.Join(s.backups, vm)
	gz, err := gzip.NewReader(http.MaxBytesReader(w, r.Body, 4<<30))
	if err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return
	}
	tr := tar.NewReader(gz)
	n := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			httpError(w, http.StatusBadRequest, "after %d files: %v", n, err)
			return
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		rel, ok := backupPath(h.Name)
		if !ok {
			continue
		}
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := writeFile(dst, tr, h.ModTime); err != nil {
			httpError(w, http.StatusInternalServerError, "%v", err)
			return
		}
		n++
	}
	s.mu.Lock()
	if m := s.vms[vm]; m != nil {
		m.LastBackup = time.Now()
		s.saveLocked()
	}
	s.mu.Unlock()
	writeJSON(w, map[string]any{"files": n})
}

func writeFile(dst string, r io.Reader, mtime time.Time) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dst), ".part-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	os.Chtimes(f.Name(), mtime, mtime)
	return os.Rename(f.Name(), dst)
}

// --- agents ---

func (s *server) agentScript(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(s.repo, "vm-child"))
}

func (s *server) scriptVersion() string {
	b, err := os.ReadFile(filepath.Join(s.repo, "vm-child"))
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// agentPoll hands the agent its VM's next job, waiting up to 50s for one.
func (s *server) agentPoll(w http.ResponseWriter, r *http.Request) {
	vm := r.Header.Get("X-Exedev-Source-Vm")
	var req struct{ Version string }
	json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req)
	// An agent running an old vm-child updates before it gets a job, so the
	// job runs with the current script.
	if v := s.scriptVersion(); req.Version != "" && req.Version != v {
		s.mu.Lock()
		s.lastPoll[vm] = time.Now()
		s.mu.Unlock()
		writeJSON(w, map[string]any{"version": v})
		return
	}
	deadline := time.After(50 * time.Second)
	// An agent runs one job at a time, so a job it took and never finished
	// is lost, say to a restart of the VM.
	s.mu.Lock()
	for _, j := range s.jobs {
		if j.VM == vm && j.State == "running" && !j.picked.IsZero() && !j.reported {
			j.State, j.Ended = "failed", time.Now()
			fmt.Fprintf(&j.ctlLog, "%s\n%s's agent stopped before finishing this.\n", j.agentLog, vm)
			j.agentLog = ""
		}
	}
	s.mu.Unlock()
	for {
		resp := map[string]any{"version": s.scriptVersion()}
		s.mu.Lock()
		s.lastPoll[vm] = time.Now()
		var next *job
		for _, j := range s.jobs {
			if j.VM == vm && j.State == "queued" {
				next = j
				break
			}
		}
		if next != nil {
			next.State, next.picked = "running", time.Now()
			resp["job"] = map[string]any{"id": next.ID, "steps": next.steps}
			s.mu.Unlock()
			writeJSON(w, resp)
			return
		}
		wake := s.wake
		s.mu.Unlock()
		select {
		case <-wake:
		case <-deadline:
			writeJSON(w, resp)
			return
		case <-r.Context().Done():
			return
		}
	}
}

func (s *server) agentReport(w http.ResponseWriter, r *http.Request) {
	vm := r.Header.Get("X-Exedev-Source-Vm")
	var req struct {
		Log  string
		Done bool
		Exit int
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return
	}
	s.mu.Lock()
	var j *job
	for _, jj := range s.jobs {
		if fmt.Sprint(jj.ID) == r.PathValue("id") && jj.VM == vm && jj.State == "running" {
			j = jj
		}
	}
	if j == nil {
		s.mu.Unlock()
		httpError(w, http.StatusNotFound, "no such running job for %s", vm)
		return
	}
	j.agentLog = req.Log
	del := false
	if req.Done {
		j.reported = true
		if req.Exit == 0 && j.Kind == "rm" {
			del = true
		} else {
			s.finishLocked(j, req.Exit)
		}
	}
	s.mu.Unlock()
	if del {
		go s.deleteVM(j)
	}
	writeJSON(w, map[string]any{"ok": true})
}

// agentBundle is what vm-child setup needs: the notes and agents from the
// repo, the VM's settings and the secrets, as a .tar.gz.
func (s *server) agentBundle(w http.ResponseWriter, r *http.Request) {
	vm := r.Header.Get("X-Exedev-Source-Vm")
	s.mu.Lock()
	kind := s.vms[vm].Kind
	s.mu.Unlock()
	if s.gh == "" || s.bb == "" {
		httpError(w, http.StatusFailedDependency, "vm-web has no VM_GITHUB_TOKEN or VM_BUILDBUDDY_API_KEY")
		return
	}
	dispatcher := ""
	if kind == "dispatcher" {
		dispatcher = "1"
	}
	vars := func(kv ...string) []byte {
		var b bytes.Buffer
		for i := 0; i < len(kv); i += 2 {
			fmt.Fprintf(&b, "%s=%s\n", kv[i], shellQuote([]string{kv[i+1]}))
		}
		return b.Bytes()
	}
	gen := map[string][]byte{
		"config": vars("name", vm, "dispatcher", dispatcher,
			"git_name", env("VM_GIT_NAME", "Dan Stowell"), "git_email", env("VM_GIT_EMAIL", "dan@buildbuddy.io"),
			"shelley_model", env("VM_SHELLEY_MODEL", "gpt-6.1-sol"), "claude_email", env("VM_CLAUDE_EMAIL", "dstowell@gmail.com")),
		"secrets": vars("gh_token", s.gh, "bb_api_key", s.bb),
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, pat := range bundleFiles {
		matches, _ := filepath.Glob(filepath.Join(s.repo, pat))
		if len(matches) == 0 {
			httpError(w, http.StatusInternalServerError, "no %s in %s", pat, s.repo)
			return
		}
		for _, m := range matches {
			b, err := os.ReadFile(m)
			if err != nil {
				httpError(w, http.StatusInternalServerError, "%v", err)
				return
			}
			rel, _ := filepath.Rel(s.repo, m)
			gen[filepath.ToSlash(rel)] = b
		}
	}
	names := make([]string, 0, len(gen))
	for n := range gen {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		tw.WriteHeader(&tar.Header{Name: n, Mode: 0o600, Size: int64(len(gen[n])), ModTime: time.Now()})
		tw.Write(gen[n])
	}
	tw.Close()
	gz.Close()
	w.Header().Set("Content-Type", "application/gzip")
	w.Write(buf.Bytes())
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func shellQuote(args []string) string {
	q := make([]string, len(args))
	for i, a := range args {
		if a != "" && !strings.ContainsAny(a, " \t\n'\"\\$`!*?&;|<>(){}[]#~") {
			q[i] = a
		} else {
			q[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(q, " ")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, format string, args ...any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf(format, args...)})
}

func (s *server) loadJobs() error {
	b, err := os.ReadFile(s.jobsFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	var stored []storedJob
	if err := json.Unmarshal(b, &stored); err != nil {
		return err
	}
	for _, sj := range stored {
		j := &job{ID: sj.ID, Kind: sj.Kind, VM: sj.VM, State: sj.State, Auto: sj.Auto, LoginURL: sj.LoginURL,
			Started: sj.Started, Ended: sj.Ended, Prompt: sj.Prompt, agentLog: sj.AgentLog, steps: sj.Steps,
			queued: sj.Queued, picked: sj.Picked, reported: sj.Reported}
		j.ctlLog.WriteString(sj.CtlLog)
		if j.State == "running" {
			switch {
			case j.steps == nil && j.Kind == "start":
				// Stopped while creating the VM, which is already recorded:
				// hand its agent the setup anyway, and let the pickup timeout
				// catch a VM that never got made.
				j.steps = [][]string{{"setup"}, startStep(j.Prompt)}
				j.State, j.queued = "queued", time.Now()
				j.ctlLog.WriteString("vm-web restarted; waiting for the VM's agent.\n")
			case j.reported && j.Kind == "rm":
				j.State, j.Ended = "failed", time.Now()
				fmt.Fprintf(&j.ctlLog, "%s\nvm-web restarted before deleting %s; its backup is done, so delete it again.\n", j.agentLog, j.VM)
				j.agentLog = ""
			}
			// Otherwise the agent may still be working on it, and reports
			// to the same ID.
		}
		s.jobs = append(s.jobs, j)
		s.nextID = max(s.nextID, j.ID)
	}
	return nil
}

func (s *server) saveJobsEvery(d time.Duration) {
	for range time.Tick(d) {
		s.saveJobs()
	}
}

var lastSavedJobs []byte

func (s *server) saveJobs() {
	s.mu.Lock()
	stored := make([]storedJob, len(s.jobs))
	for i, j := range s.jobs {
		stored[i] = storedJob{ID: j.ID, Kind: j.Kind, VM: j.VM, State: j.State, Auto: j.Auto, LoginURL: j.LoginURL,
			Started: j.Started, Ended: j.Ended, Prompt: j.Prompt, CtlLog: j.ctlLog.String(), AgentLog: j.agentLog,
			Steps: j.steps, Queued: j.queued, Picked: j.picked, Reported: j.reported}
	}
	b, err := json.Marshal(stored)
	if err != nil || bytes.Equal(b, lastSavedJobs) {
		s.mu.Unlock()
		return
	}
	lastSavedJobs = b
	s.mu.Unlock()
	// Steps can hold a login code, so only this user can read the file.
	tmp := s.jobsFile + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		log.Printf("save jobs: %v", err)
		return
	}
	if err := os.Rename(tmp, s.jobsFile); err != nil {
		log.Printf("save jobs: %v", err)
	}
}
