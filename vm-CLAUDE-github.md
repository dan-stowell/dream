# VM notes

This exe.dev VM works on REPOS, cloned in $HOME through exe.dev's GitHub
integration. The repo's own files (its README, CLAUDE.md and any charter or
loop it has) are the rules for the work; these notes only cover the VM.

- Build, test and measure right here on the VM unless the repo says
  otherwise. There's no remote build setup for this work.
- `git` and `gh` go through the integration at github.int.exe.xyz
  (`GH_HOST` is already set), so pass `-R OWNER/REPO` to `gh`. There's no
  GitHub token on the VM, and only these repos are reachable.
- Commit and push often, to the branch the repo's notes say, or its
  default branch if they say nothing.
- Codex is configured (`codex exec --sandbox read-only "PROMPT" </dev/null`
  for a review, `--sandbox workspace-write` for work; without `</dev/null`
  it waits on stdin). It runs on its own budget, so give it the work it does
  as well as you.
- Check a claim (a bug, a fix, a speedup) independently of whoever made it,
  yourself included, and accept it only with proof: a test or a measurement
  anyone can rerun.
- Helpers (subagents, codex) write their details to a file and reply in five
  lines or fewer, so your context stays small.
- In replies to Dan, make every PR and issue reference a markdown link to it.
