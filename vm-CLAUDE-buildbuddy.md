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
  `GH_HOST=github.int.exe.xyz gh ...`. spawn fast-forwards its master to
  upstream every hour, so fetch it rather than syncing it yourself. Links
  you give Dan are github.com links
  (`https://github.com/dan-stowell/buildbuddy-private-fork/pull/N`), never
  github.int.exe.xyz ones.
- Security-sensitive changes to buildbuddy are made on the private fork like
  any other work, then reviewed in Gerrit (gerrit.buildbuddy.io), never as a
  public GitHub PR. Publish to Gerrit only when Dan says the change is ready
  for teammates: squash it to one commit on a branch off origin/master in
  ~/buildbuddy, then `gerrit-push -r reviewer@buildbuddy.io` (`-n` first to
  see what it would push). It adds the Change-Id Gerrit needs and prints the
  change's link; give Dan that link. To update the change, amend the commit
  and run `gerrit-push` again: the Change-Id stays, so it becomes a new patch
  set. Pushes appear as Dan. Never vote in Gerrit (Code-Review or Merge):
  Dan or the reviewer does. Merging sends it to GitHub as a public PR, so the
  commit message and description stay as plain as any fix's. Never pull
  from the gerrit remote. Gerrit covers buildbuddy only; buildbuddy-internal
  is private already. `ssh gerrit-ssh.buildbuddy.io gerrit query
  --format=JSON owner:self status:open` lists Dan's open changes.
- Never name a customer's software, vendors or setup in code, tests,
  comments, commits or PRs, and don't say "customer". Describe the behavior
  guaranteed ("a redirect to an allowlisted private IP is followed"); keep
  the specifics in private notes.
- Don't mirror a buildbuddy `go_deps` module_override into
  buildbuddy-internal by hand: the 🔄 sync commits that bump the buildbuddy
  pin carry it over.
