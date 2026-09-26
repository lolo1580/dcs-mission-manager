# Deployment

The manager deploys **in two ways, from the same project**: a Windows `.exe`,
or a Linux Docker image. The binary and the configuration are
identical; only the packaging changes.

> ⚠️ **DCS World never runs inside the container.** The simulator is
> Windows only. The container only contains the *manager* and communicates
> with DCS over the network.

## Mode A — Windows `.exe`

On the machine running DCS.

```powershell
# 1. Builder (frontend + backend)
.\build.ps1

# 2. Install the Lua scripts into DCS (safe merge in Saved Games)
.\install-dcs.ps1            # ou .\install-dcs.ps1 -DryRun pour simuler

# 3. Lancer le manager
.\dcsmm.exe
```

The default backend address is `127.0.0.1`: everything is on the same machine.

### As a Windows service (automatic start)

```powershell
# Requires NSSM or the Task Scheduler. Example with the Task Scheduler:
schtasks /create /tn "DCSMM" /tr "C:\chemin\dcsmm.exe" /sc onstart /ru SYSTEM
```

## Mode B — Docker Linux (another machine)

On a Linux server/NAS, with DCS on **another** machine.

```bash
docker compose -f deploy/docker-compose.yml up -d
```

### Multi-arch build (ARM NAS)

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f deploy/Dockerfile -t dcsmm:latest .
```

### The point not to miss

The Lua scripts run on **Windows**, so in `Saved Games\DCS\Config\dcsmm.cfg`
you must point to the **LAN IP of the Docker machine** — not `127.0.0.1`, which
would loop back to the Windows host:

```lua
dcsmm_host = "192.168.1.50"   -- IP of the machine hosting the container
dcsmm_udp_port = 7778
dcsmm_tcp_port = 7779
```

And open the ports in the Docker machine's firewall:

| Port | Protocol | Usage |
|---|---|---|
| 8080 | TCP | Web interface |
| 7778 | UDP | Positions (live map) |
| 7779 | TCP | Events, players, chat, debriefs |

> On a NAS or native Linux, no particular problem. On **Docker Desktop
> (Windows/WSL2)**, UDP publication to the LAN is sometimes temperamental;
> a `netsh portproxy` can serve as a fallback.

## Lua injector: `dcsmm install-lua`

DCS-side installation is **idempotent and safe**:

- it **never replaces** an existing `Export.lua` (Tacview, SRS, DCS-BIOS…);
- it **merges** a block delimited by markers
  (`>>> DCSMM-BEGIN >>>` … `<<< DCSMM-END <<<`);
- it creates a **timestamped backup** before any modification;
- re-running the installation simply updates the block in place.

```powershell
dcsmm status          # installed / outdated / missing, par fichier
dcsmm install-lua     # installs or updates
dcsmm uninstall-lua   # retire le bloc et Hooks/dcsmm.lua (garde la config)
```

Affected files:

| Target | Handling |
|---|---|
| `Scripts\Export.lua` | **merge by markers** (never overwritten) |
| `Scripts\Hooks\dcsmm.lua` | file specific to the manager, copied/replaced |
| `Config\dcsmm.cfg` | created if missing; never overwritten (holds the IP/ports) |

## Configuration

A single configuration base for both modes: `DCSMM_*` variables
(see `README.md`) and `Saved Games\DCS\Config\dcsmm.cfg` on the DCS side.

## Checking that everything is connected

1. Start the manager, open `http://<machine>:8080`.
2. In DCS, load a mission.
3. The header should switch to **“connected”**, the units should appear on the map,
   and the **Session** tab should fill up.
4. At the end of the mission, the **Debriefs** tab receives the `debrief.log`.
