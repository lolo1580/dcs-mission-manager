# Plugin de statistiques — lecture directe de PostgreSQL

> Architecture, réalisée. Complément de
> [`database-backends.md`](database-backends.md).
>
> Code : [`stats-plugin/`](../stats-plugin/). Ce document décrit le **modèle
> « bibliothèque partagée »** : PostgreSQL est la source commune, le manager
> écrit, le plugin lit.

---

## 1. Principe

Le manager et le plugin **ne communiquent pas directement**. Ils partagent une
base PostgreSQL :

```
   Machine DCS                                  Autre machine / Docker
 ┌───────────────────────┐                    ┌──────────────────────────┐
 │ dcsmanager.exe        │   écrit            │ stats-web (plugin)       │
 │  DCSMANAGER_DB_DRIVER │ ─────────► PostgreSQL ◄─────────  lit (SELECT) │
 │  =postgres            │                    │  + dashboard             │
 └───────────────────────┘                    └──────────────────────────┘
```

- Le manager **écrit** dans PostgreSQL (capacité déjà présente via le refactor
  `db.Store`, `DCSMANAGER_DB_DRIVER=postgres`).
- Le plugin **lit les tables du manager** (`missions`, `events`, `players`,
  `player_stats`, `debriefs`) et agrège en SQL.
- Le plugin **n'appelle jamais l'API HTTP** du manager : le jeton d'API
  (`DCSMANAGER_API_TOKEN`) n'est donc **pas nécessaire** pour le plugin.

## 2. Pourquoi ce modèle

| Avantage | Détail |
|---|---|
| Manager non exposé | Il peut rester lié à `127.0.0.1`. Aucun port à ouvrir, aucune API à protéger |
| Pas de miroir ni d'instantanés | La data est déjà en base ; plus de `stat_snapshots`, `events`, `chat`, `sync_cursor` propres au plugin |
| Tendances exactes | La série se calcule sur `events.real_ts` : complète, pas échantillonnée |
| Séparation nette | Le conteneur plugin peut tourner n'importe où, tant qu'il joint la base |

## 3. Contrainte assumée

Le mode « bibliothèque partagée » **impose PostgreSQL** : SQLite est un fichier
local, illisible depuis une autre machine ou un conteneur. C'est inhérent au
choix. Le « SQLite par défaut » reste le mode desktop pur ; le partage passe par
Postgres.

## 4. Le plugin est un lecteur, garanti par le SGBD

Deux niveaux :

1. **Session read-only** — le pool s'ouvre avec
   `default_transaction_read_only=on` : aucune requête ne peut écrire ;
2. **Rôle SELECT-only** (recommandé) — le plugin se connecte avec un rôle
   `stats_reader` qui n'a que `SELECT` :

   ```sql
   CREATE ROLE stats_reader LOGIN PASSWORD '...';
   GRANT CONNECT ON DATABASE dcsmanager TO stats_reader;
   GRANT USAGE   ON SCHEMA public      TO stats_reader;
   GRANT SELECT  ON ALL TABLES IN SCHEMA public TO stats_reader;
   ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO stats_reader;
   ```

« Ne fait que lire » devient une garantie du moteur, pas une promesse.

## 5. Couplage au schéma : les vues `v_stats_*`

Le plugin lit **l'interface en lecture seule du manager**, et non ses tables
directement. Le manager expose des vues `v_stats_*` (créées par le store
PostgreSQL dans `migrate()`) ; le plugin ne requête que celles-ci :

| Vue | Contenu |
|---|---|
| `v_stats_missions` | id, nom, théâtre, **source**, début, fin, vainqueur |
| `v_stats_players` | id, ucid, nom, premières/dernières vues |
| `v_stats_player_stats` | instantanés par joueur + **source** |
| `v_stats_events` | événements + **source** |
| `v_stats_debriefs` | débriefs + **source** |
| `v_stats_track_positions` | positions + **source** |
| `v_stats_config` | compteurs (missions, joueurs, événements, débriefs, positions, missions de test) |

Chaque vue dérivée porte une colonne **`source`** : la politique de test est donc
appliquée par le lecteur (`source <> 'test'`, sauf `INCLUDE_TEST`) sans avoir à
connaître la table `missions`.

**Pourquoi des vues.** Le schéma interne peut alors évoluer — une colonne
renommée, une table scindée — sans casser les lecteurs, tant que la vue garde sa
forme. Un lecteur ne reçoit `SELECT` que sur les vues. C'est la contrepartie du
couplage direct, levée.

L'agrégation reproduit fidèlement celle du manager (`internal/stats`) :

- pilotes : **dernier instantané par (mission, joueur)** uniquement — sinon les
  compteurs cumulés, renvoyés toutes les quelques secondes, seraient multipliés
  par la fréquence d'échantillonnage ;
- morts et fratricide : dérivés des `events`, en résolvant l'id joueur DCS
  **par mission** (le même id peut désigner deux personnes selon la mission) ;
- politiques de test identiques (`source <> 'test'` sauf `INCLUDE_TEST`).

Les vues sont recréées avec `CREATE OR REPLACE` : les faire évoluer ne casse ni
les données ni le manager.

## 6. Requêtes dialect-sensibles

Reproduites depuis le store PostgreSQL du manager :

- pas de `GROUP BY` sur alias → expressions répétées dans une sous-requête ;
- `round(double, int)` n'existe pas → `round(x::numeric, 0)` ;
- `SUM(...)` renvoie `NULL` sans ligne → `COALESCE`.

La série temporelle utilise `to_timestamp(real_ts / 1000.0)::date` (les
horodatages sont en millisecondes Unix).

## 7. Ce qui a été retiré (ancienne approche API)

La version précédente du plugin interrogeait `/api/stats/*`, stockait des
instantanés et miroitait événements/chat via `?sinceId=`. Ce mode a été
**remplacé** par la lecture directe. Les éléments supprimés :

- client HTTP vers le manager, `sync.go`, `schema.sql`, tables propres ;
- les options `DCSMANAGER_URL`, `DCSMANAGER_TOKEN`, `SYNC_INTERVAL`,
  `MANAGER_NAME`, `MIRROR_*`, `SYNC_SCOPES`.

> Les endpoints `?sinceId=` ajoutés au manager restent utiles en soi (un
> consommateur d'API pourra s'en servir), mais le plugin ne les utilise plus.

## 8. Configuration

| Variable | Défaut | Rôle |
|---|---|---|
| `MANAGER_DATABASE_URL` | *(requis)* | DSN PostgreSQL du manager, ouvert en lecture |
| `PLUGIN_LISTEN_ADDR` | `:8090` | Tableau de bord du plugin |
| `QUERY_TIMEOUT` | `15s` | Délai d'une requête |
| `INCLUDE_TEST` | `false` | Inclure les missions simulées |
| `PLUGIN_AUTH_TOKEN` | *(vide)* | Protège l'API/le dashboard du plugin |

## 9. Surface d'API du plugin

`/api/plugin/health`, `/summary`, `/overview`, `/series?event=&days=`,
`/missions`, `/export?type=&format=`, plus `/pilots`, `/weapons`, `/engines`,
`/network`.

## 10. Docker

Le `docker-compose.yml` du plugin démarre **PostgreSQL + stats-web**. Si le plugin
et Postgres sont sur une autre machine que le manager, ils n'ont besoin que de la
base — pas de l'API du manager. Voir
[`stats-plugin/README.md`](../stats-plugin/README.md).

## 11. Ce qui reste ouvert

- **Tests d'intégration SQL** — ✅ couverts : `stats-plugin/store_test.go` exerce
  les agrégations contre un PostgreSQL réel quand `PLUGIN_TEST_DATABASE_URL` est
  défini : dernier instantané par mission/joueur, résolution de l'id DCS, politique
  de test, séries, et **rejet des écritures** (session read-only). La CI
  (`.github/workflows/ci.yml`, job `stats-plugin`) fournit un service PostgreSQL.
- **Vues read-only** — ✅ livrées : le manager crée `v_stats_*` (avec `source` et
  `v_stats_config`) et le plugin ne lit que celles-ci, de sorte que le schéma
  interne peut évoluer sans casser les lecteurs. Vérifié contre un PostgreSQL
  réel (7 vues présentes, `v_stats_config` correcte, suites de tests vertes).
- **Multi-managers** : aujourd'hui un plugin lit une base. Plusieurs managers
  partageraient une base (le `source`/les missions cohabitent), mais l'isolation
  par instance n'existe plus dans ce modèle.
