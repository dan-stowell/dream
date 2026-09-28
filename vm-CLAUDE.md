# VM notes

This is a throwaway exe.dev dev VM. The buildbuddy, buildbuddy-internal,
buildbuddy-helm and buildbuddy-toolchain repos are shallow clones in $HOME,
each on a branch named after the VM.

- Build and test remotely with `bb remote build` and `bb remote test`, always
  passing `--config=remote`. `bazel` and `bazelisk` are aliases for `bb`, and
  `BUILDBUDDY_API_KEY` is already set.
- `bb view <invocation-id-or-url>` shows the logs for a BuildBuddy invocation.
  `--errors` prints just the first build error; adding target labels or
  `--test_filter=Name` prints the failed test output. See `bb view --help`.
- `gh` is logged in as Dan's own GitHub account, so anything done with it
  (pushes, PRs, comments) shows up as Dan.
- Commit and push often.
