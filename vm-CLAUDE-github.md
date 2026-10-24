## Repos

- REPOS, reached through exe.dev's GitHub integration at
  github.int.exe.xyz rather than github.com. There's no GitHub token on this
  VM, and only these repos are reachable.
- `git push` and `gh` both go through the integration (`GH_HOST` is already
  set to github.int.exe.xyz), so pass `-R OWNER/REPO` to `gh`. A public repo
  fetches from github.com, which is what `bb remote`'s runners clone; a
  private one can't be cloned by them, so `bb remote` won't work there.
- These repos don't define `--config=remote-minimal`, so wherever the notes
  above say to pass it, pass its flags yourself instead, after the bazel
  command: `bb remote test //some:target
  --remote_executor=grpcs://buildbuddy.buildbuddy.io
  --remote_cache=grpcs://buildbuddy.buildbuddy.io
  --bes_backend=grpcs://buildbuddy.buildbuddy.io
  --bes_results_url=https://buildbuddy.buildbuddy.io/invocation/
  --remote_download_minimal --jobs=100`.
- There's no lint run set up for these repos, so follow whatever formatting
  and lint tooling the repo's own docs (README, docs/) describe, running any
  bazel part of it the same way.
- Open issues in the repo the work is in. If it has no `agent-written` label,
  skip that label rather than creating it.
