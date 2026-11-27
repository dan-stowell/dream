# VM notes

This exe.dev VM works on the repo in REPODIR. It lives only on this VM:
there is no GitHub remote, and nothing is pushed anywhere. The repo's own
files (its README, CHARTER.md, LOOP.md and PLAN.md) are the rules for the
work; these notes only cover the VM.

- Build, test and measure right here on the VM. There's no remote build
  setup for this work.
- Commit often. The repo is the memory of the work, and the VM's backups
  (hourly, to Dan's control VM) save its whole git history, uncommitted
  changes included. Don't add a remote or push.
- There's no GitHub token on the VM. Public sources (crates.io, GitHub
  releases, nixpkgs, mirrors) are fetched over the network as usual.
- Codex is configured (`codex exec --sandbox read-only "PROMPT" </dev/null`
  for a review, `--sandbox workspace-write` for work; without `</dev/null`
  it waits on stdin). It runs on its own budget, so give it the work it does
  as well as you.
- Check a claim (a result, a fix, a reproduction) independently of whoever
  made it, yourself included, and accept it only with proof: a script
  anyone can rerun.
- Helpers (subagents, codex) write their details to a file and reply in five
  lines or fewer, so your context stays small.
