# F/A-18C : intégration des deux panels Logitech

Mise à jour : 7 octobre 2026. Matériel confirmé par les photos de l’utilisateur et les fiches Logitech :
- [Switch Panel, référence 945-000030](https://www.logitechg.com/en-us/shop/p/flight-simulator-switch-panel.945-000030) : PZ55.
- [Multi Panel, référence 945-000028](https://www.logitechg.com/en-us/shop/p/flight-simulator-autopilot-multipanel.945-000028) : PZ70.

Les compatibilités annoncées par Logitech concernent les simulateurs cités sur ces pages. Ici, DCS est pris en charge par le lecteur HID et DCS-BIOS de cette application.

## Diagnostic du journal fourni

Le journal reçu montre `aircraft=""` et `commands=false` pour tous les événements. Plusieurs interrupteurs, positions moteur, boutons AP, les volets et les deux directions du trim sont détectés. La détection USB ne signifie pas que ces commandes sont attribuées ou envoyées. Le profil Hornet ne comportait auparavant que cinq attributions.

L’extrait ne suffit pas à juger les touches absentes du journal, ni à confirmer un rebond des boutons APR/REV. Le premier rapport USB sert de référence : un interrupteur laissé dans sa position initiale ne produit pas d’action avant d’être déplacé.

## Attributions ajoutées et conservées

Le profil `FA-18C_hornet` contient désormais 20 attributions (19 commandes physiques distinctes, la molette ayant deux contextes), trois sources LED de train et aucune valeur LCD prédéfinie. Le trim utilise le [nouveau plugin DCS Manager](plugin-panels-dcs.fr.md), sans pilote supplémentaire ; les autres commandes et les LED restent DCS-BIOS.

| Panel | Commande physique | Fonction Hornet et comportement |
|---|---|---|
| PZ55 | BAT | `BATTERY_SW` : ON=0, OFF=1, jamais ORIDE |
| PZ55 | DE-ICE | `ENG_ANTIICE_SW` : ON=0, OFF=1, jamais TEST |
| PZ55 | PITOT HEAT | `PITOT_HEAT_SW` : ON=1, AUTO=0 |
| PZ55 | LIGHTS PANEL | `INST_PNL_DIMMER` : intensité maximale=65535, éteint=0 ; instruments uniquement |
| PZ55 | LIGHTS NAV | `POSITION_DIMMER` : intensité maximale=65535, éteint=0 ; le master extérieur du cockpit reste à positionner |
| PZ55 | LIGHTS LANDING | `LDG_TAXI_SW` : ON=1, OFF=0 ; lampe commune landing/taxi |
| PZ55 | Moteur L | `ENGINE_CRANK_SW` : LEFT=0, maintenu tant que le sélecteur reste sur L |
| PZ55 | Moteur R | `ENGINE_CRANK_SW` : RIGHT=2, maintenu tant que le sélecteur reste sur R |
| PZ55 | Moteur OFF et BOTH | `ENGINE_CRANK_SW` : centre/OFF=1, arrête le crank ; n’arrête pas les moteurs |
| PZ55 | GEAR UP / DOWN | `GEAR_LEVER` : haut=0, bas=1 |
| PZ70 | FLAPS UP / DOWN | `FLAP_SW DEC` / `INC`, passe au cran voisin ; le retour physique au centre ne renvoie pas de commande |
| PZ70 | AP | `UFC_AP` : pression=1, relâchement=0 ; ouvre le menu AP, ne sélectionne pas un mode automatiquement |
| PZ70 | IAS | `THROTTLE_ATC_SW` : pression=1, relâchement=0 ; bouton ATC du Hornet |
| PZ70 | REV | `STICK_PADDLE_SW` : pression=1, relâchement=0 ; désengagement AP/NWS, pas un mode reverse |
| PZ70 | Molette +/− avec sélecteur HDG | `LEFT_DDI_HDG_SW` : + envoie 2, − envoie 0, puis centre=1 après 50 ms |
| PZ70 | Molette +/− avec sélecteur CRS | `LEFT_DDI_CRS_SW` : même impulsion et relâchement |

Les lampes PZ55 L/N/R suivent `FLP_LG_LEFT_GEAR_LT`, `FLP_LG_NOSE_GEAR_LT`, `FLP_LG_RIGHT_GEAR_LT` en vert. Aucun rouge de transit ni voyant AP PZ70 n’est ajouté sans source appropriée. Le voyant d’un bouton pressé ne constitue pas une preuve d’engagement de son mode.

## Commandes restantes, avec le motif

| Commande physique | État et travail restant |
|---|---|
| ALT (générateur), AVIONICS MASTER, FUEL PUMP, COWL | Pas d’attribution automatique : organes du jet différents. Définir les fonctions souhaitées puis vérifier les positions ; deux générateurs nécessitent plusieurs commandes pour un seul interrupteur. |
| LIGHTS BEACON | Pas d’équivalent distinct confirmé dans le catalogue Hornet. Décider d’une réaffectation explicite. |
| LIGHTS STROBE | `STROBE_SW` existe, mais ses trois positions ne sont pas nommées dans le catalogue local. Vérifier le sens réel en cockpit avant de choisir ON/OFF. |
| LIGHTS TAXI | Le Hornet utilise la même commande que LANDING. Laisser ce bouton libre évite que deux interrupteurs indépendants s’annulent. |
| Moteur START | Le jet nécessite APU puis crank gauche/droite. Pas de démarrage à magnétos équivalent ; définir une séquence explicite avant attribution. |
| PITCH TRIM | Attribution ajoutée : `DCSM_PITCH_TRIM`, plugin Lua, HOTAS 13 / UP 3014 / DN 3015, avec relâchement. Inversion activée pour suivre UP/DN physiques. Validation en cockpit encore nécessaire ; ne pas utiliser `RUD_TRIM` ni `SAI_MAN_PITCH_ADJ`. |
| AUTO THROTTLE OFF/ARM | Interrupteur maintenu alors que le Hornet exporte un bouton ATC momentané. Le bouton IAS fournit une commande utilisable. Pour OFF/ARM, ajouter impulsion et retour d’état ATC afin d’éviter une bascule désynchronisée. |
| HDG, NAV, ALT, VS, APR (boutons) | Pas d’équivalence directe universelle. Les boutons option UFC dépendent du menu affiché ; il faut gérer le contexte AP et vérifier l’effet réel avant attribution. |
| Sélecteur ALT / VS / IAS / HDG / CRS | Déjà utilisé comme contexte de molette et d’affichage. Il n’engage aucun mode AP. La molette n’agit actuellement qu’en HDG/CRS. |
| LCD PZ70 | Pilote matériel disponible, mais aucune consigne sélectionnée Hornet correctement exportée/configurée ici. Ne pas afficher des valeurs brutes de jauge comme des pieds ou des nœuds. `SBY_COMPASS_HDG` serait un cap réel, pas le cap sélectionné. Ajouter une source validée avec conversion et libellé explicite. |

## Cheminement de la correction

1. Lecture du journal, identification du défaut de liaison avion/envoi et du profil incomplet.
2. Confirmation du matériel à partir des photos et des deux fiches Logitech.
3. Vérification des identifiants/interfaces/positions dans `Saved Games/DCS/Scripts/DCS-BIOS/doc/json/FA-18C_hornet.json`, complétée par le module Lua Hornet local.
4. Extension du profil intégré et de son JSON distribuable ; conservation des attributions personnalisées lors de la migration locale.
5. Ajout de `pulse_reset` pour les molettes envoyant un état momentané : commande directionnelle, pause de 50 ms, puis centre. Correction du choix des positions personnalisées pour suivre le sens de rotation plutôt que le bit « actif ».
6. Ajout du réglage de relâchement dans l’éditeur et d’un message visible lorsqu’aucun avion n’est détecté. Import/export utilisent le même champ JSON.
7. Tests de sens, inversion, contexte HDG/CRS, relâchement, boutons momentané et positions moteur ; validation des sept profils contre les catalogues installés, construction de l’interface et de l’exécutable.

La temporisation bloque brièvement le traitement de l’événement courant (50 ms par cran). Sa durée et le sens réel des réglages HDG/CRS doivent être validés en cockpit ; l’option d’inversion permet de corriger le sens sans modifier le code.

## Vérification en cockpit et rollback

Fermer l’ancienne application puis lancer `dcsmanager-panels.exe`. Charger une mission F/A-18C, entrer dans le cockpit et vérifier que DCS-BIOS indique `FA-18C_hornet`. Activer l’envoi des commandes et les sorties LED/LCD dans l’application. Un choix manuel de profil dans l’éditeur ne remplace pas l’avion détecté pour l’envoi réel.

Tester d’abord batterie, pitot, volets, pression/relâchement AP/IAS/REV, puis la molette HDG/CRS sur un affichage approprié. Vérifier que le changement s’arrête à chaque cran. Tester les commandes de crank uniquement dans une procédure de démarrage appropriée ; OFF/BOTH les recentrent. Comparer les trois voyants au cockpit.

Migration locale sauvegardée avant écriture : `data/mappings.before-profile-fixes-20261007-191502-721.json`. Pour restaurer les attributions précédentes, fermer l’application puis recopier cette sauvegarde sur `data/mappings.json`. L’ancien `dcsmanager.exe` est conservé.

La vérification automatique couvre les commandes générées et leur conformité aux catalogues. Elle ne remplace pas un essai avec les deux panels physiques et DCS en vol.
