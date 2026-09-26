# Changelog

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) et le projet adhère
au [versionnage sémantique](https://semver.org/lang/fr/).

## [Non publié]

### À venir

- Phase 3 — Débriefings (transport réseau + parseur)
- Phase 4 — Statistiques avancées (7 modules)
- Phase 5 — Packaging final et injecteur Lua

## [0.3.0] — 2026-09-26

**Phase 2 — Événements & joueurs.** Le backend reçoit et historise les événements
de jeu, les joueurs connectés et le chat.

### Ajouté

- **Backend**
  - `internal/model` : types échangés (messages, joueurs, événements, chat, missions).
  - `internal/tcp` : récepteur TCP au format **JSON ligne par ligne** (NDJSON),
    avec reconnexion par connexion et journalisation.
  - `internal/live` : état de session en mémoire (événements, joueurs triés par camp,
    chat, mission courante) avec plafonds.
  - `internal/db` : **persistance SQLite pure Go** (`modernc.org/sqlite`, sans CGO),
    tables `missions`, `events`, `chat`, `players`, `player_stats`, `meta`.
  - `internal/ingest` : pont entre le direct et la base ; ouvre la mission courante,
    résout les joueurs par **UCID** (carrière qui survit aux renommages) et
    n'enregistre les stats que des joueurs actifs.
  - `internal/api` : `GET /api/game-events`, `/api/players`, `/api/chat`,
    `/api/mission`, `/api/history/{events,chat,missions}` ; trame SSE `session`.
  - `internal/config` : `DCSMM_DB_ENABLED`.

- **Scripts DCS**
  - `Hooks/dcsmm.lua` : envoi TCP des événements (`onGameEvent`), du chat
    (`onChatMessage`), des joueurs et de leurs statistiques (`net.get_player_list`,
    `net.get_player_info`, `net.get_stat`), et des transitions de mission. Connexion
    persistante, reconnexion automatique, appels protégés par `pcall`, jamais bloquant.
  - `Config/dcsmm.cfg` : `dcsmm_players_interval`.

- **Frontend**
  - Onglets **Carte** / **Session** ; nom de la mission en cours dans l'en-tête.
  - Panneau **Joueurs** : nom, score, kills air/sol/navire, atterrissages, ping
    (surligné au-delà de 250 ms), code couleur par camp.
  - Panneau **Événements** : filtres par type avec compteurs, horodatage et
    description lisible des arguments DCS.
  - Panneau **Chat** : historique et zone de saisie (envoi vers DCS à venir).

- **Outils & documentation**
  - `tools/send-events.mjs` : simulateur du canal TCP (mission, joueurs, événements, chat).
  - `docs/phase2-events.md`.
  - Tests : `internal/live` (plafonds, tri, mission) et `internal/db` (missions,
    événements, chat, UCID, stats).

## [0.2.0] — 2026-09-25

**Phase 1 — Live map complète.** Tous les objets de la mission sont suivis, classés
et affichés sur une carte interactive.

### Ajouté

- **Backend**
  - `internal/category` : classification des **types DCS exacts** en familles
    (avion, hélicoptère, sol, navire, structure), avec surcharge par fichier JSON.
  - `internal/theatre` : théâtres DCS (Caucase, Syrie, NTTR, Golfe, Mariannes, etc.)
    avec leur emprise géographique.
  - `internal/udp` : prise en charge des messages **`world`** (listes d'objets) en plus
    d'`ownship` ; datagrammes jusqu'à 1 Mo.
  - `internal/state` : identifiants stables, plafond d'unités (`MaxUnits`), âge par unité.
  - `internal/api` : filtres (`?category=`, `?coalition=`, `?ownship=`, `?q=`), résumé
    par catégorie/coalition, `GET /api/units/{id}`, `GET /api/theatres`, et service des
    **tuiles de carte** DCS (`/api/tiles/<theatre>/<z>/<x>/<y>.png`) avec protection
    contre la traversée de chemin.
  - `internal/config` : nouvelles options (`DCSMM_UNIT_TTL`, `DCSMM_TILES_DIR`,
    `DCSMM_BASEMAP_URL`, `DCSMM_CATEGORIES`, `DCSMM_MAX_UNITS`).

- **Scripts DCS**
  - `Export.lua` : export de **tous les objets du monde** en plus du joueur, filtrable
    par rayon (`dcsmm_world_radius`), plafonné (`dcsmm_max_objects`), coalitions
    sélectionnables ; accès défensif aux fonctions `LoGet*`/`Export.*`.
  - `Config/dcsmm.cfg` : nouvelles options documentées.

- **Frontend**
  - Carte : icônes SVG par catégorie, couleurs par coalition, **traces de vol** pour
    avions et hélicoptères, survol avec infobulle, recentrage sur le joueur.
  - **Sélecteur de fond de carte** : Satellite, Relief, Routier, Sombre — plus l'option
    « DCS » automatique si des tuiles authentiques sont présentes. Aucun fond ne
    nécessite de clé API (le mode sombre est un filtre CSS sur les tuiles OSM).
    Le choix est mémorisé dans le navigateur.
  - Panneau latéral : filtres coalitions/catégories avec compteurs en direct, recherche,
    « mon appareil uniquement », liste des unités sélectionnables.
  - Fiche unité : type, catégorie, coalition, pays, position, altitude, cap, âge.
  - Support des **tuiles DCS** avec repli automatique sur un fond de carte réel.

- **Outils & documentation**
  - `tools/send-telemetry.mjs` : émet aussi des objets de monde (sol, navires, IA).
  - `tools/export-tiles.py` : extraction de tuiles depuis une image de carte géoréférencée.
  - `tools/inspect-maps.py` : recense les scans de cartes et exporte les coins.
  - `categories.example.json`, `docs/live-map.md`, `maps_dcs/README.md`.
  - `maps_dcs/` (≈1,2 Go) exclu de git ; seul son README est suivi.
  - Tests unitaires supplémentaires (catégories, théâtres, plafond du store, messages monde).

## [0.1.0] — 2026-09-25

Première version : **PoC Phase 0** — validation de la chaîne DCS → Go → navigateur.

### Ajouté

- **Documentation**
  - `README.md` : vision, architecture, prérequis, démarrage rapide, configuration,
    installation Lua, déploiement `.exe`/Docker, structure, roadmap.
  - `CHANGELOG.md` : suivi des versions.
  - `docs/architecture.md` : flux de données, composants, choix techniques.
  - `docs/dcs-installation.md` : guide d'installation côté DCS et dépannage.

- **Outils**
  - `tools/send-telemetry.mjs` : émetteur de télémétrie factice pour tester sans DCS.
  - `Makefile` et `build.ps1` : commandes de build (frontend, backend, Docker).

- **Scripts DCS (`dcs-lua/`)**
  - `Config/dcsmm.cfg` : modèle de configuration (IP backend, ports, intervalle d'envoi).
  - `Export.lua` : export de la position du joueur vers le backend en UDP/JSON,
    échantillonné une fois par seconde via `LuaExportActivityNextEvent` (aucun impact
    sur les performances du simulateur).

- **Backend Go (`backend/`)**
  - `internal/config` : configuration par variables d'environnement (`DCSMM_*`) avec défauts.
  - `internal/udp` : récepteur UDP décodant le JSON des positions.
  - `internal/state` : store en mémoire des unités (avec péremption).
  - `internal/api` : serveur HTTP (REST `GET /api/state`, flux **Server-Sent Events**
    `/api/events`) + service de l'interface web embarquée.
  - `cmd/dcsmm/main.go` : point d'entrée assemblant les briques.

- **Frontend (`frontend/`)**
  - Interface Leaflet minimale affichant la position des unités en temps réel via
    **Server-Sent Events**, avec une page de repli générée si le frontend n'est pas buildé.

- **Déploiement (`deploy/`)**
  - `Dockerfile` multi-stage (build frontend + backend → image Alpine minimale, non-root).
  - `docker-compose.yml` avec ports UDP/TCP publiés et volume SQLite.

### Notes

- DCS World reste sur Windows et ne tourne jamais dans le conteneur Docker ; le manager
  communique avec lui par le réseau.
- Le temps réel de la Phase 0 utilise **SSE** (flux descendant). Un canal TCP
  bidirectionnel (commandes vers DCS) est prévu en Phase 2.
