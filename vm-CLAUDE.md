# VM notes

This is a throwaway exe.dev dev VM. The buildbuddy, buildbuddy-internal,
buildbuddy-helm and buildbuddy-toolchain repos are shallow clones in $HOME,
each on a branch named after the VM. That name is a placeholder: unless the
branch already exists on origin, rename it before its first push to a short
kebab-case name for the work (`git branch -m fix-cache-eviction-race`), with
no prefix. Give each separate piece of work its own descriptively named
branch, cut from master, rather than piling it onto one branch.

- Build and test remotely with `bb remote build` and `bb remote test`, always
  passing `--config=remote-minimal`. That runs every action on RBE with 100
  jobs and leaves outputs in the remote cache instead of the runner's disk.
  Don't lower `--jobs`. `bazel` and `bazelisk` are aliases for `bb`, and
  `BUILDBUDDY_API_KEY` is already set.
- Run remote work through `bb remote` only, never `bb execute`: `bb execute`
  produces no invocation, so there's no link to share or inspect.
- Never build or test on the VM itself (plain `bb build`, `bb test`,
  `go build`, `go test` and the like); run them on RBE through `bb remote`
  instead. The lint run below is the one exception. This rule is about where
  builds run, not whether: never skip building or running tests because of
  it, and don't tell helpers to just read code instead.
- The same goes for every bazel command inside `bb remote --script='...'`:
  without `--config=remote-minimal` everything compiles on the runner and its
  outputs fill the runner's disk. Runners are recycled, so a full disk carries
  over to later runs (`bazel clean --expunge` in a script recovers it).
- In scripts, check that a computed target list is non-empty before passing it
  to bazel. `bazel test $(cat targets.txt)` with an empty file builds a huge
  chunk of the repo.
- `--remote_download_minimal` means test logs aren't downloaded, so read them
  with `bb view <invocation-id-or-url>` rather than from `bazel-testlogs`.
  `--errors` prints just the first build error; adding target labels or
  `--test_filter=Name` prints the failed test output. See `bb view --help`.
- Before pushing, catch formatting and lint issues with
  `bb run --config=remote -- //tools/lint` in buildbuddy, or
  `bb run --config=remote -- @com_github_buildbuddy_io_buildbuddy//tools/lint`
  in buildbuddy-internal. Add `-fix` to fix what it can. Run it right here on
  the VM, not under `bb remote`, and with `--config=remote` rather than
  `remote-minimal` or `cache`, so the linters build on RBE and only the lint
  run itself happens locally. Don't hand-run binaries out of its runfiles.
- Give each test file its own test target. Don't combine separate test files
  (suites) into one target.
- `gh` is logged in as Dan's own GitHub account, so anything done with it
  (pushes, PRs, comments) shows up as Dan.
- Commit and push often.
- When a branch is worth looking at, open a draft PR with
  `gh pr create --draft` and keep it updated by pushing. Only ever draft:
  never run `gh pr ready` or `gh pr merge`, never request reviewers, and never
  mark a PR ready some other way. Dan does all of that. The body says
  what changed, why, and how it was tested (with BuildBuddy invocation links).
  Add labels where they fit, choosing from `gh label list`; don't create new
  ones.
- Open issues in buildbuddy-internal unless asked to put them elsewhere. Keep
  them short: a clear, concrete title and at most 2-3 one-line bullets in the
  body. Label every issue you open `agent-written`, plus any other labels
  from `gh label list` that fit.
- Never reply to issue or PR comments written by humans; Dan answers those.
- After every push, ask codex for a review: run
  `codex exec --sandbox read-only "PROMPT" </dev/null` in the repo (without
  the `</dev/null` it waits on stdin), with a prompt that says what to review
  (e.g. the branch's changes from `git diff master...HEAD`) and what to look
  for, then weigh and address what it finds. Prefer it to
  `codex review`, which can't take custom instructions together with `--base`.
  `-o FILE` saves codex's final message. Codex is already configured (it uses
  gpt-6-astra), so pass no model or provider flags. Don't ask codex to request
  reviews itself.
