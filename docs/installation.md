# Manuel d'installation — DCS Manager

Ce guide couvre l'installation complète, du téléchargement à la première mission
enregistrée, ainsi que les options avancées (PostgreSQL, plugin de statistiques)
et le dépannage.

> DCS Manager est un **compagnon local de DCS World** : il tourne sur la **même
> machine Windows** que le simulateur. C'est ce qui lui permet de lire les
> fichiers de DCS (Saved Games, terrains) directement. Il n'y a ni serveur distant,
> ni compte à créer.

---

## Sommaire

1. [Ce dont vous avez besoin](#1-ce-dont-vous-avez-besoin)
2. [Installer depuis une release](#2-installer-depuis-une-release-recommandé)
3. [Installer depuis les sources](#3-installer-depuis-les-sources-développement)
4. [Installer les scripts Lua dans DCS](#4-installer-les-scripts-lua-dans-dcs)
5. [Premier lancement](#5-premier-lancement)
6. [Configuration](#6-configuration)
7. [Option : base PostgreSQL](#7-option--base-postgresql)
8. [Option : plugin de statistiques](#8-option--plugin-de-statistiques)
9. [Mise à jour et désinstallation](#9-mise-à-jour-et-désinstallation)
10. [Dépannage](#10-dépannage)
11. [Référence CLI](#11-référence-cli)
12. [Ports et pare-feu](#12-ports-et-pare-feu)

---

## 1. Ce dont vous avez besoin

| Élément | Requis | Notes |
|---|---|---|
| Windows | 10 ou 11 | Le manager utilise **WebView2**, fourni avec Windows 10/11 |
| DCS World | ✅ | Version stable ou Open Beta |
| Accès à `Saved Games\DCS\` | ✅ | ou `DCS.openbeta` |
| LuaSocket | ✅ | **fourni avec DCS**, rien à installer |
| Droits administrateur | ❌ | aucune étape n'exige d'élévation |

Pour **installer depuis les sources** uniquement, il faut en plus :

- **Go 1.25+** — <https://go.dev/dl/> (`winget install GoLang.Go`)
- **Node.js 20+** — <https://nodejs.org/>
- **Git**

> Le binaire final est **autonome** : ni Go ni Node ne sont nécessaires pour
> l'utiliser une fois construit ou téléchargé.

---

## 2. Installer depuis une release (recommandé)

### 2.1 Télécharger

Sur la page **Releases** du dépôt, récupérez :

| Fichier | Rôle |
|---|---|
| `dcsmanager.exe` | Le manager (interface web incluse dans le binaire) |
| `dcsmanager-lua-<version>.zip` | Les scripts Lua à installer dans DCS (si vous préférez l'installateur manuel) |
| `SHA256SUMS.txt` | Sommes de contrôle, pour vérifier l'intégrité |

Vérification facultative de l'exécutable :

```powershell
(Get-FileHash .\dcsmanager.exe -Algorithm SHA256).Hash.ToLower()
# à comparer à la ligne correspondante de SHA256SUMS.txt
```

Placez `dcsmanager.exe` dans un dossier de votre choix (par exemple
`C:\DCS Manager\`).

### 2.2 Installer les scripts côté DCS

Le binaire sait installer les scripts tout seul, de façon **sûre** (il *fusionne*
avec un `Export.lua` existant, il ne l'écrase jamais) :

```powershell
.\dcsmanager.exe install-lua
```

Simuler d'abord, sans rien écrire :

```powershell
.\dcsmanager.exe install-lua --dry-run
```

Vérifier l'état à tout moment :

```powershell
.\dcsmanager.exe status
```

Si DCS n'est pas détecté automatiquement :

```powershell
.\dcsmanager.exe install-lua --saved-games "D:\Saved Games\DCS"
```

> Vous préférez le faire à la main ? Voir la [section 4](#4-installer-les-scripts-lua-dans-dcs).

### 2.3 Lancer

```powershell
.\dcsmanager.exe
```

Le manager s'ouvre dans **sa propre fenêtre**. Détails au [point 5](#5-premier-lancement).

---

## 3. Installer depuis les sources (développement)

### 3.1 Récupérer le code

```powershell
git clone <url-du-depot> "DCS Manager"
cd "DCS Manager"
```

### 3.2 Construire

Le script construit d'abord l'interface (Svelte + Vite), puis le binaire Go qui
l'embarque :

```powershell
.\build.ps1              # frontend + backend -> .\dcsmanager.exe
```

Autres cibles utiles :

```powershell
.\build.ps1 -Target frontend   # interface seulement
.\build.ps1 -Target backend    # binaire seulement (interface déjà construite)
.\build.ps1 -Target run        # lancer depuis les sources
.\build.ps1 -Target test       # tests Go
```

Équivalent avec `make` : `make build`, `make run`, `make test`.

### 3.3 Enchaîner build + installation

```powershell
.\build.ps1
.\install-dcs.ps1              # fusionne les scripts Lua dans Saved Games
.\install-dcs.ps1 -DryRun      # simulation
```

`install-dcs.ps1` n'est qu'un raccourci vers `dcsmanager.exe install-lua`.

---

## 4. Installer les scripts Lua dans DCS

> ⚠️ **Toujours fusionner, jamais écraser.** `Export.lua` est très souvent déjà
> modifié par Tacview, SRS, DCS-BIOS, etc. **Sauvegardez** le fichier existant
> avant toute modification.

DCS charge deux familles de scripts depuis le dossier *Saved Games* :

```
%USERPROFILE%\Saved Games\DCS\          (ou DCS.openbeta)
├─ Config\
│   └─ dcsmanager.cfg          ← adresse du backend
└─ Scripts\
    ├─ Export.lua              ← positions (télémétrie) → UDP
    └─ Hooks\
        └─ dcsmanager.lua      ← événements, joueurs, chat → TCP
```

### 4.1 Méthode automatique (recommandée)

```powershell
.\dcsmanager.exe install-lua
```

L'installateur encadre son code par des marqueurs, ce qui rend l'opération
réversible (`uninstall-lua`) et évite d'écraser le code des autres outils.

### 4.2 Méthode manuelle

1. Copiez `Config\dcsmanager.cfg` dans `Saved Games\DCS\Config\`.
2. **Si `Saved Games\DCS\Scripts\Export.lua` existe déjà** :
   - faites un backup horodaté :

     ```powershell
     Copy-Item "$env:USERPROFILE\Saved Games\DCS\Scripts\Export.lua" `
               "$env:USERPROFILE\Saved Games\DCS\Scripts\Export.lua.bak-$(Get-Date -Format yyyyMMdd)"
     ```
   - ouvrez `dcs-lua\Export.lua`, copiez **le bloc `do … end`** complet, et
     collez-le **à la fin** de votre `Export.lua` ;
   - vérifiez qu'il n'y a qu'une seule paire active de fonctions
     `LuaExportStart` / `LuaExportActivityNextEvent`.
3. **Sinon**, copiez simplement `dcs-lua\Export.lua` à cet emplacement.
4. Copiez `dcs-lua\Hooks\dcsmanager.lua` dans `Scripts\Hooks\`.
5. **Redémarrez DCS.**

Détails complémentaires : [`dcs-installation.md`](dcs-installation.md).

---

## 5. Premier lancement

### 5.1 Démarrer le manager

```powershell
.\dcsmanager.exe
```

Le manager s'ouvre dans une **fenêtre native** (composant WebView2). Le serveur
HTTP tourne derrière la fenêtre — c'est lui qui sert l'API et l'interface.

Pour l'utiliser depuis un navigateur (second écran, tablette) :

```powershell
.\dcsmanager.exe serve
# puis ouvrir http://localhost:8080
```

> Le manager **refuse de démarrer deux fois**. Si une instance tourne déjà,
> la fenêtre affiche un message, et `serve` se termine avec le code `3`.

### 5.2 Lancer une mission dans DCS

Chargez une mission : le manager enregistre les événements, les joueurs, les
statistiques, et sauvegarde chaque mission pour les débriefings.

### 5.3 Vérifier que tout fonctionne

- Dans la fenêtre du manager, l'onglet **Carrière & statistiques** se remplit.
- Les débriefings apparaissent après un vol dans l'onglet **Débriefings**.
- En cas de doute, `GET /api/health` répond :

  ```powershell
  Invoke-RestMethod http://localhost:8080/api/health
  ```

### 5.4 Où sont les données

| Chemin | Contenu |
|---|---|
| `.\data\dcsmanager.db` | Base SQLite (missions, événements, stats, débriefings) |
| `.\data\dcsmanager.log` | Journal (utile en mode fenêtre, sans console) |
| `.\dcsmanager.log` / console | Journal en mode `serve` |

Le chemin de la base est réglable via `DCSMANAGER_DB_PATH`.

---

## 6. Configuration

Toute la configuration passe par des **variables d'environnement** préfixées
`DCSMANAGER_`. Aucune n'est obligatoire pour une installation normale.

| Variable | Défaut | Description |
|---|---|---|
| `DCSMANAGER_HTTP_ADDR` | `127.0.0.1:8080` | Adresse de l'interface web + SSE. `0` choisit un port libre (fenêtre native). `0.0.0.0:8080` l'expose au réseau — **protégez-la alors avec `DCSMANAGER_API_TOKEN`** |
| `DCSMANAGER_API_TOKEN` | *(vide)* | Si défini, les appels **non locaux** doivent le présenter. L'accès local (loopback) reste autorisé sans jeton |
| `DCSMANAGER_UDP_ADDR` | `127.0.0.1:7776` | Télémétrie des unités. **Pas 7778** : DCS-BIOS l'occupe |
| `DCSMANAGER_TCP_ADDR` | `127.0.0.1:7779` | Événements + commandes |
| `DCSMANAGER_DB_PATH` | `./data/dcsmanager.db` | Base SQLite |
| `DCSMANAGER_DB_ENABLED` | `true` | Persistance (sinon tout en mémoire) |
| `DCSMANAGER_DB_DRIVER` | `sqlite` | `sqlite` ou `postgres` (voir §7) |
| `DCSMANAGER_DB_DSN` | *(vide)* | Chaîne de connexion PostgreSQL |
| `DCSMANAGER_THEATRE` | `Caucasus` | Théâtre par défaut |
| `DCSMANAGER_SAVED_GAMES` | *(auto)* | Dossier Saved Games, si la détection échoue |
| `DCSMANAGER_CHARTS_DIR` | `./maps_dcs` | Cartes aéronautiques (approches, plans) |
| `DCSMANAGER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

Exemple — exposer l'interface au réseau local en mode `serve` :

```powershell
$env:DCSMANAGER_HTTP_ADDR = "0.0.0.0:8080"
.\dcsmanager.exe serve
```

Réglages côté DCS : `Saved Games\DCS\Config\dcsmanager.cfg` (hôte, ports,
intervalles). Voir [`dcs-installation.md`](dcs-installation.md).

---

## 7. Option : base PostgreSQL

Par défaut, DCS Manager utilise **SQLite** : rien à installer, tout est dans le
binaire. Une base **PostgreSQL** est possible pour un usage avancé. Le moteur est
choisi au démarrage : **un redémarrage est nécessaire** pour changer.

### 7.1 Démarrer un PostgreSQL (Docker)

Un `docker-compose.yml` est fourni à la racine :

```powershell
docker compose up -d postgres
```

### 7.2 Configurer et lancer le manager

```powershell
$env:DCSMANAGER_DB_DRIVER = "postgres"
$env:DCSMANAGER_DB_DSN    = "postgres://dcs:dcs@localhost:5432/dcsmanager?sslmode=disable"
.\dcsmanager.exe
```

Le manager crée son schéma au premier démarrage.

> **Pas de migration automatique depuis SQLite** : c'est un mode « nouveau
> départ ». Les données SQLite existantes ne sont pas copiées.

Détails et compromis : [`database-backends.md`](database-backends.md).

---

## 8. Option : plugin de statistiques

Le **plugin de statistiques** est un service **optionnel et séparé**. Dans ce
modèle, **PostgreSQL est la bibliothèque partagée** : le manager y écrit, et le
plugin lit directement ses tables pour servir son propre tableau de bord. Le
plugin n'appelle **pas** l'API du manager.

```
Manager (machine DCS) ── écrit ──► PostgreSQL ◄── lit (SELECT) ── Plugin (Docker)
```

> **Prérequis** : le manager doit tourner sur **PostgreSQL** (section 7), pas sur
> SQLite. SQLite est un fichier local, illisible depuis un autre conteneur ou une
> autre machine. Le plugin hérite donc du mode « bibliothèque partagée ».

### 8.1 Côté manager — écrire dans PostgreSQL

```powershell
$env:DCSMANAGER_DB_DRIVER = "postgres"
$env:DCSMANAGER_DB_DSN    = "postgres://dcs:dcs@localhost:5432/dcsmanager?sslmode=disable"
.\dcsmanager.exe
```

Le manager reste sur `127.0.0.1` : **rien à exposer**, aucune API à protéger,
aucun port pare-feu à ouvrir.

### 8.2 Côté plugin — lire la base

```powershell
cd stats-plugin
$env:MANAGER_DATABASE_URL = "postgres://stats_reader:change-me@localhost:5432/dcsmanager?sslmode=disable"
go run .
# tableau de bord : http://localhost:8090
```

Le plugin ouvre la base **en lecture seule** (`default_transaction_read_only=on`),
et il est conseillé de lui donner un **rôle SELECT-only** :

```sql
CREATE ROLE stats_reader LOGIN PASSWORD 'change-me';
GRANT CONNECT ON DATABASE dcsmanager TO stats_reader;
GRANT USAGE   ON SCHEMA public      TO stats_reader;
GRANT SELECT  ON ALL TABLES IN SCHEMA public TO stats_reader;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT ON TABLES TO stats_reader;
```

### 8.3 Plugin sur une autre machine

Le plugin (et PostgreSQL) peuvent tourner **n'importe où**, tant que la base est
joignable. Comme le plugin ne parle pas au manager en HTTP, il n'y a **pas de
jeton d'API à configurer** pour lui : il suffit que l'adresse de PostgreSQL soit
accessible (réseau privé, port 5432). Le manager, lui, n'a toujours rien à
exposer.

Guide complet : [`stats-plugin/README.md`](../stats-plugin/README.md).

> Si vous avez un jour besoin d'exposer l'**interface/API du manager** elle-même
> (tablette, navigateur distant), faites-le avec `DCSMANAGER_API_TOKEN` : voir la
> section 6. Ce jeton **n'est pas utilisé par le plugin** dans ce modèle.

---

## 9. Mise à jour et désinstallation

### Mettre à jour

1. Téléchargez le nouvel `dcsmanager.exe` et remplacez l'ancien.
2. Relancez `.\dcsmanager.exe install-lua` pour mettre à jour les scripts Lua.
3. `.\dcsmanager.exe status` indique si les scripts installés sont **à jour**,
   **obsolètes** ou **absents**.

### Désinstaller les scripts Lua

Retire le bloc géré de `Export.lua`/hooks, **sans** toucher au code des autres
outils :

```powershell
.\dcsmanager.exe uninstall-lua
```

### Supprimer les données enregistrées (destructif)

```powershell
.\dcsmanager.exe purge --source test     # sessions simulées uniquement
.\dcsmanager.exe purge --mission-id 3    # une mission et tout ce qui y est lié
.\dcsmanager.exe purge --all             # tout (garde le schéma et les joueurs)
.\dcsmanager.exe purge --all --dry-run   # montrer sans supprimer
```

Chaque session est étiquetée `live` (DCS) ou `test` (outils de test). Les
statistiques excluent `test` par défaut.

---

## 10. Dépannage

| Symptôme | Piste |
|---|---|
| La fenêtre affiche « another instance is already running » | Une instance tourne déjà sur `DCSMANAGER_HTTP_ADDR`. Fermez-la, ou changez le port |
| Rien n'apparaît dans les statistiques | Scripts Lua non installés (`dcsmanager status`), ou DCS non redémarré |
| Aucune télémétrie | `dcsmanager.cfg` manquant/au mauvais endroit (il doit être dans `Config\`) |
| Conflit avec DCS-BIOS | Les deux se côtoient : le manager utilise UDP 7776, DCS-BIOS 7778. Ne remplacez pas 7776 par 7778 |
| `LuaSocket introuvable` | Installation DCS incomplète ; LuaSocket est fourni avec DCS |
| En mode fenêtre, aucune console | Regardez `data\dcsmanager.log` |
| Le manager ne trouve pas DCS | Renseignez `DCSMANAGER_SAVED_GAMES` ou `--saved-games` |
| L'interface n'est pas joignable d'un autre appareil | Lancez `serve` avec `DCSMANAGER_HTTP_ADDR=0.0.0.0:8080` (et restreignez au réseau local) |

---

## 11. Référence CLI

```text
dcsmanager                 Ouvre le manager dans une fenêtre native
dcsmanager serve           Lance le serveur seul ; interface sur http://localhost:8080
dcsmanager install-lua     Installe/fusionne les scripts Lua dans Saved Games
dcsmanager uninstall-lua   Retire le bloc installé (garde la config)
dcsmanager status          Installé / obsolète / manquant, par fichier
dcsmanager purge           Supprime des sessions (destructif ; voir options)
dcsmanager version         Affiche la version

install-lua / uninstall-lua / status :
  --saved-games <dir>   Dossier Saved Games (auto-détecté)
  --lua-dir <dir>       Dossier dcs-lua de la distribution (auto-détecté)
  --dry-run             N'écrit rien, montre les actions

purge (exactement une option requise) :
  --source test|live    Supprime les sessions d'une source
  --mission-id <n>      Supprime une mission et tout ce qui y est lié
  --all                 Supprime toutes les sessions
```

---

## 12. Ports et pare-feu

| Port | Protocole | Usage |
|---|---|---|
| 7776 | UDP | Télémétrie des unités (depuis DCS) |
| 7779 | TCP | Événements, joueurs, chat, commandes |
| 8080 | TCP | Interface web + SSE (`/api/*`) |

Par défaut, tout écoute sur **`127.0.0.1`** : rien n'est exposé au réseau et
aucune règle de pare-feu n'est nécessaire. Si vous élargissez l'écoute
(`0.0.0.0`), pensez que **l'API n'a pas d'authentification** et contient un
endpoint destructif (`purge`) : réservez-la au réseau local.

---

## Voir aussi

- [`../README.md`](../README.md) — vue d'ensemble, fonctionnalités, architecture
- [`dcs-installation.md`](dcs-installation.md) — installation côté DCS en détail
- [`architecture.md`](architecture.md) — composants internes et flux de données
- [`database-backends.md`](database-backends.md) — SQLite par défaut, PostgreSQL en option
- [`stats-plugin.md`](stats-plugin.md) — plugin de statistiques (lecture PostgreSQL)
- [`stats-plugin/README.md`](../stats-plugin/README.md) — plugin de statistiques
