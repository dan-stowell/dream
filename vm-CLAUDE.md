# VM notes

This is a throwaway exe.dev dev VM. The buildbuddy, buildbuddy-internal,
buildbuddy-helm and buildbuddy-toolchain repos are shallow clones in $HOME,
each on a branch named after the VM.

- Build and test remotely with `bb remote build` and `bb remote test`, always
  passing `--config=remote-minimal`. That runs every action on RBE and leaves
  outputs in the remote cache instead of the runner's disk. `bazel` and
  `bazelisk` are aliases for `bb`, and `BUILDBUDDY_API_KEY` is already set.
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
- After every push, ask codex for a review: run
  `codex exec --sandbox read-only "PROMPT"` in the repo, with a prompt that
  says what to review (e.g. the branch's changes from `git diff master...HEAD`)
  and what to look for, then weigh and address what it finds. Prefer it to
  `codex review`, which can't take custom instructions together with `--base`.
  `-o FILE` saves codex's final message. Codex is already configured (it uses
  gpt-6-astra), so pass no model or provider flags. Don't ask codex to request
  reviews itself.
