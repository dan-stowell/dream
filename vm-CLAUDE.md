# VM notes

This is a throwaway exe.dev dev VM. The repos listed under Repos below are
shallow clones in $HOME, each on a branch named after the VM. That name is a
placeholder: unless the branch already exists on origin, rename it before its
first push to a short kebab-case name for the work
(`git branch -m fix-cache-eviction-race`), with no prefix. Give each separate
piece of work its own descriptively named branch, cut from the default
branch, rather than piling it onto one branch.

- Build and test remotely with `bb remote build` and `bb remote test`, always
  passing `--config=remote-minimal`. That runs every action on RBE with 100
  jobs and leaves outputs in the remote cache instead of the runner's disk.
  Don't lower `--jobs`. `bazel` and `bazelisk` are aliases for `bb`, and
  `BUILDBUDDY_API_KEY` is already set.
- Run remote work through `bb remote` only, never `bb execute`: `bb execute`
  produces no invocation, so there's no link to share or inspect.
- Never build or test on the VM itself (plain `bb build`, `bb test`,
  `go build`, `go test` and the like); run them on RBE through `bb remote`
  instead. The lint run under Repos is the one exception. This rule is about
  where builds run, not whether: never skip building or running tests because
  of it, and don't tell helpers to just read code instead.
- Run bazel commands directly, as `bb remote build`, `bb remote test` and the
  like. `bb remote --script='...'` is an escape hatch for work that truly
  can't be done as a bazel command. If you do need it, every bazel command
  inside it needs `--config=remote-minimal` too: without it everything
  compiles on the runner and its outputs fill the runner's disk. Runners are
  recycled, so a full disk carries over to later runs (`bazel clean
  --expunge` in a script recovers it).
- In scripts, check that a computed target list is non-empty before passing it
  to bazel. `bazel test $(cat targets.txt)` with an empty file builds a huge
  chunk of the repo.
- `--remote_download_minimal` means test logs aren't downloaded, so read them
  with `bb view <invocation-id-or-url>` rather than from `bazel-testlogs`.
  `--errors` prints just the first build error; adding target labels or
  `--test_filter=Name` prints the failed test output. See `bb view --help`.
- Give each test file its own test target. Don't combine separate test files
  (suites) into one target.
- Commit and push often.
- When a branch is worth looking at, open a draft PR with
  `gh pr create --draft` and keep it updated by pushing. Only ever draft:
  never run `gh pr ready` or `gh pr merge`, never request reviewers, and never
  mark a PR ready some other way. Dan does all of that. Keep the body
  short: a one-line headline plus at most 2-3 one-sentence bullets. Don't
  add labels to PRs.
- Open issues where Repos below says, unless asked to put them elsewhere.
  Keep them short: a clear, concrete title and at most 2-3 one-line bullets
  in the body. Label every issue you open `agent-written`, plus any other labels
  from `gh label list` that fit.
- Never reply to issue or PR comments written by humans; Dan answers those.
- After every push, ask codex for a review: run
  `codex exec --sandbox read-only "PROMPT" </dev/null` in the repo (without
  the `</dev/null` it waits on stdin), with a prompt that says what to review
  (e.g. the branch's changes from `git diff master...HEAD`, or `main` if
  that's the default branch) and what to look for, then weigh and address
  what it finds. Prefer it to `codex review`, which can't take custom
  instructions together with `--base`. `-o FILE` saves codex's final message.
  Codex is already configured (it uses gpt-6-astra), so pass no model or
  provider flags. Don't ask codex to request reviews itself.
