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
