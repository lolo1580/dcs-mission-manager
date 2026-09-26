# Événements, joueurs et chat (Phase 2)

## Principe

En plus du canal **UDP** (positions, Phase 1), le backend écoute un canal **TCP**
sur lequel les hooks Lua poussent des messages **JSON, une ligne par message**.

```
DCS (Windows)                                    Backend
Scripts/Hooks/dcsmm.lua
  onGameEvent ─┐
  onChatMessage ┼─► TCP 7779, JSON ligne par ligne ─► internal/tcp
  net.get_*   ─┘                                        │
                                                        ├─► internal/live  (mémoire, UI temps réel)
                                                        └─► internal/ingest ─► internal/db (SQLite)
```

Le choix du **JSON ligne par ligne** (NDJSON) plutôt qu'une trame binaire est
délibéré : c'est débogable à l'œil, le parseur tient en quelques lignes, et le
protocole peut évoluer sans casser les anciens clients.

## Messages

| `type` | Contenu | Émis par |
|---|---|---|
| `mission` | `phase` (`start`/`end`), `name`, `theatre`, `winner` | `onSimulationStart` / `onSimulationStop` |
| `event` | `event`, `args[]`, `t` | `onGameEvent` |
| `players` | `players[]` (id, UCID, nom, camp, slot, stats) | `net.get_player_list` + `net.get_stat` |
| `chat` | `from`, `message` | `onChatMessage` |

### Événements capturés

`kill`, `friendly_fire`, `mission_end`, `self_kill`, `change_slot`, `connect`,
`disconnect`, `crash`, `eject`, `takeoff`, `landing`, `pilot_death`.

### Statistiques par joueur

Exactement celles de l'API `net.get_stat` : ping, crashes, kills véhicules /
avions / navires, score, atterrissages, éjections. Plus l'**UCID**, indispensable
pour des statistiques de carrière qui survivent aux changements de pseudo.

## Rafraîchissement

- **Événements** et **chat** : envoyés dès qu'ils surviennent.
- **Joueurs** : rafraîchis toutes les `dcsmm_players_interval` secondes (défaut 5),
  et immédiatement lors d'un `change_slot`, `connect`, `disconnect` ou `mission_end`.
- Le rafraîchissement périodique passe par `onSimulationFrame`, donc il ne bloque
  jamais une frame : aucun appel réseau bloquant n'est fait dans un callback.

## Robustesse côté Lua

- Connexion TCP **persistante** avec reconnexion automatique si le backend
  redémarre ou est absent.
- `settimeout(0.5)` : un backend injoignable ne fige pas le simulateur.
- Chaque appel `net.*`/`Sim.*` est protégé par `pcall`.
- `tcp-nodelay` activé pour un envoi immédiat des événements.

## API Web

| Route | Description |
|---|---|
| `GET /api/game-events` | Événements récents (mémoire) |
| `GET /api/players` | Joueurs connectés |
| `GET /api/chat` | Chat récent ; `POST` réservé à l'envoi vers DCS (à venir) |
| `GET /api/mission` | Mission en cours |
| `GET /api/history/events` | Événements persistés (SQLite) |
| `GET /api/history/chat` | Chat persisté |
| `GET /api/history/missions` | Missions passées |

Les mêmes données arrivent en temps réel par **SSE** (`/api/events`) sous la forme
d'une trame `{"type":"session", ...}` émise chaque seconde.

## Persistance

SQLite via **modernc.org/sqlite** (pur Go, sans CGO) : le binaire reste unique et
cross-compilable pour le `.exe` Windows comme pour l'image Docker.

Tables : `missions`, `events`, `chat`, `players`, `player_stats`, `meta`.
La persistance peut être coupée avec `DCSMM_DB_ENABLED=false` (tout reste en
mémoire).

## Canal de commandes (à venir)

Le canal TCP est **bidirectionnel par construction** : le backend pourra pousser
des commandes (kick, changement de mission, message de chat) que le hook Lua
exécutera. L'endpoint `POST /api/chat` existe déjà et répond `501` explicitement
tant que ce canal n'est pas câblé.
