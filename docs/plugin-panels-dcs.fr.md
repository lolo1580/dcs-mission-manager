# Plugin panels DCS Manager, sans vJoy ni pilote supplémentaire

Première version implémentée le 7 octobre 2026. Priorité : touches PZ55/PZ70 et trois LED du train ; LCD facultatif. Le code, les essais simulés et l’exécutable sont prêts. Les scripts de cette version ne sont pas encore installés dans Saved Games et le trim n’a pas été testé en cockpit.

## Fonctions de cette version

| Fonction | Liaison |
|---|---|
| Lecture des interrupteurs, boutons, volets et molettes | Lecteur USB HID existant dans l’application |
| Attributions cockpit existantes | DCS-BIOS, avec profils par avion |
| Trim longitudinal du F/A-18C | Nouveau plugin Lua, UDP local 127.0.0.1:7780 |
| LED du train gauche / nez / droite | Valeurs cockpit DCS-BIOS → sorties USB PZ55 |
| Diagnostic du plugin | Connexion, avion, impulsions acceptées et erreurs dans Panneaux |

Le profil Hornet comporte maintenant 20 attributions, trois sources LED et aucun LCD prédéfini. Le plugin complète DCS-BIOS : cette version ne remplace pas toute sa liaison. Les autres avions conservent leurs attributions existantes ; le trim ajouté est réservé au Hornet.

L’option vJoy a été retirée de l’interface et du code exécuté. Aucun pilote Windows n’a été installé. Les attributions restent dans DCS Manager : cette solution ne crée pas de colonnes de périphériques dans Options → Commandes de DCS.

## Cheminement technique

1. Vérification des fichiers de DCS installé, version 2.9.30.28738 : `Mods/aircraft/FA-18C/Input/FA-18C/joystick/default.lua`, `Cockpit/Scripts/devices.lua` et `command_defs.lua`. HOTAS=13, trim UP=3014, DOWN=3015, pression=1 et relâchement=0. Ces valeurs ne sont pas universelles entre avions ou versions.
2. Ajout de `dcs-lua/PanelCommands.lua`, chargé depuis `Scripts/DCSManager/PanelCommands.lua` par le bloc Export.lua géré. L’installateur l’ajoute comme quatrième fichier, avec sauvegardes et désinstallation.
3. Chaînage de `LuaExportBeforeNextFrame`, Start et Stop pour préserver les exports précédents, notamment DCS-BIOS. Listener local non bloquant ; aucun fichier de l’installation du jeu modifié.
4. Ajout du client Go `backend/internal/panelplugin`. Il vérifie la connexion et l’avion avant l’envoi, surveille les accusés de réception et ne renvoie jamais automatiquement une impulsion perdue.
5. Ajout au catalogue Hornet de `DCSM_PITCH_TRIM`, catégorie **DCS Manager plugin**, interface `variable_step`. Aucun contrôle BIOS ni source LED remplacé. Les commandes BIOS continuent leur chemin habituel : aucun double envoi.
6. Attribution PITCH TRIM : UP physique → valeur positive → 3014 ; DN → négative → 3015. `invert=true` compense le sens des bits HID existants. La magnitude ne règle pas l’intensité : chaque événement génère une impulsion.
7. Retrait de la passerelle vJoy, de son API et de son écran. Ajout de `/api/panels/plugin` pour le diagnostic et, après le signalement de cabrage, pour l’activation/arrêt du trim dans Paramètres → Panneaux.
8. Export du profil fourni et migration des profils locaux avec sauvegarde, en conservant les attributions personnalisées. Exécutable reconstruit : `dcsmanager-panels.exe` ; l’ancien `dcsmanager.exe` reste conservé.

## Protocole et relâchement

Suite au signalement de cabrage, le trim expérimental est désormais **désactivé à chaque démarrage**. Il faut activer sa case dédiée en plus de l’envoi ou du mode test. Le diagnostic disponible ne confirme pas la cause du cabrage : aucun envoi du nouveau trim n’apparaissait dans la dernière mission enregistrée consultée.

Couper cette case, fermer l’application, ou couper à la fois envoi et mode test envoie `CANCEL`, annule les demandes en attente et demande le relâchement de l’impulsion active. Cela ne remet pas le trim de l’avion à sa position initiale. Recharger une mission est nécessaire pour comparer avec un état de départ identique. La correction Lua doit être installée, puis DCS redémarré, pour reconnaître CANCEL.

Un échec de relâchement était auparavant seulement journalisé et l’impulsion était oubliée. Le plugin conserve maintenant le device/commande pour réessayer, signale ERR_RELEASE à l’application et refuse les nouvelles impulsions tant que ce relâchement échoue. Aucun succès en cockpit n’est déduit de ces essais simulés.

Messages versionnés : `DCSM1 <session hex32> <sequence> PING`, `TRIM UP` ou `TRIM DN`. Réponses : PONG, OK ou ERR avec l’avion courant. Aucun texte Lua ni identifiant de device fourni par un profil n’est exécuté.

Le ping établit la session chaque seconde. Après trois secondes sans ping, session et impulsions restantes sont annulées. Les séquences dupliquées sont rejetées. La file Lua est limitée à huit impulsions, chaque impulsion expire après une seconde ; au maximum seize datagrammes sont traités par frame. Le client limite aussi les demandes en attente.

Une pression dure au minimum 50 ms, puis est relâchée à la première frame disponible. Changement d’avion, arrêt de mission, changement de session et expiration de liaison annulent la file et tentent le relâchement du device pressé. Pendant une pause ou un gel sans callbacks DCS, le plugin ne peut pas exécuter le relâchement : il le fera au retour des frames ou au callback Stop. Couper envoi et mode test, ou la case trim dédiée, demande désormais l’annulation explicite au prochain callback ; l’absence d’accusé ne garantit pas sa réception.

**« Impulsion acceptée » signifie que l’appel Lua n’a pas levé d’erreur. Cela ne prouve pas que le trim a changé dans le simulateur.** L’accès HOTAS depuis Export.lua reste à valider en cockpit. Erreur de device visible dans le diagnostic ; accusé de réception absent signalé après deux secondes. Aucun repli silencieux vers une autre commande de trim.

## Installation et essai

### Installation en un clic

Dans `dcsmanager-panels.exe`, ouvrir **Paramètres → Installation DCS**, puis cliquer **Installer / mettre à jour les scripts**. Le dossier Saved Games détecté est affiché. Sans dossier détecté, le bouton est désactivé ; utiliser la configuration `DCSMANAGER_SAVED_GAMES` ou la commande ci-dessous avec un chemin explicite.

L’application refuse l’installation si le processus `DCS.exe` tourne, y compris au menu. Elle installe les scripts embarqués dans l’exécutable, fusionne le bloc Export.lua avec les autres exports, sauvegarde chaque fichier existant modifié et affiche les chemins des sauvegardes. Une configuration `Config/dcsmanager.cfg` existante est conservée ; elle est créée si absente. Refaire l’installation ne duplique pas le bloc. L’état affiché est relu sur disque, sans redémarrer l’application.

L’installation ne fournit pas DCS-BIOS : il reste nécessaire pour les autres touches et les LED. Il faut lancer DCS après l’installation. Les erreurs et les actions déjà effectuées sont affichées si une installation échoue en cours de route ; les sauvegardes permettent de restaurer ces fichiers.

Tests supplémentaires : installation API dans un dossier temporaire, conservation des exports et de la configuration personnalisée, sauvegarde exacte, deuxième installation identique, statut actualisé et refus si DCS tourne ou dossier absent. Essai navigateur avec réponses fictives : bouton, erreur, succès, sauvegarde et dossier absent. Aucun script réel du joueur n’a été installé pendant ces vérifications.

### Installation en ligne de commande

Fermer DCS et toute autre instance de DCS Manager avant la mise à jour. Depuis le dossier du projet, PowerShell :

```powershell
# Prévisualiser les fichiers et sauvegardes.
.\dcsmanager-panels.exe install-lua --saved-games "$env:USERPROFILE\Saved Games\DCS" --dry-run

# Installer les scripts avec sauvegardes.
.\dcsmanager-panels.exe install-lua --saved-games "$env:USERPROFILE\Saved Games\DCS"

# Démarrer la version préparée.
.\dcsmanager-panels.exe
```

Installation également disponible dans Réglages. DCS-BIOS doit rester installé et chargé dans Export.lua pour les autres touches, l’avion actif côté application et les LED. Les outils d’export chargés ensuite doivent aussi chaîner leurs callbacks.

1. Charger une mission solo avec le F/A-18C du joueur. Vérifier **plugin connecté**, **DCS-BIOS connecté** et `FA-18C_hornet` dans Panneaux.
2. Vérifier PITCH TRIM → `DCSM_PITCH_TRIM`, inversé. Activer la case **Activer le trim expérimental du panel**, puis l’envoi ou le mode test au moment de l’essai.
3. Tourner vers UP puis DN. Vérifier sens, mouvement dans DCS, compteur et absence de trim continu après l’arrêt de la roue.
4. Essayer plusieurs crans rapides, quitter puis revenir dans une mission : aucun ancien cran ne doit être rejoué. Tester la déconnexion de l’application.
5. Activer les sorties LED. Vérifier gauche, nez et droite avec `FLP_LG_LEFT_GEAR_LT`, `FLP_LG_NOSE_GEAR_LT`, `FLP_LG_RIGHT_GEAR_LT`. Elles représentent les témoins cockpit, pas la position du levier. Rouge en transit et dépendance électrique restent à valider/compléter, sans couleur inventée.

## Vérifications et limites

- Tests Go : client UDP, sens du trim via profil, isolement du catalogue, conformité des profils, installation et désinstallation. Suite complète et analyse statique.
- Syntaxe des trois scripts Lua contrôlée en Lua 5.1. Test avec devices/sockets simulés : deux sens, pression/relâchement, mauvais avion, session, doublons, file, expiration et arrêt.
- Interface compilée et vérifiée dans un navigateur avec API fictive ; aucun envoi aux panels ou au cockpit.
- Pas de validation physique DCS ni multijoueur pendant ce développement. Le LCD n’est pas une condition de cette version.

Reproduction Lua : `node tools/check-lua.mjs --strict` avec luaparse disponible, puis `lua tools/test-panel-plugin.lua` ou Fengari. La vérification syntaxique cible Lua 5.1 ; Fengari ne remplace pas le runtime DCS.

## Retour arrière

Pour abandonner seulement le trim expérimental, désactiver l’envoi et retirer son attribution dans l’éditeur ; touches BIOS et LED restent disponibles. Sauvegarde locale avant ajout : `data/mappings.before-profile-fixes-20261007-204527-939.json`. Fermer l’application avant de la recopier sur `data/mappings.json`.

La commande suivante retire **tous les scripts gérés DCS Manager**, dont télémétrie, plugin et hook ; elle conserve la configuration utilisateur et les blocs des autres outils dans Export.lua. À utiliser uniquement pour désinstaller cette intégration :

```powershell
.\dcsmanager-panels.exe uninstall-lua --saved-games "$env:USERPROFILE\Saved Games\DCS"
```

Pour revenir à la version Lua précédente, restaurer les sauvegardes indiquées par l’installateur. L’expérience vJoy abandonnée reste consultable dans l’historique Git ; aucune fonctionnalité vJoy n’est active.
