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
- Put up new buildbuddy work as a Gerrit change (gerrit.buildbuddy.io),
  never as a GitHub PR, and don't use the old private fork. Squash it to one
  commit on a branch off origin/master in ~/buildbuddy, then `gerrit-push -w`
  (`-n` first to see what it would push). `-w` makes it work in progress:
  Dan sees it, and no one else is notified. The script adds the Change-Id
  Gerrit needs and prints the change's link; give Dan that link. To update
  the change, amend the commit and run `gerrit-push -w` again: the Change-Id
  stays, so it becomes a new patch set.
- Dan decides where each change goes from there. **Review in Gerrit:** he
  adds reviewers and starts the review himself, or tells you to
  (`gerrit-push -r reviewer@buildbuddy.io` on the next patch set). **To
  GitHub:** when he says so, open a PR on buildbuddy-io from the same commit
  with the `Change-Id:` trailer removed, then abandon the Gerrit change
  (`ssh gerrit-ssh.buildbuddy.io gerrit review --abandon <change>,<patchset>`).
  Changes already on GitHub as PRs stay there.
- Pushes appear as Dan. Never vote in Gerrit (Code-Review or Merge): Dan or
  the reviewer does. Merging sends it to GitHub as a public PR, so the commit message and description stay as plain as any fix's,
  security fixes included. Never pull from the gerrit remote.
  `ssh gerrit-ssh.buildbuddy.io gerrit query --format=JSON owner:self
  status:open` lists Dan's open changes.
- Gerrit covers buildbuddy only. buildbuddy-internal is private already, so
  work there goes up as draft PRs on GitHub as before.
- Never name a customer's software, vendors or setup in code, tests,
  comments, commits or PRs, and don't say "customer". Describe the behavior
  guaranteed ("a redirect to an allowlisted private IP is followed"); keep
  the specifics in private notes.
- Don't mirror a buildbuddy `go_deps` module_override into
  buildbuddy-internal by hand: the 🔄 sync commits that bump the buildbuddy
  pin carry it over.
