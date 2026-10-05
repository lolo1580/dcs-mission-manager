# Backend de base de données optionnel : SQLite par défaut, PostgreSQL en option

> Étude d'architecture, **réalisée** (refactor `db.Store` + implémentation
> PostgreSQL). Voir la section « État : livré » en fin de document.
> Objectif : permettre à l'utilisateur de **choisir son moteur de base**, avec
> **SQLite par défaut** (comportement inchangé) et **PostgreSQL en option**.

---

## 0. État : livré

Le refactor décrit ici a été implémenté :

- **`db.Store`** (`backend/internal/db/store.go`) : interface de persistance
  (+ `Querier` pour les requêtes analytiques). SQLite (`*db.DB`) et PostgreSQL
  (`internal/db/postgres`) l'implémentent, prouvé par `var _ db.Store = …`.
- **Consommateurs basculés sur l'interface** : `api`, `app`, `ingest`, `stats`,
  `tracker`, `debriefstore` ne connaissent plus `*db.DB`.
- **`internal/db/postgres`** : schéma, `RETURNING id` (remplace `LastInsertId`),
  rebind `?`→`$n`, `round(numeric)` + sous-requête pour la heatmap, index
  « une seule mission ouverte » adapté.
- **Config** : `DCSMANAGER_DB_DRIVER=sqlite|postgres` + `DCSMANAGER_DB_DSN` ;
  `app.openStore` choisit l'implémentation, en gardant un **vrai nil d'interface**
  quand la persistance est désactivée.
- **Suite de conformité** (`internal/db/conformance_test.go`) : le même contrat
  `db.Store` rejoué contre SQLite toujours, et contre PostgreSQL quand
  `DCSMANAGER_TEST_POSTGRES_DSN` est défini.
- **`docker-compose.yml`** à la racine : un PostgreSQL pour tester l'option.

> **Sécurité réseau.** Exposer le manager (`DCSMANAGER_HTTP_ADDR=0.0.0.0:8080`)
> désactivait jusqu'ici toute protection : `localOnly` tombait, et l'API — qui
> **n'a pas d'authentification** — porte un endpoint destructeur (`purge`).
> Un **jeton d'API optionnel** (`DCSMANAGER_API_TOKEN`) a été ajouté : quand il
> est défini, les appels **non-loopback** doivent le présenter (`Bearer` ou
> `?token=`), l'accès local restant libre. Le plugin envoie ce jeton via
> `DCSMANAGER_TOKEN`. C'est ce qui rend possible un plugin **sur une autre
> machine** sans exposer l'API.

---

## 1. Objet et recommandation

Le manager actuel s'appuie sur **SQLite, en pur Go, embarqué dans le binaire**
(`backend/internal/db/db.go`). L'ajout de PostgreSQL est **réalisable**, mais ce
n'est pas un simple changement de DSN : la couche de persistance expose aujourd'hui
son handle SQL brut et du SQL propre à SQLite est écrit à la main dans plusieurs
packages.

Recommandation : introduire une **interface de stockage** avec deux implémentations
(`sqlite`, `postgres`), et déplacer les requêtes analytiques des stats derrière une
petite abstraction dialect-aware. SQLite reste l'implémentation par défaut ; Postgres
est un mode **avancé**, activé par variable d'environnement, avec **redémarrage requis**.

Le choix « dans les settings » doit être compris ainsi : la configuration du manager
est **intégralement pilotée par variables `DCSMANAGER_*`** (`backend/internal/config/config.go`),
chargées **avant** l'ouverture de la base (`backend/internal/app/app.go:85-113`).
Le réglage est donc une **variable d'environnement persistée**, et l'onglet Settings
l'affiche / l'explique (le changement effectif demande un redémarrage, on ne bascule
pas de base à chaud).

---

## 2. État des lieux : ce qui doit changer exactement

### 2.1 Le schéma actuel est du SQLite en dur

`db.go:62-204` crée les tables et index avec une syntaxe 100 % SQLite :

| SQLite (`db.go`) | PostgreSQL équivalent |
|---|---|
| `id INTEGER PRIMARY KEY AUTOINCREMENT` | `id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY` |
| `REAL` (lat, lng, alt, t, g…) | `double precision` — **attention** : `REAL` en Postgres est un flottant 32 bits, il ne peut pas représenter exactement `real_ts` (millisecondes, ~1.7×10¹²) |
| `TEXT` | `text` (équivalent) |
| `INTEGER` (0/1 pour `owns`/`ownship`) | `smallint` ou `boolean` |
| DSN `...?_pragma=journal_mode(WAL)...` (`db.go:37`) | spécifique SQLite, à supprimer (`db.go:37`) |

### 2.2 L'index « une seule mission ouverte » est SQLite-only

`db.go:211` :

```sql
CREATE UNIQUE INDEX idx_missions_one_open ON missions((1)) WHERE ended_at IS NULL
```

Postgres **refuse une expression constante dans un index**. Il faut une colonne
dédiée, par exemple :

```sql
ALTER TABLE missions ADD COLUMN open_guard smallint;
-- open_guard = 1 tant que ended_at IS NULL, NULL sinon (via trigger ou code)
CREATE UNIQUE INDEX idx_missions_one_open ON missions(open_guard) WHERE ended_at IS NULL;
```

La logique `ensureSingleOpenMissionIndex` (`db.go:210-228`) doit être réécrite par
dialecte (la réconciliation des doublons reste identique).

### 2.3 `LastInsertId()` — non supporté par les drivers Postgres

Quatre emplacements à réécrire en `INSERT ... RETURNING id` :

- `backend/internal/db/persist.go:98` — `UpsertPlayer`
- `backend/internal/db/maintenance.go:79` — `EnsureMissionTagged`
- `backend/internal/db/maintenance.go:134` — `StartMission`
- `backend/internal/db/debrief.go:30` — `SaveDebrief`

### 2.4 Placeholders `?` → `$1, $2…`

Toutes les requêtes utilisent `?`. Postgres attend `$n`. Il y a **~60 requêtes**
concernées, réparties dans :

- `internal/db/persist.go`, `maintenance.go`, `tracking.go`, `debrief.go`
- `internal/stats/stats.go` (11 requêtes analytiques : lignes 153, 223, 244, 282,
  346, 370, 405, 447, 492, 507, 512)
- `internal/ingest/ingest.go:186`

### 2.5 `PRAGMA table_info` — introspection SQLite-only

`db.go:243` (`ensureColumn`) utilise `PRAGMA table_info(<table>)`. En Postgres :

```sql
SELECT 1 FROM information_schema.columns
WHERE table_name = $1 AND column_name = $2;
```
ou simplement `ALTER TABLE ... ADD COLUMN IF NOT EXISTS ...` (supporté depuis PG 9.6).

### 2.6 Deux incompatibilités de requête plus subtiles

**a) `GROUP BY` sur alias de sortie** — `internal/db/tracking.go:95` :

```sql
SELECT ROUND(lat / ?, 0) * ? AS glat, ROUND(lng / ?, 0) * ? AS glng, COUNT(*) AS weight
...
GROUP BY glat, glng
```

SQLite autorise `GROUP BY` sur un alias de la liste `SELECT` ; **Postgres non**.
Il faut répéter les expressions ou passer par une sous-requête :

```sql
SELECT glat, glng, COUNT(*) AS weight
FROM (
  SELECT round((lat / $1)::numeric, 0) * $2 AS glat,
         round((lng / $3)::numeric, 0) * $4 AS glng
  FROM track_positions WHERE ...
) g
GROUP BY glat, glng ORDER BY weight DESC LIMIT $5
```

**b) `ROUND(double, int)`** — `round(numeric, int)` existe, pas `round(double precision, int)`.
Il faut caster en `numeric` puis re-caster en `double precision` à la lecture
(cf. ci-dessus). Même remarque pour toute arithmétique de position.

### 2.7 Le handle SQL brut fuit hors du package `db`

`db.go:279` expose `func (d *DB) SQL() *sql.DB`. Utilisé par :

- `internal/stats/stats.go` (11 fois) — logique d'agrégation
- `internal/ingest/ingest.go:186` — vérification d'existence de mission
- les tests : `internal/ingest/ingest_test.go`, `internal/tracker/*_test.go`,
  `internal/db/maintenance_test.go`

C'est le point d'architecture à corriger : c'est **lui** qui rend le changement de
moteur envahissant.

### 2.8 Dépendance driver

Le binaire est aujourd'hui **CGO-free** (`db.go:1-5`), contrainte importante. Le
driver Postgres devra rester pur Go : **`github.com/jackc/pgx/v5`** (via
`pgx/v5/stdlib`) ou `lib/pq`. Recommandation : **pgx v5 / stdlib**.

---

## 3. Architecture cible

### 3.1 Interface de stockage

Extraire une interface depuis `*db.DB`, puis deux implémentations.

```
backend/internal/
├─ store/                     # interface + modèles partagés (ex-"db")
│   └─ store.go               # type Store interface { ... }
├─ store/sqlite/              # implémentation actuelle, déplacée
│   ├─ sqlite.go              # Open, schéma, migrations (SQLite)
│   ├─ persist.go
│   ├─ maintenance.go
│   ├─ tracking.go
│   └─ debrief.go
└─ store/postgres/            # nouvelle implémentation
    ├─ postgres.go
    ├─ schema.sql
    └─ ...
```

Interface indicative (≈ 27 méthodes + accès requêtes) :

```go
package store

type Store interface {
    Close() error

    // missions
    EnsureMission(name, theatre string) (int64, error)
    EnsureMissionTagged(name, theatre, source string) (int64, error)
    StartMission(name, theatre, source string) (int64, error)
    OpenMissionID() int64
    EndOpenMission(winner string) error
    UpgradeMissionSource(id int64, source string) error
    CountMissions(source string) (int, error)
    Missions(limit int) ([]model.Mission, error)
    MissionsWithSource(source string, limit int) ([]model.Mission, error)

    // events / chat / stats / players
    SaveEvent(missionID int64, e model.Event) error
    SaveChat(missionID int64, c model.Chat) error
    SaveStats(missionID, playerID int64, p model.Player) error
    UpsertPlayer(ucid, name string) (int64, error)
    RecentEvents(limit int) ([]model.Event, error)
    RecentChat(limit int) ([]model.Chat, error)

    // tracking
    SaveSamples(missionID int64, samples []model.Sample) error
    SaveLoss(missionID int64, s model.Sample) error
    Heatmap(missionID int64, source string, grid float64, limit int) ([]HeatPoint, error)
    Trails(missionID int64, limitUnits, maxPointsPerUnit int) (map[string][]TrailPoint, error)
    PruneTracking(olderThan time.Duration) (int64, error)

    // debriefs
    SaveDebrief(rec model.Debrief) (model.Debrief, error)
    Debriefs(limit int) ([]model.Debrief, error)
    Debrief(id int64) (model.Debrief, error)

    // maintenance
    PurgeMission(id int64) (PurgeResult, error)
    PurgeSource(source string) (PurgeResult, error)
    PurgeAll() (PurgeResult, error)

    // accès requêtes dialect-aware (voir 3.3)
    Querier
}
```

### 3.2 Qui dépend de quoi

`stats`, `ingest`, `tracker`, `debriefstore`, `api`, `app` dépendent de
`store.Store` (interface), jamais d'une implémentation concrète. `app.go` choisit
l'implémentation à la construction.

### 3.3 Le point délicat : les requêtes analytiques des stats

`stats.go` fait de l'agrégation en Go parce que les `args` d'événements sont du JSON
dont le sens dépend du type (commentaire `stats.go:10-12`). Il contient donc du SQL
brut. Deux options :

**Option 1 — `Querier` dialect-aware (pragmatique, recommandée).**
Le `Store` expose une façade qui **réécrit les placeholders** selon le dialecte :

```go
type Querier interface {
    Query(query string, args ...any) (*sql.Rows, error)
    QueryRow(query string, args ...any) *sql.Row
}
```

- impl. SQLite : passe-plat, `?` tel quel ;
- impl. Postgres : `Rebind("?") → $1..$n` (algo de `sqlx.Rebind`, ou pgx).

`stats.go` ne change presque pas : `s.db.SQL().Query(...)` → `s.db.Query(...)`.
On garde les 11 requêtes **telles quelles**, seules les 3 incompatibilités de §2.6
sont corrigées dans les requêtes concernées (`tracking.go`, pas `stats.go`).

Avantage : diff minimal. Risque : `Rebind` naïf casse si un `?` apparaît dans un
littéral SQL — à vérifier, mais **aucune requête du projet n'a de `?` littéral**
(les JSON passent par des arguments).

**Option 2 — méthodes dédiées (puriste).**
Porter chaque requête stats dans le `Store` (`Pilots`, `Weapons`, `Coalitions`,
`Network`, `Overview`, `eventCountsByPlayer`…) avec une implémentation par dialecte.
Plus propre, mais double la maintenance des requêtes analytiques.

Recommandation : **Option 1**, avec correction ciblée des requêtes déjà
dialect-sensibles.

---

## 4. Schéma PostgreSQL

Traduction directe du schéma `db.go:62-204`. Points de vigilance :

```sql
CREATE TABLE IF NOT EXISTS missions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT NOT NULL,
    theatre    TEXT,
    source     TEXT NOT NULL DEFAULT 'live',
    started_at BIGINT NOT NULL,
    ended_at   BIGINT,
    winner     TEXT,
    open_guard SMALLINT               -- 1 si ouverte, NULL sinon (cf. index)
);

CREATE TABLE IF NOT EXISTS events (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mission_id BIGINT REFERENCES missions(id),
    event      TEXT NOT NULL,
    args       TEXT,
    detail     TEXT,
    t          DOUBLE PRECISION,
    real_ts    BIGINT NOT NULL
);
-- ... idem chat, players, player_stats, meta, debriefs,
--     track_positions, losses, avec lat/lng/alt DOUBLE PRECISION
```

Notes :

- **`real_ts BIGINT`** : en SQLite c'est `INTEGER`, donc un entier 64 bits. Le
  mapping correct côté Postgres est `BIGINT`, **pas** `REAL`.
- `ownship`/`owns` : `SMALLINT NOT NULL DEFAULT 0` (le code passe `boolInt`, `tracking.go:220`).
- Les index sont les mêmes qu'en SQLite, sauf `idx_missions_one_open` (§2.2).
- Le `meta` reste `key TEXT PRIMARY KEY`.
- Les noms de colonnes `"from"`, `type`, `g` nécessitent parfois des guillemets
  (déjà le cas pour `"from"` dans `persist.go:149`, `tracking.go:21`).

---

## 5. Migrations

Remplacer `ensureColumn` (`db.go:242-276`) par une version par dialecte :

- SQLite : `PRAGMA table_info` (inchangé) ;
- Postgres : `ALTER TABLE ... ADD COLUMN IF NOT EXISTS ...`, ou lecture de
  `information_schema.columns`.

La logique de réconciliation de §2.2 (`UPDATE missions SET ended_at = started_at
WHERE ended_at IS NULL AND id <> (SELECT ...)`) est portable telle quelle.

---

## 6. Configuration et « settings »

### 6.1 Nouvelles variables d'environnement

| Variable | Défaut | Rôle |
|---|---|---|
| `DCSMANAGER_DB_DRIVER` | `sqlite` | `sqlite` ou `postgres` |
| `DCSMANAGER_DB_DSN` | *(vide)* | DSN Postgres, ex. `postgres://dcs:pass@localhost:5432/dcsmanager?sslmode=disable` |
| `DCSMANAGER_DB_PATH` | `./data/dcsmanager.db` | inchangé, utilisé seulement si `driver=sqlite` |

`config.go` gagne `DBDriver` et `DBDSN` ; `app.go:102-113` fait :

```go
switch cfg.DBDriver {
case "postgres":
    database, err = postgres.Open(cfg.DBDSN)
default:
    database, err = sqlite.Open(cfg.DBPath)
}
```

En cas d'échec d'ouverture, le comportement actuel est conservé : la base est
désactivée et le manager démarre en mémoire (`app.go:106-108`).

### 6.2 Le « settings » dans l'UI

`SettingsPanel.svelte` (`frontend/src/lib/`) devient le lieu naturel pour un sous-onglet
**Base de données** montrant :

- le moteur actif (`GET /api/health` enrichi, ou un `GET /api/settings`) ;
- l'adresse/DSN (masqué) ;
- un bouton « Tester la connexion » (`POST /api/settings/db-test`) ;
- un texte clair : **le changement nécessite un redémarrage**, et comment le faire.

Comme la config est chargée une fois au démarrage, on ne peut pas basculer à chaud.
Deux niveaux possibles :

1. **Minimal** : l'UI affiche les variables à définir ; l'utilisateur les met dans
   son environnement / script de lancement.
2. **Confortable** : persister le choix dans `data/settings.json`, lu **avant**
   `db.Open`, et proposer « Redémarrer avec PostgreSQL » (écrit le fichier puis
   relance le process). À noter : cela introduit une source de configuration
   persistée à côté des variables d'environnement — à trancher explicitement.

---

## 7. Docker : PostgreSQL uniquement

Le manager reste un `.exe` Windows ; Docker ne sert qu'à **héberger Postgres**.
Le manager s'y connecte via `localhost:5432` (port mappé). Un `docker-compose.yml`
à la racine (ou dans `tools/`) :

```yaml
services:
  postgres:
    image: postgres:17-alpine
    container_name: dcsmanager-postgres
    environment:
      POSTGRES_USER: dcs
      POSTGRES_PASSWORD: dcs
      POSTGRES_DB: dcsmanager
    ports:
      - "5432:5432"
    volumes:
      - dcsmanager-pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dcs -d dcsmanager"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  dcsmanager-pgdata:
```

Lancement côté manager :

```powershell
$env:DCSMANAGER_DB_DRIVER = "postgres"
$env:DCSMANAGER_DB_DSN    = "postgres://dcs:dcs@localhost:5432/dcsmanager?sslmode=disable"
.\dcsmanager.exe
```

Aucun `Dockerfile` n'est nécessaire pour le manager (il n'est pas conteneurisé).

---

## 8. Migration des données existantes (SQLite → Postgres)

Outils possibles, par ordre de coût :

1. **Aucune migration** : Postgres est un mode « nouveau départ ». Le plus simple.
2. **Outil dédié** `dcsmanager migrate-db --from-sqlite ./data/dcsmanager.db --to <DSN>`
   qui ouvre les deux stores, lit mission par mission et réinsère via l'interface.
   Réutilise le `Store` — c'est un bénéfice direct du refactor.
3. **Dump SQL** : `sqlite3 .dump` + adaptation (RETURNING, types, index) — fragile,
   à éviter.

Recommandation : option 1 d'abord, option 2 ensuite (le refactor la rend facile).

> **Livré** : l'option 2 existe maintenant sous `dcsmanager migrate-db` (paquet
> `internal/migrate`). Elle copie toutes les tables au niveau SQL, préserve les
> ids, avance les séquences `IDENTITY` et est idempotente. Testée contre un
> PostgreSQL réel (copie, préservation des ids, idempotence, séquences).

---

## 9. Tests

Les tests qui ouvrent une base utilisent `db.Open(<temp>)` :
`internal/ingest`, `internal/tracker`, `internal/stats`, `internal/debriefstore`,
`internal/api/maintenance_test.go`, `internal/db/*_test.go`.

Stratégie proposée :

- définir une fabrique de test `newTestStore(t)` qui renvoie SQLite par défaut ;
- ajouter un jeu de tests paramétré : si `DCSMANAGER_TEST_POSTGRES_DSN` est défini,
  **rejouer toute la suite contre Postgres** (mêmes invariants, deux moteurs) ;
- les tests qui utilisent `SQL()` (`ingest_test.go`, `tracker_test.go`,
  `maintenance_test.go`) passent par le `Querier` de §3.3.

C'est le meilleur garde-fou : sans exécuter la suite contre Postgres, les
divergences de dialecte réapparaîtront.

---

## 10. Risques et compromis

| Risque | Détail | Mitigation |
|---|---|---|
| Contredit la promesse « local, zéro config » | `README.md:9`, `docs/architecture.md`, et l'en-tête `config.go:1-6` en font un **choix délibéré** | Présenter Postgres comme mode **avancé**, documenté, non défaut |
| Type `REAL` | Perte de précision sur `real_ts`/coordonnées si mappé en `real` 32 bits | Mapper en `double precision` / `bigint` |
| `GROUP BY` alias + `ROUND(double,int)` | 2 requêtes à réécrire | §2.6, tester la heatmap |
| Volume de requêtes | ~60 requêtes, 4 `LastInsertId` | `Querier` + rebind limite le diff |
| Driver | CGO-free à préserver | pgx v5 `stdlib` (pur Go) |
| Maintenance double | deux schémas, deux migrations, deux jeux de corrections | Suite de tests paramétrée |
| Basculer à chaud | impossible, config chargée au démarrage | redémarrage explicité dans l'UI |

---

## 11. Plan par phases

| Phase | Contenu | Dépend de | Ampleur |
|---|---|---|---|
| 1 | Extraire `store.Store`, sortir `SQL()` de l'API publique, remplacer par `Querier` | — | gros, mécanique |
| 2 | Porter `stats`, `ingest`, `tracker` sur l'interface | 1 | moyen |
| 3 | Implémentation Postgres : schéma, `RETURNING`, migrations, index ouvert | 1 | moyen |
| 4 | Config `DCSMANAGER_DB_DRIVER`/`DSN` + `docker-compose.yml` + sous-onglet Settings | 3 | petit |
| 5 | Suite de tests paramétrée SQLite/Postgres | 3 | moyen |
| 6 *(option)* | `migrate-db` SQLite→Postgres | 3 | petit |

---

## 12. Alternative : plugin stats séparé (sans toucher au cœur)

Si l'objectif réel est **d'avoir des stats « lourdes » dans une web app séparée avec
Postgres**, et non de migrer le stockage du manager, une variante beaucoup moins
risquée existe :

- garder **SQLite** dans le manager, inchangé ;
- un **service stats conteneurisé (Docker)** qui interroge l'API REST existante
  (`/api/stats/overview`, `/api/stats/pilots`, `/api/stats/weapons`,
  `/api/stats/engines`, `/api/stats/network`, cf. `internal/api/api.go:250-254`),
  écrit dans **Postgres**, et sert sa propre web app ;
- le manager tourne en `dcsmanager serve` avec `DCSMANAGER_HTTP_ADDR=0.0.0.0:8080`
  (attention : **pas d'authentification**, à restreindre au réseau local).

Le « switch » devient un réglage **du plugin**, pas du noyau. C'est la voie à
privilégier si l'on veut Postgres **sans** refondre la persistance du manager.

> Détail complet de cette variante : [`stats-plugin.md`](stats-plugin.md).

---

## 13. Décision attendue

1. Refonte complète `store.Store` + Postgres (phases 1-5) — intégré, coûteux ;
2. Plugin stats séparé (§12) — isolé, peu risqué ;
3. Les deux : plugin d'abord, refonte ensuite si le besoin grandit.

Le présent document couvre l'option 1 ; l'option 2 est esquissée en §12.
