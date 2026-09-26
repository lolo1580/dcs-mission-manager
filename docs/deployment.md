# Déploiement

Le manager se déploie **de deux façons, à partir du même projet** : un `.exe`
Windows, ou une image Docker Linux. Le binaire et la configuration sont
identiques ; seul l'emballage change.

> ⚠️ **DCS World ne tourne jamais dans le conteneur.** Le simulateur est
> Windows uniquement. Le conteneur ne contient que le *manager* et communique
> avec DCS par le réseau.

## Mode A — `.exe` Windows

Sur la machine qui fait tourner DCS.

```powershell
# 1. Builder (frontend + backend)
.\build.ps1

# 2. Installer les scripts côté DCS (fusion sûre dans Saved Games)
.\install-dcs.ps1            # ou .\install-dcs.ps1 -DryRun pour simuler

# 3. Lancer le manager
.\dcsmm.exe
```

L'adresse backend par défaut est `127.0.0.1` : tout est sur la même machine.

### En service Windows (démarrage automatique)

```powershell
# Nécessite NSSM ou le planificateur de tâches. Exemple avec le planificateur :
schtasks /create /tn "DCSMM" /tr "C:\chemin\dcsmm.exe" /sc onstart /ru SYSTEM
```

## Mode B — Docker Linux (autre machine)

Sur un serveur/NAS Linux, avec DCS sur une **autre** machine.

```bash
docker compose -f deploy/docker-compose.yml up -d
```

### Build multi-arch (NAS ARM)

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -f deploy/Dockerfile -t dcsmm:latest .
```

### Le point à ne pas manquer

Les scripts Lua tournent sur **Windows**, donc dans `Saved Games\DCS\Config\dcsmm.cfg`
il faut pointer vers l'**IP LAN de la machine Docker** — pas `127.0.0.1`, qui
bouclerait sur l'hôte Windows :

```lua
dcsmm_host = "192.168.1.50"   -- IP de la machine qui héberge le conteneur
dcsmm_udp_port = 7778
dcsmm_tcp_port = 7779
```

Et ouvrir les ports dans le pare-feu de la machine Docker :

| Port | Protocole | Usage |
|---|---|---|
| 8080 | TCP | Interface web |
| 7778 | UDP | Positions (live map) |
| 7779 | TCP | Événements, joueurs, chat, débriefs |

> Sur un NAS ou un Linux natif, aucun souci particulier. Sur **Docker Desktop
> (Windows/WSL2)**, la publication UDP vers le LAN est parfois capricieuse ;
> un `netsh portproxy` peut servir de secours.

## Injecteur Lua : `dcsmm install-lua`

L'installation côté DCS est **idempotente et sûre** :

- elle **ne remplace jamais** un `Export.lua` existant (Tacview, SRS, DCS-BIOS…) ;
- elle **fusionne** un bloc délimité par des marqueurs
  (`>>> DCSMM-BEGIN >>>` … `<<< DCSMM-END <<<`) ;
- elle crée une **sauvegarde horodatée** avant toute modification ;
- relancer l'installation met simplement à jour le bloc en place.

```powershell
dcsmm status          # installed / outdated / missing, par fichier
dcsmm install-lua     # installe ou met à jour
dcsmm uninstall-lua   # retire le bloc et Hooks/dcsmm.lua (garde la config)
```

Fichiers concernés :

| Cible | Traitement |
|---|---|
| `Scripts\Export.lua` | **fusion par marqueurs** (jamais écrasé) |
| `Scripts\Hooks\dcsmm.lua` | fichier propre au manager, copié/remplacé |
| `Config\dcsmm.cfg` | créé s'il manque ; jamais écrasé (contient l'IP/les ports) |

## Configuration

Une seule base de configuration pour les deux modes : variables `DCSMM_*`
(voir `README.md`) et `Saved Games\DCS\Config\dcsmm.cfg` côté DCS.

## Vérifier que tout est branché

1. Lancer le manager, ouvrir `http://<machine>:8080`.
2. Dans DCS, charger une mission.
3. L'en-tête doit passer à **« connecté »**, les unités apparaître sur la carte,
   et l'onglet **Session** se remplir.
4. En fin de mission, l'onglet **Débriefs** reçoit le `debrief.log`.
