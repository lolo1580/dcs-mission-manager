# DCS-side installation

## Where the files are

DCS loads two families of scripts from the *Saved Games* folder:

```
%USERPROFILE%\Saved Games\DCS\          (ou DCS.openbeta)
├─ Config\
│   └─ dcsmm.cfg          ← adresse du backend
└─ Scripts\
    ├─ Export.lua         ← positions (live map)
    └─ Hooks\
        └─ dcsmm.lua      ← events, players, chat
```

## Step 1 — Configuration

Copy `dcs-lua/Config/dcsmm.cfg` to `Saved Games\DCS\Config\dcsmm.cfg`, then adapt:

```lua
dcsmm_host = "127.0.0.1"   -- the manager runs locally, nothing to change
dcsmm_udp_port = 7778
dcsmm_tcp_port = 7779
dcsmm_enabled = true
dcsmm_send_interval = 1.0
```

## Step 2 — Export.lua (live map)

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

Copy `dcs-lua/Hooks/dcsmm.lua` to `Saved Games\DCS\Scripts\Hooks\dcsmm.lua`.
DCS automatically loads all the `.lua` files in this folder, sorted by name.

## Step 4 — Restart DCS

Start a mission. In the DCS logs, you should see:

```
DCSMM: position export enabled (127.0.0.1:7778)
```

## Troubleshooting

| Symptom | Clue |
|---|---|
| Nothing in the logs | `dcsmm.cfg` missing or misplaced (it must be in `Config\`) |
| `LuaSocket introuvable` | Incomplete DCS installation; LuaSocket ships with DCS |
| The dot does not appear | Backend not started, or a firewall is blocking 127.0.0.1 |
| The dot is jerky in game | Increase `dcsmm_send_interval` |

## Test without DCS

A test emitter is provided:

```bash
node tools/send-telemetry.mjs 127.0.0.1 7778
```

It simulates four aircraft flying around the Caucasus.
