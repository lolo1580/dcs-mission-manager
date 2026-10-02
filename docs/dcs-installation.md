# DCS-side installation

## Where the files are

DCS loads two families of scripts from the *Saved Games* folder:

```
%USERPROFILE%\Saved Games\DCS\          (ou DCS.openbeta)
├─ Config\
│   └─ dcsmanager.cfg          ← adresse du backend
└─ Scripts\
    ├─ Export.lua         ← positions (telemetry)
    └─ Hooks\
        └─ dcsmanager.lua      ← events, players, chat
```

## Step 1 — Configuration

Copy `dcs-lua/Config/dcsmanager.cfg` to `Saved Games\DCS\Config\dcsmanager.cfg`, then adapt:

```lua
dcsmanager_host = "127.0.0.1"   -- the manager runs locally, nothing to change
dcsmanager_udp_port = 7776      -- DCS-BIOS owns 7778; keep them apart
dcsmanager_tcp_port = 7779
dcsmanager_enabled = true
dcsmanager_send_interval = 1.0
```

> **Running alongside DCS-BIOS.** DCS-BIOS listens for commands on UDP 7778, so
> the manager uses 7776 for its own telemetry. That is what lets the two run at
> the same time; changing this port back to 7778 would make one of them fail to
> start.

## Step 2 — Export.lua (telemetry)

> ⚠️ **Never overwrite** an existing `Export.lua`. Tacview, SRS and DCS-BIOS all
> add their own code to it.

### Case A — you don't yet have a `Saved Games\DCS\Scripts\Export.lua`

Simply copy `dcs-lua/Export.lua` to that location.

### Case B — an `Export.lua` already exists

1. Make a timestamped backup:
   ```powershell
   Copy-Item "$env:USERPROFILE\Saved Games\DCS\Scripts\Export.lua" `
             "$env:USERPROFILE\Saved Games\DCS\Scripts\Export.lua.bak-$(Get-Date -Format yyyyMMdd)"
   ```
2. Open `dcs-lua/Export.lua` and copy **the entire `do ... end` block** (inner lines,
   without the header comments).
3. Paste it **at the end** of your existing `Export.lua`.
4. Check that there is only one pair of active `LuaExportStart` /
   `LuaExportActivityNextEvent` functions. If your file already defines one, merge
   the content of `sendOwnship()` into it.

## Step 3 — Hooks (Phase 2, optional for now)

Copy `dcs-lua/Hooks/dcsmanager.lua` to `Saved Games\DCS\Scripts\Hooks\dcsmanager.lua`.
DCS automatically loads all the `.lua` files in this folder, sorted by name.

## Step 4 — Restart DCS

Start a mission. In the DCS logs, you should see:

```
DCSMANAGER: position export enabled (127.0.0.1:7776)
```

## Troubleshooting

| Symptom | Clue |
|---|---|
| Nothing in the logs | `dcsmanager.cfg` missing or misplaced (it must be in `Config\`) |
| `LuaSocket introuvable` | Incomplete DCS installation; LuaSocket ships with DCS |
| The dot does not appear | Backend not started, or a firewall is blocking 127.0.0.1 |
| The dot is jerky in game | Increase `dcsmanager_send_interval` |

## Test without DCS

A test emitter is provided:

```bash
node tools/send-telemetry.mjs 127.0.0.1 7776
```

It simulates four aircraft flying around the Caucasus.
