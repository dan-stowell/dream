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
- `gh` is logged in as Dan's own GitHub account, so anything done with it
  (pushes, PRs, comments) shows up as Dan.
- Commit and push often.
- When asked for a codex review, run `codex review` in the repo, e.g.
  `codex review --base master` for the branch or `codex review --uncommitted`
  for the working tree (`codex review --help` lists more options). It's
  already configured, so pass no model or provider flags. Only do this when
  asked, and don't ask codex to request reviews itself.
