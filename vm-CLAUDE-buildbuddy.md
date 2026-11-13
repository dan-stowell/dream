## Repos

- buildbuddy, buildbuddy-internal, buildbuddy-helm, buildbuddy-toolchain and
  podman-static, all from buildbuddy-io, with master as the default branch.
- Before pushing, catch formatting and lint issues with
  `bb run --config=remote -- //tools/lint` in buildbuddy, or
  `bb run --config=remote -- @com_github_buildbuddy_io_buildbuddy//tools/lint`
  in buildbuddy-internal. Add `-fix` to fix what it can. Run it right here on
  the VM, not under `bb remote`, and with `--config=remote` rather than
  `remote-minimal` or `cache`, so the linters build on RBE and only the lint
  run itself happens locally. Don't hand-run binaries out of its runfiles.
- `gh` is logged in as Dan's own GitHub account, so anything done with it
  (pushes, PRs, comments) shows up as Dan.
- Open issues in buildbuddy-internal.
- Open new work as draft PRs on the private fork,
  dan-stowell/buildbuddy-private-fork, and publish it to buildbuddy-io once
  it's in shape; PRs already public stay public. `gh` can't see the fork, so
  go through the exe.dev GitHub integration: the remote
  `https://github.int.exe.xyz/dan-stowell/buildbuddy-private-fork`, and
  `GH_HOST=github.int.exe.xyz gh ...`. Sync its master from upstream before
  cutting a branch (`git push fork origin/master:refs/heads/master`).
- Never name a customer's software, vendors or setup in code, tests,
  comments, commits or PRs, and don't say "customer". Describe the behavior
  guaranteed ("a redirect to an allowlisted private IP is followed"); keep
  the specifics in private notes.
- Don't mirror a buildbuddy `go_deps` module_override into
  buildbuddy-internal by hand: the 🔄 sync commits that bump the buildbuddy
  pin carry it over.
