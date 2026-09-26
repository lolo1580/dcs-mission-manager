# DCS Mission Manager

Un gestionnaire tout-en-un pour **DCS World** : live map en temps réel, lecture des
débriefings, et statistiques avancées. Déployable soit en **`.exe` Windows**, soit en
**image Docker** — à partir du **même projet**.

> ⚠️ **DCS World ne tourne jamais dans Docker.** Le simulateur reste sur Windows.
> Le conteneur ne contient que le *manager* (backend + Web UI), qui communique avec
> DCS par le réseau. Voir [Architecture](#architecture).

---

## Sommaire

- [Fonctionnalités](#fonctionnalités)
- [Architecture](#architecture)
- [Prérequis](#prérequis)
- [Démarrage rapide (PoC Phase 0)](#démarrage-rapide-poc-phase-0)
- [Configuration](#configuration)
- [Installation des scripts Lua dans DCS](#installation-des-scripts-lua-dans-dcs)
- [Déploiement](#déploiement)
- [Structure du projet](#structure-du-projet)
- [Roadmap](#roadmap)
- [Licence](#licence)

---

## Fonctionnalités

| Fonction | État | Détail |
|---|---|---|
| Live map temps réel | ✅ Phase 1 | Tous les objets, catégories, filtres, traces, recherche |
| Tuiles DCS authentiques | 📋 Prévu | Exporteur de tuiles F10 (dossier `tiles/`) |
| Événements & joueurs | ✅ Phase 2 | Kills, crashes, chat, joueurs, historique SQLite |
| Débriefings | ✅ Phase 3 | Envoi réseau de `debrief.log`, parseur Lua, historique |
| Contrôle serveur | 🚧 Partiel | Chat vers DCS (canal de commandes) à venir |
| Stats avancées | ✅ Phase 4 | Pilotes, armes, engins, balance, réseau (carrière + mission) |
| Cartes analytiques & sortie | ✅ Phase 4 bis | Heatmaps, traces, analyse de sortie, télémétrie ownship |
| Aérodromes | ✅ Phase 6 | 21 terrains du Caucase : coordonnées, Tower, TACAN, ILS, cartes |
| Fog of war | ✅ Phase 7 | Respect des options de mission F10 (filtrage côté serveur) |

### Statistiques avancées (prévues)

- **Fiche pilote & carrière** — kills/morts/KD, éjections, crashes, temps de vol, par **UCID**
- **Analyse d'armes** — efficacité par arme, matrice de kills, friendly-fire
- **Cartes analytiques** — heatmaps kills/morts, traces de vol rejouables
- **Balance & méta** — balance coalition, appareils joués, timeline de mission
- **Analyse de sortie** — durée, distance, altitude/vitesse/G max (telemetry)
- **Qualité réseau** — ping, déconnexions, codes d'erreur
- **Analyse par engin** — granularité **type DCS exact** (`F-16C_50`, `T-72B`, `SA-10`…), plateforme + cible + matchups

---

## Architecture

```
     MACHINE A (Windows — DCS)                    MACHINE B (Linux/NAS — Docker)
┌──────────────────────────────┐            ┌──────────────────────────────────┐
│ DCS World                     │  UDP + TCP │ Backend Go (conteneur)            │
│  Config/dcsmm.cfg  ─ IP(B) ──┼───────────►│  • listener UDP (positions)       │
│  Scripts/Export.lua  → pos    │◄───────────┼─ • canal TCP (events + commandes) │
│  Scripts/Hooks/dcsmm.lua      │            │  • état + SQLite (volume)         │
│   → events/joueurs/chat       │            │  • parse debrief (reçu réseau)    │
│   → debrief.log (lu + envoyé) │            │  • REST + SSE                     │
└──────────────────────────────┘            └───────────────┬──────────────────┘
                                              HTTP/WS ──────┘
                                       Navigateurs LAN (machine B ou autres)
```

### Un seul projet, deux emballages

```
              UN SEUL CODE SOURCE (Go + Svelte embarquée)
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
   Mode A : dcsmm.exe (machine A)     Mode B : image Docker (machine B)
   même machine que DCS               Linux/NAS, accès LAN
   IP backend = 127.0.0.1             IP backend = IP LAN de B
```

**Stack :** Go (backend, binaire unique + UI embarquée) · Svelte + Vite + Leaflet (frontend) · SQLite (persistance).

---

## Prérequis

### Côté DCS (machine A, Windows)

- DCS World (dernière version stable ou open beta)
- Accès à `%USERPROFILE%\Saved Games\DCS\` (ou `DCS.openbeta`)
- LuaSocket — fourni avec DCS, aucune installation requise

### Côté développement / manager

- **Go 1.22+** — <https://go.dev/dl/> (`winget install GoLang.Go`)
- **Node.js 20+** — <https://nodejs.org/> (uniquement pour builder le frontend)
- **Git**

### Côté Docker (machine B, Linux/NAS)

- Docker Engine 24+ et Docker Compose v2
- Architecture `amd64` ou `arm64` (build multi-arch fourni)

---

## Démarrage rapide (PoC Phase 0)

Le PoC valide toute la chaîne : **DCS → UDP → Go → SSE → navigateur**.

### 1. Lancer le manager

```powershell
# Backend seul (sert aussi un placeholder si le frontend n'est pas buildé)
go run ./backend/cmd/dcsmm
```

Par défaut, le backend écoute :

- `127.0.0.1:7778` en **UDP** (positions)
- `0.0.0.0:8080` en **HTTP** (Web UI + flux temps réel `GET /api/events` en SSE)

Ouvre ensuite <http://localhost:8080>.

### 2. Installer les scripts Lua dans DCS

Copie les fichiers de `dcs-lua/` vers ton dossier Saved Games — voir
[Installation des scripts Lua](#installation-des-scripts-lua-dans-dcs). Le PoC n'a besoin
que de `Export.lua` et de `Config/dcsmm.cfg`.

### 3. Lancer DCS et une mission

Ton appareil apparaît comme un point sur la carte, mis à jour une fois par seconde.

> En mode Docker sur une autre machine, remplace `127.0.0.1` par l'IP LAN de la machine B
> dans `Saved Games\DCS\Config\dcsmm.cfg`.

---

## Configuration

Toute la configuration passe par **variables d'environnement** (préfixe `DCSMM_`) avec des
défauts raisonnables — identique pour l'`.exe` et pour Docker.

| Variable | Défaut | Description |
|---|---|---|
| `DCSMM_HTTP_ADDR` | `0.0.0.0:8080` | Adresse d'écoute HTTP (Web UI + SSE) |
| `DCSMM_UDP_ADDR` | `127.0.0.1:7778` | Adresse d'écoute UDP (télémétrie Live map) |
| `DCSMM_TCP_ADDR` | `127.0.0.1:7779` | Adresse d'écoute TCP (events + commandes) |
| `DCSMM_DB_PATH` | `./data/dcsmm.db` | Chemin de la base SQLite |
| `DCSMM_DB_ENABLED` | `true` | Activer la persistance (sinon tout en mémoire) |
| `DCSMM_THEATRE` | `Caucasus` | Théâtre par défaut |
| `DCSMM_UNIT_TTL` | `5` (secondes) | Délai avant qu'une unité silencieuse disparaisse |
| `DCSMM_TILES_DIR` | `./tiles` | Dossier des tuiles de carte DCS |
| `DCSMM_BASEMAP` | `satellite` | Fond par défaut : `satellite`, `topo`, `osm`, `dark` |
| `DCSMM_BASEMAP_URL` | *(vide)* | Fond personnalisé optionnel (template `{z}/{x}/{y}`) |
| `DCSMM_CATEGORIES` | `./categories.json` | Surcharge de classification des engins |
| `DCSMM_MAX_UNITS` | `5000` | Nombre maximum d'unités suivies |
| `DCSMM_TRACK_INTERVAL` | `3` (secondes) | Fréquence d'échantillonnage des positions |
| `DCSMM_TRACK_GRACE` | `15` (secondes) | Absence avant de compter une unité comme perdue |
| `DCSMM_TRACK_RETENTION` | `86400` (secondes) | Durée de conservation de l'historique |
| `DCSMM_REVEAL_ALL_UNITS` | `false` | Désactive le fog of war (tout diffuser ; solo/conception) |
| `DCSMM_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Côté DCS — `Saved Games\DCS\Config\dcsmm.cfg`

```lua
-- Adresse du backend (IP LAN de la machine Docker, ou 127.0.0.1 en local)
dcsmm_host = "127.0.0.1"
dcsmm_udp_port = 7778
dcsmm_tcp_port = 7779

-- Live map
dcsmm_send_interval = 1.0    -- position du joueur (secondes)
dcsmm_world_enabled = true   -- export de tous les objets
dcsmm_world_interval = 2.0   -- liste des objets (secondes)
dcsmm_world_radius = 0       -- filtre par rayon en km (0 = tout)
```

---

## Installation des scripts Lua dans DCS

> ⚠️ **Toujours merger, jamais écraser.** `Export.lua` est très souvent déjà modifié par
> Tacview, SRS, DCS-BIOS, etc. Sauvegarde le fichier existant avant toute modification.

Les scripts se placent dans le dossier *Saved Games* de DCS :

```
%USERPROFILE%\Saved Games\DCS\          (ou DCS.openbeta)
├─ Config\
│   └─ dcsmm.cfg          ← configuration de l'adresse backend
└─ Scripts\
    ├─ Export.lua         ← positions (live map) ; à MERGER avec l'existant
    └─ Hooks\
        └─ dcsmm.lua      ← événements, joueurs, chat (Phases 2+)
```

1. Copie `dcs-lua/Config/dcsmm.cfg` dans `Saved Games\DCS\Config\`.
2. Si `Saved Games\DCS\Scripts\Export.lua` existe déjà : fais-en une copie
   (`Export.lua.bak-AAAAMMJJ`), puis ajoute le bloc `do ... end` fourni dans
   `dcs-lua/Export.lua` **à la fin** du fichier existant.
3. Sinon, copie simplement `dcs-lua/Export.lua`.
4. Redémarre DCS.

---

## Déploiement

### Mode A — `.exe` Windows (machine DCS)

```powershell
# 1. Builder le frontend et le backend
.\build.ps1

# 2. Installer les scripts côté DCS (fusion sûre dans Saved Games)
.\install-dcs.ps1            # ajouter -DryRun pour simuler

# 3. Lancer
.\dcsmm.exe
```

### Mode B — Docker (machine B, Linux/NAS)

```bash
docker compose -f deploy/docker-compose.yml up -d
```

Ports publiés : `8080/tcp` (UI), `7778/udp` (positions), `7779/tcp` (events + commandes).
Un volume persiste la base SQLite.

> Sur la machine A, pointe `dcsmm_host` vers l'**IP LAN de la machine B** (pas `127.0.0.1`)
> et autorise les ports dans le pare-feu.

Build multi-arch pour NAS ARM :

```bash
make docker-multiarch
# ou directement :
docker buildx build --platform linux/amd64,linux/arm64 -f deploy/Dockerfile -t dcsmm:latest .
```

### CLI

```powershell
dcsmm                 # lance le manager (interface web + réception DCS)
dcsmm install-lua     # installe/fusionne les scripts Lua dans Saved Games
dcsmm uninstall-lua   # retire le bloc installé (garde la config)
dcsmm status          # installed / outdated / missing, par fichier
dcsmm version
```

---

## Structure du projet

```
DCS mission manager/
├─ README.md
├─ CHANGELOG.md
├─ VERSION
├─ Makefile / build.ps1       # commandes de build
├─ install-dcs.ps1           # installe les scripts Lua dans Saved Games
├─ dcs-lua/                  # scripts à installer côté DCS
│   ├─ Config/dcsmm.cfg      # modèle de configuration
│   ├─ Export.lua            # positions → UDP (live map)
│   └─ Hooks/dcsmm.lua       # events / joueurs / chat
├─ backend/                  # Go
│   ├─ go.mod
│   ├─ cmd/dcsmm/main.go     # serveur + CLI (install/uninstall/status)
│   └─ internal/
│       ├─ config/           # chargement env + défauts
│       ├─ install/          # injecteur Lua (fusion par marqueurs)
│       ├─ aerodrome/        # aérodromes et fréquences (données embarquées)
│       ├─ category/         # classification des engins (type DCS → famille)
│       ├─ theatre/          # théâtres DCS et leurs emprises
│       ├─ basemap/          # fonds de carte (satellite, relief, osm, sombre)
│       ├─ model/            # types échangés DCS ↔ backend
│       ├─ lua/              # parseur de données Lua (debrief.log)
│       ├─ debrief/          # analyse des débriefs
│       ├─ debriefstore/     # réassemblage des transferts de débrief
│       ├─ udp/              # récepteur positions (live map)
│       ├─ tcp/              # récepteur événements / joueurs / chat
│       ├─ live/             # état de session en mémoire
│       ├─ ingest/           # pont live → base de données
│       ├─ tracker/          # historique positions + détection de pertes
│       ├─ db/               # persistance SQLite (pur Go)
│       ├─ state/            # store unités (en mémoire)
│       ├─ stats/            # agrégations statistiques
│       └─ api/              # REST + SSE + tuiles + UI embarquée (dist/)
├─ frontend/                 # Svelte + Vite + Leaflet
│   └─ src/
│       ├─ App.svelte
│       └─ lib/              # carte, panneaux, stores
├─ tiles/                    # tuiles DCS par théâtre
├─ tools/                    # émetteur de télémétrie de test, extracteur de tuiles
├─ deploy/                   # Dockerfile + docker-compose.yml
└─ docs/                     # documentation
```

---

## Roadmap

- [x] **Phase 0 — PoC** : `Export.lua` (position joueur) → Go → carte Leaflet
- [x] **Phase 1 — Live map** : tous les objets, catégories, filtres, traces, recherche, tuiles DCS
- [x] **Phase 2 — Événements & joueurs** : `onGameEvent`, chat, `net.get_stat`, historique SQLite
- [x] **Phase 3 — Débriefings** : envoi réseau de `debrief.log`, parseur Lua, historique
- [x] **Phase 4 — Stats avancées** : vue d'ensemble, pilotes, armes, engins, balance, réseau
- [x] **Phase 4 bis — Cartes analytiques & sortie** : heatmaps, traces, télémétrie
- [x] **Phase 5 — Packaging** : CLI, injecteur Lua sûr, `.exe` + Docker multi-arch
- [x] **Phase 6 — Aérodromes** : 21 terrains du Caucase (fréquences, cartes)
- [x] **Phase 7 — Fog of war** : respect des options F10 de la mission (filtrage serveur)

Le plan complet et détaillé est disponible dans le fichier de plan du projet.

---

## Licence

À définir.
