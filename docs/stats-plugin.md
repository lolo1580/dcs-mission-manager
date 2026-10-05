# Plugin de statistiques séparé : Docker + PostgreSQL

> Étude d'architecture — aucun code de production n'est modifié par ce document.
> Complément de [`database-backends.md`](database-backends.md) (§12).
> Objectif : obtenir une **web app de statistiques dédiée**, avec **PostgreSQL**,
> **sans toucher au cœur** du manager (qui reste SQLite, local, un seul `.exe`).
>
> **PoC P0 livré** : le squelette exécutable se trouve dans [`stats-plugin/`](../stats-plugin/)
> (Go + pgx, Postgres en Docker, dashboard de tendances). Voir son
> [`README.md`](../stats-plugin/README.md).

---

## 1. Objet, non-objectifs, principe

### Objectif

Ajouter des capacités statistiques « lourdes » sans refondre la persistance du
manager :

- **historique temporel** (courbes, tendances) — le manager ne stocke que l'état
  agrégé courant, pas de séries ;
- **agrégation multi-machines** (plusieurs postes DCS → une seule base) ;
- **requêtes analytiques SQL** (fenêtres, classements, comparatifs de carrière) ;
- **web app séparée**, éventuellement accessible depuis un autre appareil.

### Non-objectifs

- Remplacer SQLite dans le manager (c'est l'autre étude) ;
- conteneuriser `dcsmanager.exe` (il doit lire DCS et Saved Games en local) ;
- devenir une dépendance obligatoire : le manager doit fonctionner **sans** le plugin.

### Principe

Le plugin est un **consommateur externe de l'API REST** du manager. Il tourne à côté
(Docker), interroge `http://<manager>:8080/api/...`, écrit dans PostgreSQL et sert
son propre front. Le manager ne sait même pas qu'il existe.

```
┌─────────────────────────── Machine Windows (DCS) ───────────────────────────┐
│                                                                             │
│  dcsmanager.exe  (SQLite, inchangé)                                         │
│      └─ API REST + SSE  ── 127.0.0.1:8080  (ou 0.0.0.0:8080 pour le plugin)  │
│                                                                             │
└──────────────────────────────────┬──────────────────────────────────────────┘
                                   │ HTTP (pull / éventuellement SSE)
                                   ▼
┌──────────────────── Docker (même machine ou autre) ─────────────────────────┐
│                                                                             │
│  stats-web (conteneur)  ──►  poll API ──►  PostgreSQL (conteneur)            │
│      └─ web app (dashboard, tendances)                                      │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Surface d'API disponible (contrat d'entrée)

Tout est déjà exposé par `internal/api/api.go:250-269`. Le plugin n'a besoin d'aucune
nouvelle route pour démarrer :

| Route | Contenu | Usage plugin |
|---|---|---|
| `GET /api/health` | `{status, service, units}` | santé / détection de version |
| `GET /api/stats/overview?scope=...` | missions, pilotes, kills, morts, crashes, éjections, FF, coalitions | tableau de bord |
| `GET /api/stats/pilots` | score, kills, morts, K/D, atterrissages, éjections, ping | classements |
| `GET /api/stats/weapons` | kills/FF par arme, victimes, plateformes | analyse d'armement |
| `GET /api/stats/engines` | par type DCS exact : kills, pertes, sorties, K/D | analyse par machine |
| `GET /api/stats/network` | ping moyen/max par pilote | qualité réseau |
| `GET /api/history/missions?limit=N` | missions passées | liste |
| `GET /api/history/events?limit=N` | N derniers événements bruts | miroir (voir §4) |
| `GET /api/history/chat?limit=N` | N derniers messages de chat | miroir |
| `GET /api/debriefs` / `/api/debriefs/{id}?raw=1` | débriefs (parsed + brut) | archive |
| `GET /api/career` | carnet de vol (logbook) | profil carrière |
| `GET /api/modules`, `/api/mods`, `/api/scripts` | inventaire DCS | contexte |
| `GET /api/events` (SSE) | flux temps réel (état, session) | rafraîchissement live |

Scopes : `?scope=career` (défaut) ou `?scope=mission&missionId=N` ; `?includeTest=1`
pour inclure les sessions simulées (exclues par défaut, cf. `internal/api/stats.go:15-38`).

Formes de réponse (extraits de `internal/stats/stats.go:66-135`) :

```jsonc
// /api/stats/pilots -> { ... } (tableau d'objets)
[{ "ucid":"…","name":"…","missions":3,"score":1200,"killsAir":5,"killsCar":2,
   "killsShip":0,"kills":7,"deaths":2,"crashes":1,"ejections":0,"landings":4,
   "friendlyFire":0,"avgPing":42.5,"kd":3.5 }]

// /api/stats/engines
[{ "typeId":"F-16C_50","category":"plane","kills":9,"deaths":3,"sorties":12,"kd":3.0 }]

// /api/stats/weapons
[{ "weapon":"AIM-120C","kills":4,"friendlyFire":0,
   "victimsByType":{"Su-27":2},"killersByType":{"F-16C_50":3} }]
```

---

## 3. Synchro incrémentale : état avant/après P3

> **Mis à jour après P3** : l'endpoint additif a été **implémenté** (voir §14).
> Cette section décrit le problème initial et pourquoi cet ajout était nécessaire.

Les routes d'historique étaient **des « N derniers »**, sans curseur :

- `handleHistoryEvents` → `s.db.RecentEvents(limit)` (`internal/api/session.go:99-110`) ;
- `RecentEvents` fait `ORDER BY id DESC LIMIT ?` (`internal/db/persist.go:115-121`) ;
- de même `RecentChat` (`persist.go:144-149`) et `Missions` (`maintenance.go:293-326`).

Conséquence : **on ne pouvait pas demander « ce qui est arrivé depuis l'id N »**. Si le
plugin redémarrait ou ratait une fenêtre, il ne pouvait « repasser » que sur les N
derniers, avec un trou possible et des doublons.

Deux stratégies en découlaient, **toutes deux livrées** :

**Stratégie S1 — Agrégats uniquement.** Le plugin interroge périodiquement
`/api/stats/*` et stocke des **instantanés** horodatés. Il apporte les **tendances**
(que le manager n'a pas). C'est le mode par défaut.

**Stratégie S2 — Miroir complet.** Le manager expose désormais une lecture additive :

```
GET /api/history/events?sinceId=N&limit=1000
  -> { "count": n, "events": [...], "nextSinceId": <max id renvoyé> }
GET /api/history/chat?sinceId=N&limit=1000
  -> { "count": n, "chat": [...],   "nextSinceId": <max id renvoyé> }
```

`sinceId` remplace le `ORDER BY id DESC LIMIT` par `WHERE id > ? ORDER BY id ASC LIMIT ?`.
Lecture pure, non destructive, rétrocompatible : sans `sinceId`, le comportement
d'origine (les N derniers) est conservé.

---

## 4. Schéma PostgreSQL (miroir + séries)

Le plugin possède **son** schéma, indépendant de SQLite. Deux familles de tables :
le **miroir** (données brutes, si S2) et les **séries** (instantanés, S1).

```sql
-- ---- métadonnées de synchro -------------------------------------------------
CREATE TABLE IF NOT EXISTS sync_cursor (
    source          text PRIMARY KEY,        -- ex. URL du manager
    manager_version text,
    last_event_id   bigint NOT NULL DEFAULT 0,
    last_chat_id    bigint NOT NULL DEFAULT 0,
    last_mission_id bigint NOT NULL DEFAULT 0,
    last_run_at     timestamptz,
    last_ok         boolean
);

-- ---- miroir (S2) ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS missions (
    id         bigint PRIMARY KEY,           -- id côté manager
    name       text NOT NULL,
    theatre    text,
    source     text NOT NULL DEFAULT 'live',
    started_at bigint NOT NULL,
    ended_at   bigint,
    winner     text,
    synced_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS players (
    id         bigint PRIMARY KEY,
    ucid       text,
    name       text NOT NULL,
    first_seen bigint,
    last_seen  bigint
);

CREATE TABLE IF NOT EXISTS events (
    id         bigint PRIMARY KEY,
    mission_id bigint REFERENCES missions(id),
    event      text NOT NULL,
    args       jsonb,                        -- texte JSON côté manager -> jsonb
    detail     jsonb,
    t          double precision,
    real_ts    bigint NOT NULL,
    synced_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_events_mission ON events(mission_id);
CREATE INDEX IF NOT EXISTS idx_events_event   ON events(event);

CREATE TABLE IF NOT EXISTS chat (
    id         bigint PRIMARY KEY,
    mission_id bigint REFERENCES missions(id),
    "from"     text NOT NULL,
    message    text NOT NULL,
    real_ts    bigint NOT NULL
);

CREATE TABLE IF NOT EXISTS player_stats (
    id            bigint PRIMARY KEY,
    mission_id    bigint REFERENCES missions(id),
    player_id     bigint REFERENCES players(id),
    dcs_player_id bigint,
    side          integer,
    slot          text,
    unit_type     text,
    ping          integer,
    crashes       integer,
    kills_car     integer,
    kills_air     integer,
    kills_ship    integer,
    score         integer,
    landings      integer,
    ejects        integer,
    real_ts       bigint NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pstats_mission ON player_stats(mission_id);
CREATE INDEX IF NOT EXISTS idx_pstats_player  ON player_stats(player_id);
CREATE INDEX IF NOT EXISTS idx_pstats_dcs     ON player_stats(mission_id, dcs_player_id);

-- ---- séries temporelles (S1) ------------------------------------------------
-- Un instantané JSON par scope et par collecte : c'est ce qui donne les courbes.
CREATE TABLE IF NOT EXISTS stat_snapshots (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    scope       text NOT NULL,               -- 'career' | 'mission'
    mission_id  bigint,
    captured_at timestamptz NOT NULL DEFAULT now(),
    include_test boolean NOT NULL DEFAULT false,
    kind        text NOT NULL,               -- 'overview' | 'pilots' | 'weapons' | 'engines' | 'network'
    payload     jsonb NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snap_kind_time ON stat_snapshots(kind, captured_at);
```

Notes de mapping :

- `args`/`detail` sont stockés en **TEXT JSON** côté manager → `jsonb` ici, ce qui
  permet des requêtes analytiques (`args->>6` pour l'arme, etc.). Attention : les
  `args` DCS sont des **tableaux**, indexés (`stats.go:298-319`) ;
- `real_ts` en **bigint** (millisecondes, cf. l'autre étude) ;
- les identifiants du manager sont **réutilisés tels quels** (`id`), ce qui rend le
  miroir idempotent : `INSERT ... ON CONFLICT (id) DO UPDATE` ;
- les tables `track_positions`/`losses` (télémétrie) sont **volumineuses** : à ne
  synchroniser qu'en option, ou pas du tout dans un premier temps.

---

## 5. Synchronisation

### 5.1 Boucle (S1 — instantanés)

```
toutes les N minutes :
  pour scope in (career, mission) :
    pour kind in (overview, pilots, weapons, engines, network) :
      GET /api/stats/<kind>?scope=<scope>[&missionId=…]
      INSERT stat_snapshots(scope, mission_id, kind, payload, captured_at)
```

Fréquence conseillée : 5–15 min en veille, plus rapprochée pendant un vol (détection
via `GET /api/state` ou le flux SSE `/api/events`). Idempotence : on peut dédupliquer
sur `(kind, scope, hash(payload))` pour ne pas empiler des instantanés identiques.

### 5.2 Boucle (S2 — miroir incrémental)

```
GET /api/history/events?sinceId=<last_event_id>&limit=1000
UPSERT events  ON CONFLICT (id) DO UPDATE
UPDATE sync_cursor SET last_event_id = max(id)
```

Idem `chat` puis `missions`, puis dérivation de `player_stats` (à ajouter comme route
`s?sinceId=` côté manager, ou reconstruite depuis les événements).

### 5.3 Détection de trou

Si au démarrage le plugin constate que `min(id)` renvoyé par le manager est très
supérieur à `last_event_id + 1`, il doit **signaler** la perte (« historique
tronqué ») plutôt que de faire semblant. C'est une conséquence directe de §3.

---

## 6. Web app du plugin

Un front dédié, qui n'entre pas en concurrence avec l'UI du manager :

| Vue | Contenu | Source |
|---|---|---|
| Tendances | courbes de kills/K/D/score dans le temps | `stat_snapshots` |
| Classement carrière | tri, filtres, comparaison de pilotes | instantanés + miroir |
| Armement | efficacité par arme, matrice | miroir `events` (S2) |
| Machines | par type DCS, K/D comparés | miroir / `engines` |
| Multi-serveurs | agrégation de plusieurs managers | `sync_cursor` multi-lignes |
| Export | CSV/JSON, API publique du plugin | tout |

Stack suggérée : **Go** (cohérent avec le projet, binaire unique, `pgx` v5 pur Go) +
template HTML ou réutilisation de Svelte. Alternative : Node + Svelte (proche du front
existant). Le choix n'a pas d'impact sur le manager.

Le plugin expose **sa propre** API (`/api/plugin/...`) et **sa propre** page ; il ne
réutilise pas l'UI embarquée.

---

## 7. Docker

Deux topologies, à choisir selon le besoin.

### Topologie A — Postgres seul en Docker, plugin en local (recommandée)

```
Manager (Windows, .exe, SQLite) ──► stats-web (local, .exe ou Node) ──► Postgres (Docker)
```

- Pas de problème de loopback : le plugin tourne sur la machine et joint
  `127.0.0.1:8080` normalement.
- Docker ne sert qu'à héberger Postgres.

### Topologie B — tout en Docker

```
Manager (Windows) ◄── stats-web (conteneur) ──► Postgres (conteneur)
```

**Piège à connaître** : un conteneur ne peut **pas** joindre un service du host lié à
`127.0.0.1`. `host.docker.internal` atteint le host, mais pas son loopback. Il faut
donc lancer le manager avec `DCSMANAGER_HTTP_ADDR=0.0.0.0:8080` — et alors **l'API
est exposée sur le réseau, sans authentification** (voir §8).

`docker-compose.yml` :

```yaml
services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_USER: dcs
      POSTGRES_PASSWORD: change-me
      POSTGRES_DB: stats
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dcs -d stats"]
      interval: 5s
      timeout: 3s
      retries: 10

  stats-web:
    build:
      context: ./stats-plugin
      dockerfile: Dockerfile
    environment:
      DCSMANAGER_URL: "http://host.docker.internal:8080"
      DATABASE_URL: "postgres://dcs:change-me@postgres:5432/stats?sslmode=disable"
      SYNC_INTERVAL: "10m"
      MANAGER_NAME: "server-1"          # identifies this manager in the shared DB
      PLUGIN_AUTH_TOKEN: ""             # set to protect an exposed plugin
    ports:
      - "8090:8090"
    depends_on:
      postgres:
        condition: service_healthy
    extra_hosts:
      - "host.docker.internal:host-gateway"   # utile sous Linux

volumes:
  pgdata:
```

---

## 8. Sécurité

Point sensible, déjà noté dans le README :

- **l'API du manager n'a pas d'authentification**, et elle contient un endpoint
  **destructif** (`DELETE /api/maintenance/purge`, `internal/api/maintenance.go:24`).
  L'exposer (`0.0.0.0`) pour un conteneur est un vrai risque.
- Recommandations :
  1. privilégier la **topologie A** (plugin local, Postgres en Docker) pour garder
     l'API sur `127.0.0.1` ;
  2. en topologie B, **filtrer** au niveau du pare-feu Windows (n'autoriser que
     l'hôte Docker) et/ou placer un reverse proxy devant ;
  3. ne jamais publier le port Postgres au-delà de la machine (`ports` vs réseau
     Docker interne) ;
  4. mots de passe Postgres non par défaut ; `sslmode` à adapter.
- Le plugin lui-même devrait porter une authentification s'il est joint à distance.

---

## 9. Risques et compromis

| Risque | Détail | Mitigation |
|---|---|---|
| Sync non incrémentale | `?limit=N` seulement, trous/ doublons possibles | S1 (instantanés) d'abord ; endpoint additif `?sinceId=` pour S2 |
| API sans auth exposée | loopback non joignable depuis un conteneur | topologie A, ou firewall + proxy |
| Double maintenance | schéma et logique de stats dupliqués | le plugin **ne recalcule pas** : il lit les agrégats du manager |
| Cohérence | le manager reste la source de vérité | le plugin est **lecture seule** sur le manager |
| Volume télémétrie | `track_positions` très gros | ne pas synchroniser par défaut |
| Dérive de version d'API | pas de versionnage des routes | figer le contrat, `GET /api/health` pour détecter |

---

## 10. Plan par phases

| Phase | Contenu | État |
|---|---|---|
| P0 | PoC : lire `/api/stats/*`, écrire `stat_snapshots` dans Postgres, une page de courbe | ✅ livré |
| P1 | `docker-compose` Postgres + plugin, config `DCSMANAGER_URL`/`DATABASE_URL` | ✅ livré |
| P2 | Web app : onglets (overview, pilotes, armes, machines, réseau, tendances), tri, filtres | ✅ livré |
| P3 | Endpoint additif `?sinceId=` côté manager + miroir incrémental `events`/`chat` (jsonb) | ✅ livré |
| P4 | Route `missions?sinceId=`, multi-managers, auth du plugin, exports CSV/JSON | ✅ livré |

---

## 11. Livré (P0–P4)

### Côté plugin — `stats-plugin/`

- **Snapshots** des agrégats (`stat_snapshots`, `jsonb`) avec déduplication par
  hash : pas d'empilement de lignes identiques ;
- **Miroir incrémental** des événements, du chat **et des missions** (`events`,
  `chat`, `missions`, `jsonb`) via `?sinceId=`, avec des **curseurs monotones**
  (`sync_cursor`, `GREATEST`) qui ne peuvent jamais reculer. Les missions
  **récentes** sont en plus re-synchronisées à chaque passe, car une mission qui
  se termine garde son id mais gagne `ended_at`/`winner` ;
- **Multi-managers** : chaque ligne porte une colonne `instance` (issue de
  `MANAGER_NAME`). Plusieurs instances du plugin peuvent partager une seule base
  sans jamais mélanger leurs données ; le dashboard a un sélecteur d'instance ;
- **Authentification** optionnelle (`PLUGIN_AUTH_TOKEN`) : jeton Bearer ou
  cookie, comparaison à temps constant ; désactivée par défaut (loopback) ;
- **API** : `/api/plugin/health` (compteurs mirés + instances), `/summary`,
  `/series`, `/latest`, `/missions`, `/instances`, `/export?type=events|missions|series&format=csv|json`,
  plus les routes de commodité `/pilots`, `/weapons`, `/engines`, `/network` ;
- **Dashboard** : onglets Overview / Pilots / Weapons / Aircraft & vehicles /
  Network / **Missions** / Trends / **Export**, tri par colonne, filtres texte,
  sélecteurs d'instance et de scope, courbe de tendance.

### Côté manager — cœur Go (ajout additif)

- `GET /api/history/events?sinceId=N`, `.../chat?sinceId=N` et
  `.../missions?sinceId=N` (`internal/api/session.go`), adossés à
  `EventsSince`/`ChatSince`/`MissionsSince` (`internal/db/history.go`) ;
- rétrocompatibles (sans `sinceId`, comportement d'origine) ;
- testés (`internal/api/history_test.go`, `internal/db/history_test.go`).

### Tests du plugin

- tests unitaires sans base (config, client, auth, assets) : `go test ./...` ;
- **tests d'intégration PostgreSQL** (SQL réel : `jsonb`, `ON CONFLICT`,
  `DISTINCT ON`, upserts en batch, curseurs) exécutés **seulement** si
  `PLUGIN_TEST_DATABASE_URL` est défini ; sinon ils se sautent, donc
  `go test ./...` reste vert sans Postgres. Voir `stats-plugin/store_test.go`.

### Note de migration du schéma

P4 a ajouté la colonne `instance` (et la table `missions`) : les tables d'un
`stats` créé avant P4 ne s'y conforment pas. Comme les données du plugin sont
**entièrement dérivées** du manager, le plus simple est de repartir de zéro :

```sql
DROP TABLE IF EXISTS stat_snapshots, sync_runs, sync_cursor, events, chat, missions;
```

La migration complète le schéma au démarrage. (Une vraie migration additive
serait possible si ce mode dépassait le stade de PoC.)

---

## 12. Décision

1. **Fait** : plugin livré (topologie A, S1 + miroir S2), manager inchangé hors
   l'endpoint additif ;
2. **Ensuite** : P4 (multi-managers, auth, exports), si le besoin se confirme ;
3. **Toujours pas** de refonte de la persistance du manager (cf.
   `database-backends.md`), sauf besoin explicite.

Ce document couvre la voie « plugin ». La voie « changer le moteur du cœur » est
traitée dans [`database-backends.md`](database-backends.md).
