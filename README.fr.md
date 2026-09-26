# DCS Mission Manager

[🇬🇧 English](README.md) | 🇫🇷 Français

Un gestionnaire tout-en-un pour **DCS World** : live map en temps réel, lecture des
débriefings, et statistiques avancées. Il tourne **en local, sur la même machine
Windows que DCS** : un seul `dcsmm.exe`, ni serveur, ni conteneur, rien à configurer.

Parce qu'il est local, il peut lire les **données de terrain de DCS lui-même** — les
aérodromes, fréquences et balises de chaque carte installée — au lieu de dépendre
d'un jeu de données maintenu à la main.

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
| Fonds de carte | ✅ Phase 8 | Satellite, Relief, Routier, **Aéronautique**, Sombre — tous sans clé |
| Théâtre & étendue | ✅ Phase 8 | 15 cartes DCS : cadrage, contour de l'étendue, aérodromes par théâtre |
| Tuiles DCS authentiques | 📋 Prévu | Exporteur de tuiles F10 (dossier `tiles/`) |
| Événements & joueurs | ✅ Phase 2 | Kills, crashes, chat, joueurs, historique SQLite |
| Débriefings | ✅ Phase 3 | Envoi réseau de `debrief.log`, parseur Lua, historique |
| Contrôle serveur | 🚧 Partiel | Chat vers DCS (canal de commandes) à venir |
| Stats avancées | ✅ Phase 4 | Pilotes, armes, engins, balance, réseau (carrière + mission) |
| Cartes analytiques & sortie | ✅ Phase 4 bis | Heatmaps, traces, analyse de sortie, télémétrie ownship |
| Aérodromes | ✅ Phase 6 | Lus depuis les fichiers de terrain de DCS : **101 aérodromes listés, 69 plaçables** sur 5 cartes installées, avec Tower/TACAN/ILS/VOR/RSBN/NDB, affichés sur la carte avec fiche au clic |
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
                      WINDOWS (une seule machine)
┌──────────────────────────────────────────────────────────┐
│ DCS World                                                 │
│  Scripts/Export.lua      → positions (UDP/JSON)           │
│  Scripts/Hooks/dcsmm.lua → events/joueurs/chat (TCP/JSON) │
└──────────────┬───────────────────────────────────────────┘
               │ 127.0.0.1 — rien à configurer
               ▼
┌──────────────────────────────────────────────────────────┐
│ dcsmm.exe — backend Go + UI web embarquée                 │
│  • listeners UDP + TCP, état en mémoire, SQLite (data/)   │
│  • parseur de débrief, statistiques, données aérodromes   │
│  • lit Mods/terrains/ et Saved Games/ directement         │
│  • REST + SSE sur http://localhost:8080                   │
└──────────────────────────────────────────────────────────┘
```

### Local par conception

```
   dcsmm.exe tourne SUR LA MÊME MACHINE WINDOWS QUE DCS
   → aucune adresse LAN à configurer, aucune règle de pare-feu, aucun conteneur
   → accès direct à Mods/terrains/ et à Saved Games/
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

---

## Configuration

Toute la configuration passe par **variables d'environnement** (préfixe `DCSMM_`) avec des
défauts raisonnables. Aucune n'est nécessaire pour une installation normale.

| Variable | Défaut | Description |
|---|---|---|
| `DCSMM_HTTP_ADDR` | `127.0.0.1:8080` | Adresse d'écoute HTTP (Web UI + SSE). Mettre `0.0.0.0:8080` pour atteindre l'UI depuis un autre appareil ; l'API est alors sans authentification |
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
| `DCSMM_SAVED_GAMES` | *(auto)* | Dossier Saved Games de DCS, si la détection échoue |
| `DCSMM_CHARTS_DIR` | `./maps_dcs` | Scans de cartes aéronautiques (approches, plans de mouvement) |
| `DCSMM_REVEAL_ALL_UNITS` | `false` | Désactive le fog of war (tout diffuser ; solo/conception) |
| `DCSMM_SOURCE` | *(auto)* | Force la source de la session : `live` ou `test` (voir plus bas) |
| `DCSMM_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Côté DCS — `Saved Games\DCS\Config\dcsmm.cfg`

```lua
-- Adresse du backend (le manager tourne en local)
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

### `.exe` Windows

```powershell
# 1. Builder le frontend et le backend
.\build.ps1

# 2. Installer les scripts côté DCS (fusion sûre dans Saved Games)
.\install-dcs.ps1            # ajouter -DryRun pour simuler

# 3. Lancer
.\dcsmm.exe
```

L'interface s'ouvre sur <http://localhost:8080>.

### CLI

```powershell
dcsmm                 # lance le manager (interface web + réception DCS)
dcsmm install-lua     # installe/fusionne les scripts Lua dans Saved Games
dcsmm uninstall-lua   # retire le bloc installé (garde la config)
dcsmm status          # installed / outdated / missing, par fichier
dcsmm purge           # supprime des sessions enregistrées (destructif)
dcsmm version
```

### Sessions de test et `purge`

Les outils de test (`tools/send-telemetry.mjs`, …) parlent exactement le même
protocole que DCS : une session enregistrée pendant qu'ils tournent est
indiscernable d'un vrai vol. Chaque session porte donc une **source** :

- `live` — enregistrée depuis DCS. Les statistiques les comptent.
- `test` — enregistrée depuis les outils de test. Conservée sur disque, mais
  **exclue des statistiques, de l'analyse et du tableau de bord** sauf demande
  explicite.

La détection est automatique : le backend reconnaît les indicatifs des fixtures
de `tools/send-telemetry.mjs`. Utilisez `DCSMM_SOURCE=test` (ou `live`) pour
forcer le verdict avec vos propres fixtures.

```powershell
dcsmm purge --source test            # supprime uniquement les sessions simulées
dcsmm purge --mission-id 3           # supprime une mission et tout ce qui y est lié
dcsmm purge --all                    # supprime toutes les sessions enregistrées
dcsmm purge --source test --dry-run  # affiche ce qui serait supprimé, sans rien faire
```

Les statistiques acceptent `?includeTest=1` pour inclure volontairement les
sessions simulées.

> En migrant depuis une version antérieure, les missions **existantes** sont
> étiquetées `live` (la migration ne peut pas savoir qu'elles étaient simulées).
> Pour effacer des données enregistrées avant cette fonction, utilisez
> `dcsmm purge --mission-id <n>` ou `--all`.

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
- [x] **Phase 5 — Packaging** : CLI, injecteur Lua sûr, binaire unique autonome
- [x] **Phase 6 — Aérodromes** : 21 terrains du Caucase (fréquences, cartes)
- [x] **Phase 7 — Fog of war** : respect des options F10 de la mission (filtrage serveur)

Le plan complet et détaillé est disponible dans le fichier de plan du projet.

---

## Licence

[MIT](LICENSE) © 2026 Laurent (lolo1580)

Projet communautaire, non affilié à Eagle Dynamics et non approuvé par cet
éditeur. « DCS World » et ses terrains sont des marques d'Eagle Dynamics SA.
