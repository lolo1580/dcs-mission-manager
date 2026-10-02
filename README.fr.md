# DCS Manager

[🇬🇧 English](README.md) | 🇫🇷 Français

Un compagnon tout-en-un pour **DCS World** : lecture des débriefings,
statistiques avancées, analyse des sorties, référence des aérodromes et cartes
aéronautiques. Il tourne
**en local, sur la même machine Windows que DCS** : un seul `dcsmanager.exe`, ni serveur,
ni conteneur, rien à configurer. Il s'ouvre dans **sa propre fenêtre**, comme un
logiciel classique : pas de navigateur à lancer, aucune adresse à retenir.

Parce qu'il est local, il peut lire les **fichiers de DCS lui-même** — les
aérodromes, fréquences et balises de chaque carte installée, ainsi que le dossier
Saved Games — au lieu de dépendre d'un jeu de données maintenu à la main.

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
| Événements & joueurs | ✅ Phase 2 | Kills, crashes, chat, joueurs, historique SQLite |
| Débriefings | ✅ Phase 3 | Envoi réseau de `debrief.log`, parseur Lua, historique |
| Contrôle serveur | ✅ Phase 2+ | Envoi d'un message de chat dans DCS (canal de commandes) |
| Stats avancées | ✅ Phase 4 | Pilotes, armes, engins, balance, réseau (carrière + mission) |
| Cartes analytiques & sortie | ✅ Phase 4 bis | Heatmap et tracés de vol, analyse de sortie, télémétrie ownship |
| Aérodromes | ✅ Phase 6 | Lus depuis les fichiers de terrain de DCS : **101 aérodromes listés, 69 plaçables** sur 5 cartes installées, avec Tower/TACAN/ILS/VOR/RSBN/NDB, et leurs cartes |
| Cartes aéronautiques | ✅ Phase 6 | Approches et plans de mouvement indexés depuis `maps_dcs/` et affichés comme documents |
| Modules installés | ✅ Nouveau | Terrains, appareils, campagnes et packs techniques, lus depuis l'inventaire de DCS |
| Carrière | ✅ Nouveau | Le logbook du joueur : grade, escadrille, décorations, heures et kills par appareil |
| Bibliothèque de missions | ✅ Nouveau | Les `.miz` de Saved Games : théâtre, date, météo, taille |
| Installation DCS | ✅ Nouveau | Mods installés, état des scripts, `Export.lua` partagé |
| Configuration | ✅ Nouveau | Les options de DCS : graphismes, difficulté, VR, terrains désactivés |

> La carte temps réel (et son imagerie) a été **retirée**. La télémétrie des unités
> est toujours reçue et échantillonnée : elle alimente les statistiques, les heatmaps
> et l'onglet Aérodromes (aérodrome le plus proche). Le gestionnaire s'axe désormais
> sur la session, les débriefs, les statistiques, l'analyse et les aérodromes.

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
│  Scripts/Hooks/dcsmanager.lua → events/joueurs/chat (TCP/JSON) │
└──────────────┬───────────────────────────────────────────┘
               │ 127.0.0.1 — rien à configurer
               ▼
┌──────────────────────────────────────────────────────────┐
│ dcsmanager.exe — backend Go + UI web embarquée                 │
│  • listeners UDP + TCP, état en mémoire, SQLite (data/)   │
│  • parseur de débrief, statistiques, données aérodromes   │
│  • lit Mods/terrains/ et Saved Games/ directement         │
│  • REST + SSE, servis à la fenêtre native (WebView2)      │
│    — ou à un navigateur en mode `serve`                   │
└──────────────────────────────────────────────────────────┘
```

### Fenêtre native

L'UI web est la même qu'avant : elle est simplement affichée dans une **fenêtre
d'application** (composant WebView2, fourni avec Windows 10/11) au lieu d'un onglet.
Le serveur HTTP continue de tourner derrière la fenêtre — c'est lui qui sert l'API —
donc rien de ce qui existait n'est perdu : `dcsmanager serve` lance le manager en mode
« navigateur », par exemple pour l'atteindre depuis un second écran ou une tablette.

Le manager refuse de démarrer deux fois : si une instance répond déjà sur
`DCSMANAGER_HTTP_ADDR`, le mode fenêtre affiche un message et s'arrête, et `dcsmanager serve`
sort avec le code 3. En mode fenêtre, un journal est écrit dans `data/dcsmanager.log` (à
côté de la base), puisqu'un exécutable lancé au double-clic n'a pas de console.

### Local par conception

```
   dcsmanager.exe tourne SUR LA MÊME MACHINE WINDOWS QUE DCS
   → aucune adresse LAN à configurer, aucune règle de pare-feu, aucun conteneur
   → accès direct à Mods/terrains/ et à Saved Games/
```

**Stack :** Go (backend, binaire unique + UI embarquée) · Svelte + Vite (frontend) · SQLite (persistance).

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
go run ./backend/cmd/dcsmanager
```

Par défaut, le backend écoute :

- `127.0.0.1:7778` en **UDP** (positions)
- `0.0.0.0:8080` en **HTTP** (Web UI + flux temps réel `GET /api/events` en SSE)

Le manager s'ouvre alors dans une **fenêtre native**. En mode `serve`, il n'ouvre pas
de fenêtre et l'interface est à consulter sur <http://localhost:8080>.

### 2. Installer les scripts Lua dans DCS

Copie les fichiers de `dcs-lua/` vers ton dossier Saved Games — voir
[Installation des scripts Lua](#installation-des-scripts-lua-dans-dcs). Le PoC n'a besoin
que de `Export.lua` et de `Config/dcsmanager.cfg`.

### 3. Lancer DCS et une mission

Le gestionnaire prend la session en compte : joueurs, événements et chat
apparaissent en direct, et chaque mission est enregistrée pour les débriefs et
les statistiques.

---

## Configuration

Toute la configuration passe par **variables d'environnement** (préfixe `DCSMANAGER_`) avec des
défauts raisonnables. Aucune n'est nécessaire pour une installation normale.

| Variable | Défaut | Description |
|---|---|---|
| `DCSMANAGER_HTTP_ADDR` | `127.0.0.1:8080` | Adresse d'écoute HTTP (Web UI + SSE). Un port `0` en choisit un libre automatiquement (fenêtre native). Mettre `0.0.0.0:8080` pour atteindre l'UI depuis un autre appareil ; l'API est alors sans authentification |
| `DCSMANAGER_UDP_ADDR` | `127.0.0.1:7778` | Adresse d'écoute UDP (télémétrie des unités) |
| `DCSMANAGER_TCP_ADDR` | `127.0.0.1:7779` | Adresse d'écoute TCP (events + commandes) |
| `DCSMANAGER_DB_PATH` | `./data/dcsmanager.db` | Chemin de la base SQLite |
| `DCSMANAGER_DB_ENABLED` | `true` | Activer la persistance (sinon tout en mémoire) |
| `DCSMANAGER_THEATRE` | `Caucasus` | Théâtre par défaut |
| `DCSMANAGER_UNIT_TTL` | `5` (secondes) | Délai avant qu'une unité silencieuse disparaisse |
| `DCSMANAGER_CATEGORIES` | `./categories.json` | Surcharge de classification des engins |
| `DCSMANAGER_MAX_UNITS` | `5000` | Nombre maximum d'unités suivies |
| `DCSMANAGER_TRACK_INTERVAL` | `3` (secondes) | Fréquence d'échantillonnage des positions |
| `DCSMANAGER_TRACK_GRACE` | `15` (secondes) | Absence avant de compter une unité comme perdue |
| `DCSMANAGER_TRACK_RETENTION` | `86400` (secondes) | Durée de conservation de l'historique |
| `DCSMANAGER_SAVED_GAMES` | *(auto)* | Dossier Saved Games de DCS, si la détection échoue |
| `DCSMANAGER_CHARTS_DIR` | `./maps_dcs` | Scans de cartes aéronautiques (approches, plans de mouvement) |
| `DCSMANAGER_SOURCE` | *(auto)* | Force la source de la session : `live` ou `test` (voir plus bas) |
| `DCSMANAGER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

### Côté DCS — `Saved Games\DCS\Config\dcsmanager.cfg`

```lua
-- Adresse du backend (le manager tourne en local)
dcsmanager_host = "127.0.0.1"
dcsmanager_udp_port = 7778
dcsmanager_tcp_port = 7779

-- Télémétrie
dcsmanager_send_interval = 1.0    -- position du joueur (secondes)
dcsmanager_world_enabled = true   -- export de tous les objets
dcsmanager_world_interval = 2.0   -- liste des objets (secondes)
dcsmanager_world_radius = 0       -- filtre par rayon en km (0 = tout)
```

---

## Installation des scripts Lua dans DCS

> ⚠️ **Toujours merger, jamais écraser.** `Export.lua` est très souvent déjà modifié par
> Tacview, SRS, DCS-BIOS, etc. Sauvegarde le fichier existant avant toute modification.

Les scripts se placent dans le dossier *Saved Games* de DCS :

```
%USERPROFILE%\Saved Games\DCS\          (ou DCS.openbeta)
├─ Config\
│   └─ dcsmanager.cfg          ← configuration de l'adresse backend
└─ Scripts\
    ├─ Export.lua         ← positions (live map) ; à MERGER avec l'existant
    └─ Hooks\
        └─ dcsmanager.lua      ← événements, joueurs, chat (Phases 2+)
```

1. Copie `dcs-lua/Config/dcsmanager.cfg` dans `Saved Games\DCS\Config\`.
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
.\dcsmanager.exe
```

Le manager s'ouvre dans sa fenêtre. Pour retrouver l'interface dans un navigateur
(second écran, tablette), lance `.\dcsmanager.exe serve` et ouvre
<http://localhost:8080>.

### CLI

```powershell
dcsmanager                 # ouvre le manager dans une fenêtre native
dcsmanager serve           # lance le serveur seul ; UI sur http://localhost:8080
dcsmanager install-lua     # installe/fusionne les scripts Lua dans Saved Games
dcsmanager uninstall-lua   # retire le bloc installé (garde la config)
dcsmanager status          # installed / outdated / missing, par fichier
dcsmanager purge           # supprime des sessions enregistrées (destructif)
dcsmanager version
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
de `tools/send-telemetry.mjs`. Utilisez `DCSMANAGER_SOURCE=test` (ou `live`) pour
forcer le verdict avec vos propres fixtures.

```powershell
dcsmanager purge --source test            # supprime uniquement les sessions simulées
dcsmanager purge --mission-id 3           # supprime une mission et tout ce qui y est lié
dcsmanager purge --all                    # supprime toutes les sessions enregistrées
dcsmanager purge --source test --dry-run  # affiche ce qui serait supprimé, sans rien faire
```

Les statistiques acceptent `?includeTest=1` pour inclure volontairement les
sessions simulées.

> En migrant depuis une version antérieure, les missions **existantes** sont
> étiquetées `live` (la migration ne peut pas savoir qu'elles étaient simulées).
> Pour effacer des données enregistrées avant cette fonction, utilisez
> `dcsmanager purge --mission-id <n>` ou `--all`.

---

## Structure du projet

```
DCS Manager/
├─ README.md
├─ CHANGELOG.md
├─ VERSION
├─ Makefile / build.ps1       # commandes de build
├─ install-dcs.ps1           # installe les scripts Lua dans Saved Games
├─ dcs-lua/                  # scripts à installer côté DCS
│   ├─ Config/dcsmanager.cfg      # modèle de configuration
│   ├─ Export.lua            # positions → UDP (télémétrie)
│   └─ Hooks/dcsmanager.lua       # events / joueurs / chat
├─ backend/                  # Go
│   ├─ go.mod
│   ├─ cmd/dcsmanager/main.go     # CLI (desktop / serve / install / purge)
│   ├─ internal/
│   │   ├─ desktop/          # fenêtre native (WebView2, pure Go)
│   │   ├─ app/              # câblage du manager, partagé desktop / serve
│   │   ├─ config/           # chargement env + défauts
│   │   ├─ install/          # injecteur Lua (fusion par marqueurs)
│   │   ├─ aerodrome/        # aérodromes et fréquences (données embarquées)
│   │   ├─ category/         # classification des engins (type DCS → famille)
│   │   ├─ theatre/          # théâtres DCS
│   │   ├─ charts/           # scans de cartes aéronautiques (maps_dcs/)
│   │   ├─ model/            # types échangés DCS ↔ backend
│   │   ├─ lua/              # parseur de données Lua (debrief.log)
│   │   ├─ debrief/          # analyse des débriefs
│   │   ├─ debriefstore/     # réassemblage des transferts de débrief
│   │   ├─ udp/              # récepteur télémétrie des unités
│   │   ├─ tcp/              # récepteur événements / joueurs / chat
│   │   ├─ live/             # état de session en mémoire
│   │   ├─ ingest/           # pont live → base de données
│   │   ├─ tracker/          # historique positions + détection de pertes
│   │   ├─ db/               # persistance SQLite (pur Go)
│   │   ├─ state/            # store unités (en mémoire)
│   │   ├─ stats/            # agrégations statistiques
│   │   └─ api/              # REST + SSE + UI embarquée (dist/)
├─ frontend/                 # Svelte + Vite
│   └─ src/
│       ├─ App.svelte
│       └─ lib/              # panneaux et stores
├─ maps_dcs/                 # scans de cartes aéronautiques (local, non versionné)
├─ tools/                    # émetteur de télémétrie de test
└─ docs/                     # documentation
```

---

## Roadmap

- [x] **Phase 0 — PoC** : `Export.lua` (position joueur) → Go → UI
- [x] **Phase 2 — Événements & joueurs** : `onGameEvent`, chat, `net.get_stat`, historique SQLite
- [x] **Phase 3 — Débriefings** : envoi réseau de `debrief.log`, parseur Lua, historique
- [x] **Phase 4 — Stats avancées** : vue d'ensemble, pilotes, armes, engins, balance, réseau
- [x] **Phase 4 bis — Cartes analytiques & sortie** : heatmaps, traces, télémétrie
- [x] **Phase 5 — Packaging** : CLI, injecteur Lua sûr, binaire unique autonome
- [x] **Phase 6 — Aérodromes** : fréquences et cartes lues depuis les fichiers DCS
- [ ] **Phase 1 — Live map** : retirée ; la télémétrie est toujours collectée pour l'analyse

Le plan complet et détaillé est disponible dans le fichier de plan du projet.

---

## Licence

[MIT](LICENSE) © 2026 Laurent (lolo1580)

Projet communautaire, non affilié à Eagle Dynamics et non approuvé par cet
éditeur. « DCS World » et ses terrains sont des marques d'Eagle Dynamics SA.
