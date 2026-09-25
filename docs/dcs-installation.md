# Installation côté DCS

## Où sont les fichiers

DCS charge deux familles de scripts depuis le dossier *Saved Games* :

```
%USERPROFILE%\Saved Games\DCS\          (ou DCS.openbeta)
├─ Config\
│   └─ dcsmm.cfg          ← adresse du backend
└─ Scripts\
    ├─ Export.lua         ← positions (live map)
    └─ Hooks\
        └─ dcsmm.lua      ← événements, joueurs, chat
```

## Étape 1 — Configuration

Copie `dcs-lua/Config/dcsmm.cfg` vers `Saved Games\DCS\Config\dcsmm.cfg`, puis adapte :

```lua
dcsmm_host = "127.0.0.1"   -- IP LAN de la machine du manager si Docker ailleurs
dcsmm_udp_port = 7778
dcsmm_tcp_port = 7779
dcsmm_enabled = true
dcsmm_send_interval = 1.0
```

## Étape 2 — Export.lua (live map)

> ⚠️ **Ne jamais écraser** un `Export.lua` existant. Tacview, SRS et DCS-BIOS y
> ajoutent tous leur propre code.

### Cas A — tu n'as pas encore de `Saved Games\DCS\Scripts\Export.lua`

Copie simplement `dcs-lua/Export.lua` à cet emplacement.

### Cas B — un `Export.lua` existe déjà

1. Fais une sauvegarde horodatée :
   ```powershell
   Copy-Item "$env:USERPROFILE\Saved Games\DCS\Scripts\Export.lua" `
             "$env:USERPROFILE\Saved Games\DCS\Scripts\Export.lua.bak-$(Get-Date -Format yyyyMMdd)"
   ```
2. Ouvre `dcs-lua/Export.lua` et copie **tout le bloc `do ... end`** (lignes internes,
   sans les commentaires d'en-tête).
3. Colle-le **à la fin** de ton `Export.lua` existant.
4. Vérifie qu'il n'y a qu'une seule paire de fonctions `LuaExportStart` /
   `LuaExportActivityNextEvent` active. Si ton fichier en définit déjà, fusionne
   le contenu de `sendOwnship()` dans les tiennes.

## Étape 3 — Hooks (Phase 2, optionnel pour l'instant)

Copie `dcs-lua/Hooks/dcsmm.lua` vers `Saved Games\DCS\Scripts\Hooks\dcsmm.lua`.
DCS charge automatiquement tous les `.lua` de ce dossier, triés par nom.

## Étape 4 — Redémarrer DCS

Lance une mission. Dans les logs DCS, tu devrais voir :

```
DCSMM: export des positions activé (127.0.0.1:7778)
```

## Dépannage

| Symptôme | Piste |
|---|---|
| Rien dans les logs | `dcsmm.cfg` absent ou mal placé (doit être dans `Config\`) |
| `LuaSocket introuvable` | Installation DCS incomplète ; LuaSocket est fourni avec DCS |
| Le point n'apparaît pas | Backend non lancé, pare-feu, ou mauvais `dcsmm_host` (Docker) |
| Le point saccadé en jeu | Augmenter `dcsmm_send_interval` |

## Test sans DCS

Un émetteur de test est fourni :

```bash
node tools/send-telemetry.mjs 127.0.0.1 7778
```

Il simule quatre appareils tournant autour du Caucase.
