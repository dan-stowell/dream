# BuildBuddy reviewer map

For choosing reviewers on Dan's (dan-stowell) PRs. Repos: `bb` = buildbuddy-io/buildbuddy,
`int` = buildbuddy-io/buildbuddy-internal. Evidence = merged PRs May 19 to Nov 19 2026:
"Nc" = N PRs authored touching the area, "Nr" = N PRs reviewed (approved or commented), from git
history plus PR reviews. "(Dan)" = Dan's own statement, which wins over the numbers.
`int` gets 🔄 commits that sync `bb` merges; they are not counted. Dan himself is left out of the
rankings, though he is a top reviewer in OCI, images, release and Bazel config.

## RBE scheduler and executors
bb: `enterprise/server/remote_execution/` (executor, runner, workspace, filecache, commandutil),
`enterprise/server/scheduling/`, `enterprise/server/cmd/executor/`, `enterprise/server/tasksize/`
1. bduffany: RBE machinery (Dan), 68c 62r. Overloaded (Dan)
2. vadimberezniker: can review (Dan), 49r; top reviewer on executor/ (21) and scheduler (19). Overloaded (Dan)
3. vanja-p: 59r here, top scheduler reviewer (22r). The best choice when the other two are busy
4. tylerwilliams: 17c 22r

## Firecracker, VFS, snapshots
bb: `.../containers/firecracker/`, `.../remote_execution/{vfs,vbd,uffd,snaploader,snaputil,copy_on_write}/`,
`enterprise/server/{vmexec,vmvfs,vmdns}/`, `enterprise/server/util/{firecrackerutil,vfs_server,ext4,vsock}/`, `enterprise/vmsupport/`
1. bduffany: Firecracker (Dan), 25c 47r
2. maggie-lou: 19c 20r; top author in snapshots (snaploader/snaputil 13c)
3. tylerwilliams: the most firecracker/ commits (16) and VFS commits (7)
4. vadimberezniker: (Dan) 13r; ties for top VFS reviewer (9r)

## OCI runtime, fetcher, registry
bb: `.../containers/{ociruntime,podman,docker}/`, `enterprise/server/oci/`, `enterprise/server/registry/`,
`enterprise/server/ocifetcher_server_proxy/`, `enterprise/server/util/{oci,ocimanifest}/`
1. bduffany: OCI runtimes (Dan), 13c 29r; reviews almost all fetcher/registry PRs
2. vadimberezniker: (Dan) 12r
3. vanja-p: 8r. (Dan writes most fetcher/registry code: 17 of 30 PRs.)

## Container images (rules_img, distroless, executor images)
bb: `images/`, `apt/`, `dockerfiles/`, `enterprise/dockerfiles/`, `deps/apt.MODULE.bazel`;
int: `enterprise/dockerfiles/`, `tools/build_rules_img.bzl`
1. tempoz (Zoey Greer): rules_img executor images (Dan), 10 of 19 image PRs
2. keith / fmeum: 3c / 3r. There is no real second reviewer: Dan reviewed 9 of the 19

## Cache (local, distributed, raft, pebble)
bb: `enterprise/server/backends/{distributed,pebble_cache,migration_cache,gcs_cache,s3_cache}/`, `enterprise/server/raft/`,
`enterprise/server/composable_cache/`, `server/remote_cache/`, `enterprise/server/remote_cache/`, `server/util/compression/`
1. vanja-p: cache, distributed cache, gRPC, Go perf, compression (Dan), 92c 90r
2. luluz66: raft only, 23c 27r on raft/ (the top raft reviewer)
3. vadimberezniker: 43r, second on pebble_cache (11r)
4. iain-macdonald: 42c; second on distributed/ (9c 10r). tyler-french: 14c in remote_cache/

## Cache proxy
bb: `enterprise/server/cache_proxy*`, `enterprise/server/*_server_proxy/`, `enterprise/server/cmd/cache_proxy/`, `enterprise/server/util/proxy_util/`
1. iain-macdonald: (Dan) 53c 25r
2. vanja-p: 15c 28r
3. jdhollen (Jim Hollenbach): 18r. tyler-french: 14c

## CLI
bb: `cli/`
1. maggie-lou: 66 of 142 CLI PRs, the clear owner (Dan did not list this)
2. bduffany: 41r
3. siggisim: 24r. fmeum: 22r. tylerwilliams: 16c

## Remote Bazel and workflows
bb: `enterprise/server/{workflow,hostedrunner,githubapp}/`, `enterprise/server/cmd/ci_runner/`, `cli/remotebazel/`, `enterprise/app/workflows/`
1. maggie-lou: (Dan) 28c 14r
2. bduffany: helps here (Dan), 10c 21r
3. fmeum: 8r. siggisim: 8r

## Auth, API keys, claims, secrets
bb: `enterprise/server/{auth,auth_service,remoteauth,selfauth,oidc,saml,scim,secrets,api}/`,
`enterprise/server/backends/authdb/`, `server/util/{claims,authutil,perms,role}/`, `enterprise/app/api_keys/`
1. iain-macdonald: the top author, 21c
2. vadimberezniker: 16r. bduffany: 9c 16r
3. siggisim: 8c 11r (wrote much of the original auth)

## Usage and billing
bb: `enterprise/server/{usage,usage_service,usagealert,billing,quota}/`, `server/usage/`, `enterprise/app/usage/`;
int: `tools/usage/`, `tools/business_metrics/`
1. bduffany: 9c 18r. vanja-p: 8c 12r (also 10r on int usage tools)
2. iain-macdonald: 14c in int usage tools. sluongng: 9c in bb
No clear owner; work is spread thin. Ask bduffany or vanja-p, and say it touches billing.

## UI and website
bb: `app/`, `enterprise/app/`, `website/`
1. siggisim: 52r (the top UI reviewer)
2. bduffany: 39c 46r
3. jdhollen: 19c 14r. iain-macdonald: 32c in enterprise/app/

## Bazel config and build tooling
bb: `.bazelrc`, `shared.bazelrc`, `MODULE.bazel`, `deps/`, `buildpatches/`, `rules/`, `platforms/`; same files in int
1. bduffany: 33c 55r
2. fmeum (Fabian Meumertzheim, Bazel expert): 47r, a frequent reviewer here and across bb
3. keith: open source / Bazel-flavored (Dan), 34c. sluongng: 49c (dep bumps, Go modernization)
4. tempoz: 29c (deps, pnpm, rules_distroless)

## Release and deploy
bb: `.github/workflows/`, `release.py`, `deployment/`; int: `.github/workflows/`, `tools/release/`, `tools/k8s.bzl`
1. bduffany: 10r bb, 18r int
2. jdelfino: 22c int release. iain-macdonald: 22c int release
3. sluongng: 8c 6r bb. Dan reviews most release PRs himself, so there is no strong second

## Macs
int: `enterprise/deployment/macstadium/`, `enterprise/mdm/`, Mac workflows (`run-macos-ansible`, `update-mac-executors`, `clean-and-reboot-*`)
1. keith: (Dan) 114 of 198 Mac PRs
2. iain-macdonald: (Dan) 65c 87r
3. siggisim: 29r

## Infra / k8s / metal (internal)
int: `enterprise/deployment/` (prod, dev, metal), `enterprise/config/`, `experiments/`, `tools/{gke,eks}/`
1. vadimberezniker: 186c 256r; owns metal (95 of 162 metal PRs)
2. iain-macdonald: 148c 118r. vanja-p: 186r, top `experiments/` reviewer (94r)
3. tylerwilliams: 105c 126r. siggisim: 121r

## Other areas found
- bb `codesearch/`: tylerwilliams 27 of 34 PRs; siggisim reviews (26r)
- bb `enterprise/gateway/`, `enterprise/dns/` (relay gateway, DNS): tylerwilliams 18c, vadimberezniker 7c; siggisim 16r
- bb `enterprise/atlas/`: vadimberezniker 18 of 20; bduffany, vanja-p review (10r each)
- int `enterprise/agentsandbox/`, `enterprise/alertexplainer/`: jdelfino authors all; bduffany reviews
- Broad, early code everywhere (Dan): tylerwilliams (116c bb) and siggisim (mostly reviews now)
- sluongng: 99c bb, spread everywhere (Go modernization, deps, CLI, executor)

## Checking review load (do this; don't trust this file)
Load changes daily. Before you pick someone, count their open review requests:
`gh search prs --review-requested=LOGIN --state=open --owner=buildbuddy-io --json number,title,repository`
(add `--limit 100`; repeat per candidate). Also check `gh search prs --author=LOGIN --state=open` for their own work in flight.
Prefer the less-loaded of the top two. bduffany and vadimberezniker are overloaded (Dan); move to the next name unless the change really needs them.
