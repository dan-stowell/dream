# vm-web

A web app that runs on an exe.dev VM (here `spawn`). It does what the `vm`
script does from a laptop: it creates classic and dispatcher VMs that run
claude with Remote Control, sends them prompts, backs up their transcripts
to `~/vm-transcripts`, and deletes them.

## What it can touch

- **exe.dev:** only `ls`, `new` and `rm`, through a token held in an
  HTTP-proxy integration. The token is never on spawn.
- **The VMs:** no SSH. Each VM it makes runs `vm-child` as a service
  (`vm-agent`). The service polls spawn for jobs (setup, start or prompt,
  login, backup) through a peer integration. exe.dev names the calling VM
  in `X-Exedev-Source-Vm`, which can't be forged. Agents use their own port
  (8001), and the UI port refuses any request that carries that header.
- **Secrets:** a GitHub token and a BuildBuddy API key live in
  `~/.config/vm/secrets` on spawn. Setup hands them to VMs this app
  manages, as the `vm` script does.

## One-time setup

From your laptop:

```sh
# 1. A token for exe.dev's API that can only list, create and delete VMs.
#    Anyone using it can delete any of your VMs, so it's stored in an
#    integration rather than on spawn.
ssh exe.dev ssh-key generate-api-key --label=spawn --cmds=ls,new,rm --exp=365d
printf '%s' 'exe1.…' | ssh exe.dev integrations add http-proxy --name spawn-api \
  --target https://exe.dev --bearer=- --attach vm:spawn

# 2. Let VMs tagged spawn reach the agent port.
ssh exe.dev integrations add http-proxy --name spawn \
  --target https://spawn.exe.xyz:8001/ --peer --attach tag:spawn
```

On spawn:

```sh
(umask 077; cat >~/.config/vm/secrets) <<'EOS'
VM_GITHUB_TOKEN=…
VM_BUILDBUDDY_API_KEY=…
EOS
sudo cp ~/dream/web/vm-web.service /etc/systemd/system/
sudo systemctl enable --now vm-web
```

Then open https://spawn.exe.xyz:8000/.

## VMs from the vm script

**Adopt** puts one under this app's management. Its agent still has to be
installed from your laptop; the UI shows the two commands. After that it
gets hourly backups, and prompts and delete work from the web UI.

## Behavior carried over from `vm`

- The notes, agents and coordinator and dispatcher prompts come from the
  repo on each setup, so edit them here.
- A dispatcher has `~/.config/vm/dispatcher.md`, named for its VM, and
  starts with "Good morning!…" if no prompt is given. **Make classic** /
  **Make dispatcher** switches a VM. A running claude keeps its old system
  prompt until it's restarted.
- The first start on a VM shows a claude.ai sign-in link. Paste the code
  back into the Activity panel.
- `vm-child` updates itself on each VM when it changes here.

Not carried over: `--repo` mode (VMs on a GitHub integration with no token).
