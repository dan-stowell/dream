// vm-web is a web front end to the vm script: it lists exe.dev VMs, starts
// them (vm NAME [PROMPT]), sends prompts to them, and deletes them after a
// transcript backup (vm rm NAME). Long-running vm commands run as background
// jobs whose output the page polls.
//
// It trusts the X-ExeDev-Email header set by exe.dev's HTTPS proxy, so serve
// it only through that proxy, with the VM's sharing kept private.
package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var static embed.FS

var (
	addr    = flag.String("addr", ":8000", "address to listen on")
	vmPath  = flag.String("vm", "", "path to the vm script (default: ../vm next to this binary's source)")
	allowed = flag.String("allow", "dstowell@gmail.com", "comma-separated emails allowed to use the app")
)

// vm's naming rule, as checked by the script itself.
var nameRE = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

var loginURLRE = regexp.MustCompile(`(?m)^vm: sign in at (\S+)$`)

const maxLog = 256 << 10

type job struct {
	ID       int       `json:"id"`
	Kind     string    `json:"kind"` // start, prompt, rm, login
	VM       string    `json:"vm"`
	State    string    `json:"state"` // running, login, ok, failed
	LoginURL string    `json:"login_url,omitempty"`
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended,omitzero"`
	Prompt   string    `json:"prompt,omitempty"` // so a login or retry can resume a start
	Log      string    `json:"log"`

	log bytes.Buffer
}

type server struct {
	vm     string
	self   string
	emails map[string]bool

	mu     sync.Mutex
	jobs   []*job
	nextID int
}

func main() {
	flag.Parse()
	s := &server{vm: *vmPath, emails: map[string]bool{}}
	if s.vm == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		s.vm = filepath.Join(filepath.Dir(exe), "..", "vm")
	}
	if _, err := os.Stat(s.vm); err != nil {
		log.Fatalf("vm script: %v", err)
	}
	s.self, _ = os.Hostname()
	for _, e := range strings.Split(*allowed, ",") {
		if e = strings.TrimSpace(e); e != "" {
			s.emails[strings.ToLower(e)] = true
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, static, "index.html")
	})
	mux.HandleFunc("GET /api/vms", s.listVMs)
	mux.HandleFunc("POST /api/vms", s.startVM)
	mux.HandleFunc("POST /api/vms/{name}/prompt", s.promptVM)
	mux.HandleFunc("POST /api/vms/{name}/rm", s.rmVM)
	mux.HandleFunc("GET /api/jobs", s.listJobs)
	mux.HandleFunc("POST /api/jobs/{id}/login", s.login)
	log.Printf("vm-web on %s, running %s", *addr, s.vm)
	log.Fatal(http.ListenAndServe(*addr, s.auth(mux)))
}

// auth admits only allowed exe.dev users, and refuses cross-site writes.
func (s *server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func (s *server) listVMs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "exe.dev", "ls", "--json").Output()
	if err != nil {
		// exe.dev explains some failures, like an unknown key, on stdout.
		if msg := bytes.TrimSpace(out); len(msg) > 0 {
			err = fmt.Errorf("%s", msg)
		} else if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			err = fmt.Errorf("%s", bytes.TrimSpace(ee.Stderr))
		}
		httpError(w, http.StatusBadGateway, "ssh exe.dev ls: %v", err)
		return
	}
	var ls struct {
		VMs []map[string]any `json:"vms"`
	}
	if err := json.Unmarshal(out, &ls); err != nil {
		httpError(w, http.StatusBadGateway, "ssh exe.dev ls: %v", err)
		return
	}
	sort.Slice(ls.VMs, func(i, j int) bool {
		return fmt.Sprint(ls.VMs[i]["vm_name"]) < fmt.Sprint(ls.VMs[j]["vm_name"])
	})
	writeJSON(w, map[string]any{"vms": ls.VMs, "self": s.self})
}

func (s *server) startVM(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name, Prompt string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return
	}
	if len(req.Name) < 5 || len(req.Name) > 52 || !nameRE.MatchString(req.Name) {
		httpError(w, http.StatusBadRequest, "use 5-52 lowercase letters, digits and single hyphens, starting with a letter")
		return
	}
	s.spawn(w, "start", req.Name, req.Prompt)
}

func (s *server) promptVM(w http.ResponseWriter, r *http.Request) {
	var req struct{ Prompt string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		httpError(w, http.StatusBadRequest, "a prompt is required")
		return
	}
	s.spawn(w, "prompt", r.PathValue("name"), req.Prompt)
}

func (s *server) rmVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == s.self {
		httpError(w, http.StatusBadRequest, "%s is running this app", name)
		return
	}
	s.spawn(w, "rm", name, "")
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req struct{ Code string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		httpError(w, http.StatusBadRequest, "a code is required")
		return
	}
	s.mu.Lock()
	var j *job
	for _, jj := range s.jobs {
		if fmt.Sprint(jj.ID) == r.PathValue("id") {
			j = jj
		}
	}
	if j == nil || j.State != "login" {
		s.mu.Unlock()
		httpError(w, http.StatusConflict, "that job isn't waiting for a login")
		return
	}
	s.mu.Unlock()
	s.spawnLogin(w, j.VM, strings.TrimSpace(req.Code), j.Prompt)
}

func (s *server) listJobs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]job, 0, len(s.jobs))
	for i := len(s.jobs) - 1; i >= 0; i-- {
		j := s.jobs[i]
		c := *j
		c.Log = j.log.String()
		c.log = bytes.Buffer{}
		out = append(out, c)
	}
	writeJSON(w, map[string]any{"jobs": out})
}

// newJob registers a job for vm, unless one is already running for it.
func (s *server) newJob(kind, vm, prompt string) (*job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.VM == vm && j.State == "running" {
			return nil, fmt.Errorf("%s is busy with job %d (%s)", vm, j.ID, j.Kind)
		}
		// A new attempt supersedes a login nobody finished.
		if j.VM == vm && j.State == "login" {
			j.State = "failed"
			j.Ended = time.Now()
			fmt.Fprintf(&j.log, "\nSuperseded by job %d.\n", s.nextID+1)
		}
	}
	s.nextID++
	j := &job{ID: s.nextID, Kind: kind, VM: vm, State: "running", Started: time.Now(), Prompt: prompt}
	s.jobs = append(s.jobs, j)
	if len(s.jobs) > 50 {
		s.jobs = s.jobs[len(s.jobs)-50:]
	}
	return j, nil
}

func (s *server) spawn(w http.ResponseWriter, kind, vm, prompt string) {
	j, err := s.newJob(kind, vm, prompt)
	if err != nil {
		httpError(w, http.StatusConflict, "%v", err)
		return
	}
	go func() {
		var args []string
		switch kind {
		case "rm":
			args = []string{"rm", vm}
		default:
			args = []string{vm}
			if prompt != "" {
				args = append(args, prompt)
			}
		}
		s.finish(j, s.run(j, args...))
	}()
	writeJSON(w, map[string]any{"id": j.ID})
}

// spawnLogin types the code into the VM's waiting login, then reruns the
// start it interrupted.
func (s *server) spawnLogin(w http.ResponseWriter, vm, code, prompt string) {
	j, err := s.newJob("login", vm, prompt)
	if err != nil {
		httpError(w, http.StatusConflict, "%v", err)
		return
	}
	go func() {
		err := s.run(j, "login", vm, code)
		if err == nil {
			args := []string{vm}
			if prompt != "" {
				args = append(args, prompt)
			}
			err = s.run(j, args...)
		}
		s.finish(j, err)
	}()
	writeJSON(w, map[string]any{"id": j.ID})
}

func (s *server) run(j *job, args ...string) error {
	shown := args
	if args[0] == "login" {
		shown = []string{"login", args[1], "CODE"}
	}
	s.appendLog(j, "$ vm "+shellQuote(shown)+"\n")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.vm, args...)
	cmd.Stdout = logWriter{s, j}
	cmd.Stderr = logWriter{s, j}
	return cmd.Run()
}

func (s *server) finish(j *job, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.Ended = time.Now()
	switch {
	case err == nil:
		j.State = "ok"
	case exitCode(err) == 3:
		if m := loginURLRE.FindStringSubmatch(j.log.String()); m != nil {
			j.State = "login"
			j.LoginURL = m[1]
			return
		}
		fallthrough
	default:
		j.State = "failed"
		fmt.Fprintf(&j.log, "\n%v\n", err)
	}
}

type logWriter struct {
	s *server
	j *job
}

func (lw logWriter) Write(p []byte) (int, error) {
	lw.s.appendLog(lw.j, string(p))
	return len(p), nil
}

func (s *server) appendLog(j *job, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j.log.WriteString(text)
	if j.log.Len() > maxLog {
		b := j.log.Bytes()
		j.log = *bytes.NewBuffer(append([]byte("…\n"), b[len(b)-maxLog/2:]...))
	}
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return -1
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
