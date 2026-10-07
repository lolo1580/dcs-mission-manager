# Bibliothèque de profils PZ55 / PZ70

Mise à jour : 7 octobre 2026. Les sept profils sont fournis dans le code, exportés ici pour import individuel et enregistrés dans les données locales de cette installation. Commandes, interfaces et sources vérifiées contre le DCS-BIOS installé ; aucun essai physique dans DCS pendant cette correction.

| Avion | Entrées | LED | LCD | Particularités |
|---|---:|---:|---:|---|
| [F/A-18C](FA-18C_hornet.json) | 20 | 3 | 0 | Batterie, éclairages, antigivrage, crank, volets, AP/ATC/paddle, molette HDG/CRS, trim via plugin DCS Manager |
| [F-16C](F-16C_50.json) | 4 | 3 | 0 | Batterie BATT/OFF ; train et trim |
| [F-5E-3](F-5E-3.json) | 8 | 3 | 2 | Volets THUMB SW/FULL ; molette HDG/CRS |
| [Mirage 2000C](M-2000C.json) | 5 | 3 | 2 | Batterie, pompe gauche, train et AP |
| [A-10C](A-10C.json) | 8 | 3 | 2 | Batterie, Pitot, train, volets progressifs et molette HDG/CRS |
| [JF-17](JF-17.json) | 6 | 1 | 0 | Batterie, pompe, train et volets ; voyant rouge du levier |
| [AV-8B](AV8BNA.json) | 6 | 3 | 0 | Batterie ON/OFF, pompe gauche, train ; modes de volets CRUISE/AUTO/STOL ; LED vert/jaune |

Les profils restent partiels. Une absence d’auto-throttle ou d’un mode LCD ne doit pas être compensée en affectant une commande de cockpit sans rapport. Le profil A-10C concerne l’identifiant `A-10C` ; il n’est pas automatiquement appliqué à un autre identifiant tel que `A-10C_2`.

## Corrections des profils existants

### Hornet

Le profil a été étendu à 20 attributions. Voir [la matrice complète et les fonctions restantes](../../docs/integration-fa18-panels.fr.md), notamment les réaffectations AP/IAS/REV et le comportement du sélecteur moteur. La molette HDG/CRS envoie une impulsion directionnelle puis un recentrage après 50 ms. Le trim `DCSM_PITCH_TRIM` nécessite le [plugin Lua DCS Manager](../../docs/plugin-panels-dcs.fr.md) et reste à valider en cockpit.

`MASTER_BAT` transmet désormais `BATTERY_SW 0` à l’activation (ON) et `BATTERY_SW 1` à la coupure (OFF). Il ne sélectionne plus ORIDE.

FLAPS UP transmet `FLAP_SW DEC` et FLAPS DOWN `FLAP_SW INC`. Chaque impulsion passe à la position précédente ou suivante : AUTO → HALF → FULL. Le retour au centre n’envoie rien.

### F-16

`MASTER_BAT` ne commande plus le carburant principal : activation `MAIN_PWR_SW 1` (BATT), coupure `MAIN_PWR_SW 2` (OFF). Le mode MAIN PWR, valeur 0, reste à sélectionner dans le cockpit. Il n’a pas été attribué à un deuxième interrupteur pour éviter deux contrôles maintenus concurrents sur le même sélecteur.

Le train UP/DOWN est ajouté. Le trim conserve le pas déclaré par DCS-BIOS, 3200, et les trois LED vertes sont conservées.

### F-5

FLAPS UP sélectionne `FLAPS 1`, soit THUMB SW ; FLAPS DOWN sélectionne `FLAPS 2`, soit FULL. EMER UP n’est plus commandé par ces touches. THUMB SW renvoie le contrôle au système du commutateur de pouce : ce n’est pas une garantie de rentrée immédiate des volets. Le sélecteur `A_FLAPS` doit être dans la position souhaitée dans le cockpit.

La molette est ajoutée pour HDG et CRS ; les lignes LCD correspondantes sont conservées. Le trim d’attitude de secours n’a pas été confondu avec le trim de profondeur de l’avion.

### Mirage

Les attributions compatibles ont été conservées. Le voyant rouge est celui du levier ; les deux voyants AP/ALT et les deux sources HSI sont inchangés.

## Nouveaux profils

### A-10C

Le train utilise la polarité propre à cet avion : UP = 1, DOWN = 0. Les volets utilisent UP = INC, DOWN = DEC, car la commande est ordonnée DN/MVR/UP. Les voyants sont les trois lampes vertes « safe ». Le LCD affiche le repère HDG et la référence CRS, pas le cap instantané de l’avion.

### JF-17

Profil de base batterie/pompe/train/volets. Le voyant supérieur du PZ55 reproduit la lampe rouge du levier, et non le verrouillage du train avant. Pas de LCD configuré sans référence numérique sélectionnée clairement identifiée.

### AV-8B

Batterie : ON = 2, OFF = 1, sans passer par ALERT. Pompe gauche : NORM = 2, OFF = 1, sans passer par OPEN. La pompe droite n’est pas commandée par le même interrupteur car les attributions restent à une commande par touche.

FLAPS UP/DOWN change le **mode** CRUISE/AUTO/STOL par pas, pas l’angle direct des volets. Leur alimentation doit être configurée correctement dans le cockpit. Les voyants reproduisent les sources NOSE/LEFT/RIGHT : jaune si la lampe de transition correspondante est active, sinon vert si la lampe « down » est active, sinon éteint. La source MAIN_GEAR n’est pas fusionnée avec les autres dans ce profil.

## Application et sauvegarde

Les nouveaux profils sont créés automatiquement seulement sur une nouvelle installation sans fichier de profils. Les installations existantes conservent leur fichier. Pour importer un profil individuellement : Paramètres → Panneaux → choisir l’avion → Importer → choisir son JSON → Appliquer l’import. Cela remplace le profil cible ; exporter sa version précédente d’abord.

La mise à jour de cette installation a sauvegardé l’ancien fichier ici :

`data/mappings.before-profile-fixes-20261007-172801-536.json`

Le script `tools/update-panel-profiles.ps1` ajoute les profils absents et les attributions manquantes, corrige les associations connues et conserve les commandes personnalisées différentes. Les sorties existantes sont conservées. Une sauvegarde est créée avant chaque écriture. Fermer le gestionnaire avant de l’exécuter.

```powershell
.\tools\verify-panel-profiles.ps1
```

L’exécutable `dcsmanager-panels.exe` est reconstruit : utiliser cette version pour les champs `state_on`/`state_off` et les pas INC/DEC. Un ancien exécutable ignore ces champs et peut reproduire les mauvaises positions.

## Retour arrière

La commande suivante remplace les profils actuels par la sauvegarde : elle supprime donc leurs modifications postérieures. Fermer le gestionnaire et conserver un export récent avant de l’utiliser.

```powershell
Copy-Item -LiteralPath '.\data\mappings.before-profile-fixes-20261007-172801-536.json' -Destination '.\data\mappings.json' -Force
```

Le détail du cheminement est dans [le journal d’intégration](../../docs/cheminement-integration-touches.fr.md).
