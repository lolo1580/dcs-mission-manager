# Manuel d'installation — DCS Manager

Ce guide couvre l'installation complète, du téléchargement à la première mission
enregistrée, ainsi que le dépannage.

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
7. [Mise à jour et désinstallation](#7-mise-à-jour-et-désinstallation)
8. [Dépannage](#8-dépannage)
9. [Référence CLI](#9-référence-cli)
10. [Ports et pare-feu](#10-ports-et-pare-feu)

---

## 1. Ce dont vous avez besoin

| Élément | Requis | Notes |
|---|---|---|
| Windows | 10 ou 11 | Le manager utilise **WebView2**, fourni avec Windows 10/11 |
| DCS World | ✅ | Version stable ou Open Beta |
| Accès à `Saved Games\DCS\` | ✅ | ou `DCS.openbeta` |
| LuaSocket | ✅ | **fourni avec DCS**, rien à installer |
| Droits administrateur | ❌ | aucune étape n'exige d'élévation |

Pour **construire l’installateur depuis les sources** uniquement, il faut en plus :

- **Go 1.25+** — <https://go.dev/dl/> (`winget install GoLang.Go`)
- **Node.js 20+** — <https://nodejs.org/>
- **Git**
- **Inno Setup 6.7+**

> L’installateur embarque l’application et ses scripts Lua : ni Go ni Node ne
> sont nécessaires pour l’utiliser.

---

## 2. Installer depuis une release (recommandé)

### 2.1 Télécharger

Sur la page **Releases** du dépôt, récupérez :

| Fichier | Rôle |
|---|---|
| `DCSManager-Setup-<version>.exe` | Installateur Windows contenant l’application et les scripts Lua |
| `SHA256SUMS.txt` | Sommes de contrôle, pour vérifier l'intégrité |

Vérification facultative de l'exécutable :

```powershell
(Get-FileHash .\DCSManager-Setup-<version>.exe -Algorithm SHA256).Hash.ToLower()
# à comparer à la ligne correspondante de SHA256SUMS.txt
```

Lancez l’installateur et suivez l’assistant. Il installe le programme pour votre
compte Windows, sans droits administrateur. Les données sont conservées dans
`%LOCALAPPDATA%\DCS Manager` lors des mises à jour. Voir aussi
[le parcours détaillé de l’assistant](installation-windows.fr.md).

### 2.2 Installer les scripts côté DCS

Dans l’assistant, vérifiez le dossier Saved Games détecté et cochez l’option
**Installer / mettre à jour les scripts Lua DCS Manager** si vous souhaitez les
installer immédiatement. Fermez DCS avant cette étape. Vous pouvez aussi les
installer plus tard depuis **Paramètres → Installation DCS** dans l’application.
L’installation fusionne le bloc DCS Manager dans `Export.lua` et conserve les
autres exports. La [section 4](#4-installer-les-scripts-lua-dans-dcs) décrit les
commandes manuelles et les contrôles.

### 2.3 Lancer

Ouvrez **DCS Manager** depuis le menu Démarrer. Détails au [point 5](#5-premier-lancement).

---

## 3. Installer depuis les sources (développement)

### 3.1 Récupérer le code

```powershell
git clone <url-du-depot> "DCS Manager"
cd "DCS Manager"
```

### 3.2 Construire

Le script construit l’interface, l’exécutable interne et l’installateur :

```powershell
.\build.ps1              # crée dist\DCSManager-Setup-<version>.exe
```

Autres cibles utiles :

```powershell
.\build.ps1 -Target frontend   # interface seulement
.\build.ps1 -Target backend    # exécutable de développement seulement
.\build.ps1 -Target run        # lancer depuis les sources
.\build.ps1 -Target test       # tests Go
```

`make build` construit uniquement un exécutable de développement ; il ne produit
pas l’installateur Windows.

### 3.3 Enchaîner build + installation

```powershell
.\build.ps1
$version = (Get-Content VERSION -Raw).Trim()
& ".\dist\DCSManager-Setup-$version.exe"
```

L’assistant propose l’installation des scripts Lua. L’exécutable de développement
et les commandes CLI restent disponibles pour les tests locaux ; seule la version
installable est publiée.

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

Depuis l’application, ouvrez **Paramètres → Installation DCS → Installer / mettre
à jour les scripts**. En ligne de commande, depuis le dossier installé :

```powershell
Set-Location "$env:LOCALAPPDATA\Programs\DCS Manager"
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

Ouvrez **DCS Manager** depuis le menu Démarrer. Pour les commandes avancées
ci-dessous, ouvrez PowerShell dans le dossier du programme :

```powershell
Set-Location "$env:LOCALAPPDATA\Programs\DCS Manager"
.\dcsmanager.exe version
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

- Dans la fenêtre du manager, les pages **Carrière** et **Statistiques** se remplissent.
- Les débriefings sont collectés après un vol ; leur vue est actuellement masquée.
- En cas de doute, `GET /api/health` répond :

  ```powershell
  Invoke-RestMethod http://localhost:8080/api/health
  ```

### 5.4 Où sont les données

| Chemin | Contenu |
|---|---|
| `%LOCALAPPDATA%\DCS Manager\data\dcsmanager.db` | Base SQLite (missions, événements, stats, débriefings) |
| `%LOCALAPPDATA%\DCS Manager\data\dcsmanager.log` | Journal de l’application |
| `%LOCALAPPDATA%\DCS Manager\settings.json` | Dossiers choisis dans l’assistant |

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
| `DCSMANAGER_DB_PATH` | `%LOCALAPPDATA%\DCS Manager\data\dcsmanager.db` | Base SQLite |
| `DCSMANAGER_DB_ENABLED` | `true` | Persistance (sinon tout en mémoire) |
| `DCSMANAGER_THEATRE` | `Caucasus` | Théâtre par défaut |
| `DCSMANAGER_SAVED_GAMES` | *(auto)* | Dossier Saved Games, si la détection échoue |
| `DCSMANAGER_CHARTS_DIR` | `%LOCALAPPDATA%\DCS Manager\maps_dcs` | Cartes aéronautiques (approches, plans) |
| `DCSMANAGER_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `DCSMANAGER_DEBUG` | `false` | Journal de débogage en direct (actions, requêtes API, panneaux). S'active aussi depuis Paramètres → Débogage |

Exemple — exposer l'interface au réseau local en mode `serve` :

```powershell
$env:DCSMANAGER_HTTP_ADDR = "0.0.0.0:8080"
.\dcsmanager.exe serve
```

Réglages côté DCS : `Saved Games\DCS\Config\dcsmanager.cfg` (hôte, ports,
intervalles). Voir [`dcs-installation.md`](dcs-installation.md).

---

## 7. Mise à jour et désinstallation

### Mettre à jour

1. Fermez DCS Manager, téléchargez le nouvel `DCSManager-Setup-<version>.exe`
   et relancez l’assistant. Il remplace le programme et conserve vos données.
2. Si les scripts Lua doivent être mis à jour, fermez DCS et cochez leur option
   dans l’assistant, ou utilisez **Paramètres → Installation DCS** ensuite.
3. La commande `.\dcsmanager.exe status`, depuis le dossier du programme,
   indique si les scripts sont **à jour**, **obsolètes** ou **absents**.

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

## 8. Dépannage

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

## 9. Référence CLI

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

## 10. Ports et pare-feu

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
