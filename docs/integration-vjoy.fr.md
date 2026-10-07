# Panels visibles dans DCS : passerelle vJoy

**Historique : option abandonnée le 7 octobre 2026 à la demande de l’utilisateur. Son code et son interface ont été retirés. Ne pas suivre l’installation décrite ci-dessous ; utiliser le [plugin DCS Manager](plugin-panels-dcs.fr.md).**

État au 7 octobre 2026 : première intégration implémentée, testée automatiquement et dans une interface à données fictives. Validation avec le pilote vJoy et les panels physiques dans DCS encore nécessaire. Aucun pilote n’a été installé pendant cette intervention.

## Utilisation

1. Installer la version Windows x64 signée de vJoy depuis [les publications BrunnerInnovation](https://github.com/BrunnerInnovation/vJoy/releases), auxquelles renvoie le projet njz3 pour Windows 10/11. Le pilote et sa DLL doivent être compatibles avec la version de Windows utilisée ; consulter les notes de la publication choisie.
2. Dans **Configure vJoy**, créer deux périphériques distincts, par défaut ID **1** et **2**, avec **64 boutons chacun**. Cette intégration utilise uniquement les boutons ; aucun axe ni retour de force ne lui est nécessaire. Garder une configuration acceptée par l’outil vJoy.
3. Vérifier la présence des deux contrôleurs dans Windows avec `joy.cpl`.
4. Fermer l’ancienne instance du gestionnaire et lancer `dcsmanager-panels.exe`.
5. Ouvrir **Paramètres → Panneaux → Attribuer les touches directement dans DCS**, vérifier les ID, puis cliquer **Activer vJoy**. Les ID peuvent être changés lorsque la passerelle est désactivée. Ils doivent être distincts et compris entre 1 et 16.
6. Vérifier **vJoy prêt**. Déplacer chaque commande au moins une fois après activation. Les positions initiales ne sont pas automatiquement envoyées.
7. Dans DCS, ouvrir **Options → Commandes**, choisir le F/A-18C Sim et attribuer les boutons dans les colonnes des périphériques vJoy. Si DCS était lancé avant la création des périphériques, le relancer si nécessaire.
8. Exporter les profils de commandes depuis DCS après configuration. Les attributions avion sont désormais conservées par DCS.

Les périphériques peuvent tous deux porter un nom vJoy : l’application ne les renomme pas « PZ55 » et « PZ70 » dans Windows. Actionner une touche et utiliser la numérotation ci-dessous pour identifier la bonne colonne. Le changement d’ID ou de configuration du pilote peut changer l’identité du périphérique vue par DCS ; conserver ses ID après les attributions.

Activation volontaire à chaque lancement ; aucun changement n’est écrit dans `data/mappings.json`. Le bouton **Revenir aux entrées DCS-BIOS** relâche les boutons et libère les périphériques. Les anciens profils d’entrée restent disponibles, mais leur envoi et le test direct restent désactivés jusqu’à réactivation explicite.

## PZ55 : vJoy ID 1 par défaut, 33 boutons utilisés

| Interrupteur | Bouton ON | Bouton OFF |
|---|---:|---:|
| BAT | 1 | 2 |
| ALT (générateur) | 3 | 4 |
| AVIONICS MASTER | 5 | 6 |
| FUEL PUMP | 7 | 8 |
| DE-ICE | 9 | 10 |
| PITOT HEAT | 11 | 12 |
| COWL | 13 | 14 |
| LIGHTS PANEL | 15 | 16 |
| LIGHTS BEACON | 17 | 18 |
| LIGHTS NAV | 19 | 20 |
| LIGHTS STROBE | 21 | 22 |
| LIGHTS TAXI | 23 | 24 |
| LIGHTS LANDING | 25 | 26 |

ON et OFF sont deux boutons mutuellement exclusifs, maintenus tant que l’interrupteur reste dans cette position, après sa première manipulation. Le bouton précédent est relâché avant le nouveau. Pour le COWL, ON/OFF sont les états électriques lus ; vérifier physiquement quel état correspond à OPEN/CLOSE avant l’attribution.

| Position | Bouton |
|---|---:|
| Moteur OFF | 27 |
| Moteur R | 28 |
| Moteur L | 29 |
| Moteur BOTH | 30 |
| Moteur START | 31 |
| Train UP | 32 |
| Train DOWN | 33 |

Chaque position moteur/train possède son propre bouton. Sa désactivation relâche ce bouton. START suit le ressort du matériel ; il ne lance aucune séquence automatique du jet.

## PZ70 : vJoy ID 2 par défaut, 29 boutons utilisés

| Commande | Bouton |
|---|---:|
| Sélecteur ALT / VS / IAS / HDG / CRS | 1 / 2 / 3 / 4 / 5 |
| AP / HDG / NAV / IAS / ALT / VS / APR / REV | 6 / 7 / 8 / 9 / 10 / 11 / 12 / 13 |
| Auto Throttle ARM / OFF | 14 / 15 |
| Volets UP / DOWN | 16 / 17 |
| Trim UP / DN | 18 / 19 |
| Molette + / − en ALT | 20 / 21 |
| Molette + / − en VS | 22 / 23 |
| Molette + / − en IAS | 24 / 25 |
| Molette + / − en HDG | 26 / 27 |
| Molette + / − en CRS | 28 / 29 |

Les boutons AP suivent pression et relâchement. Les volets suivent leur direction maintenue et leur retour au centre. Le sélecteur est exposé comme cinq boutons de position ; il sélectionne également la paire de boutons de la molette. Il n’est généralement pas nécessaire de lui attribuer une action avion.

Chaque cran de trim ou de molette produit une pression de **50 ms**, suivie d’un relâchement et d’au moins **10 ms** avant la prochaine impulsion. Les mouvements sont mis en file ; les rotations rapides peuvent donc être retransmises avec un retard. Un cran ne correspond pas à un axe analogique. Le sens des boutons UP/DN et +/− doit être vérifié physiquement ; attribuer le bouton opposé dans DCS si nécessaire.

Pour le trim Hornet, attribuer 18 et 19 aux commandes de trim longitudinal proposées par DCS. Les commandes momentané/maintenu/off disponibles dépendent de l’avion : vJoy expose les entrées, il ne crée pas de fonctions avion absentes de DCS. L’interrupteur Auto Throttle OFF/ARM ne devient donc pas automatiquement compatible avec un avion qui propose seulement une commande de bascule ; sélectionner une attribution appropriée et tester son comportement.

## LED et LCD

Les boutons vJoy ne remontent pas l’état du cockpit. Les sorties LED et LCD restent configurées dans notre application par avion et alimentées par DCS-BIOS. Pour elles, DCS-BIOS doit détecter l’avion, et les sorties doivent être activées dans l’interface. Le mode vJoy n’ajoute aucune source LCD Hornet ni LED de mode AP manquante.

Les entrées vJoy fonctionnent sans avion DCS-BIOS détecté et sans preset d’entrée dans l’application. Lorsque vJoy est sélectionné, le chemin d’envoi des entrées DCS-BIOS est contourné, y compris le test direct. L’API refuse aussi leur réarmement dans ce mode. La lecture des données et la commande des sorties continuent.

## Dépannage et retour arrière

- **DLL introuvable** : installer vJoy x64. Le chargeur recherche `vJoy/x64/vJoyInterface.dll`, puis `vJoy/vJoyInterface.dll` dans les dossiers Program Files connus. Pour une installation différente, définir un chemin absolu avant lancement :

```powershell
$env:DCSMANAGER_VJOY_DLL = 'C:\chemin\vers\x64\vJoyInterface.dll'
.\dcsmanager-panels.exe
```

- **Périphérique indisponible** : vérifier les ID dans Configure vJoy et fermer les autres logiciels qui réservent ces mêmes ID. La passerelle ne prend pas un périphérique déjà possédé par un autre logiciel.
- **Boutons insuffisants** : configurer 64 boutons sur chacun. Le minimum vérifié est 33 pour PZ55 et 29 pour PZ70.
- **vJoy arrêté / erreur d’écriture / file saturée** : les boutons sont relâchés au mieux et la file précédente est invalidée. L’application conserve le choix de routage vJoy et ne repasse pas automatiquement aux anciennes commandes BIOS. Désactiver puis réactiver après correction du problème.
- **Panel débranché** : le périphérique virtuel correspondant est remis au repos ; les événements en attente de ce panel sont annulés. Après reconnexion, manipuler à nouveau les commandes. Un exemplaire physique par modèle est pris en charge ; un second exemplaire n’obtient pas sa propre identité virtuelle.
- **Retour à l’ancien fonctionnement** : cliquer Revenir aux entrées DCS-BIOS, puis réactiver explicitement l’envoi si souhaité. Désactiver les anciennes attributions vJoy dans DCS si un autre logiciel alimente ces périphériques. Aucun profil existant n’est supprimé.

La fermeture normale relâche les boutons et libère les deux ID. Un arrêt forcé de Windows ou de l’application n’offre pas la même garantie : contrôler les états dans `joy.cpl` avant de reprendre un vol.

## Cheminement et vérifications

1. Vérification des fonctions officielles de l’[interface vJoy](https://github.com/shauleiz/vJoy/blob/master/inc/vjoyinterface.h) : acquisition, capacité, écriture de bouton, remise au repos et libération.
2. Ajout du package `backend/internal/virtualpanel` : table de boutons stable, traduction indépendante de l’avion, file asynchrone bornée à 256 événements et gestion des erreurs. Un worker unique traite les deux panels ; une impulsion peut retarder brièvement un événement de l’autre panel.
3. Chargement dynamique de la DLL Windows x64 depuis un chemin absolu, sans DLL distribuée avec l’application. Aucun pilote personnalisé, accès aux périphériques des autres logiciels ni appel de remise au repos globale de tous les vJoy.
4. Branchement dans le lecteur de panels avant le mapping BIOS ; conservation de l’actualisation des sorties et du contexte du sélecteur.
5. Ajout de `GET/POST /api/panels/virtual`, des champs ID et du bouton d’activation. L’état retourne aussi la table de boutons et la taille de file en attente. `enabled` signifie routage choisi, `ready` signifie pilote réservé et opérationnel au dernier contrôle.
6. Tests : couverture de toutes les commandes du décodeur, stabilité des numéros, ON/OFF, boutons momentané, trim et contexte de molette, répétition avec séparation des impulsions, abandon après déconnexion, rollback si le deuxième ID est occupé, conservation du routage exclusif en panne et conservation des sorties lors du changement de mode.
7. Vérification navigateur avec données fictives : activation, tableau des boutons PZ70/PZ55 et retour au mode BIOS. Script reproductible : `frontend/scripts/panel-virtual-ui-review.cjs` après le fixture existant. Les tests navigateur n’envoient aucune commande aux panels réels.

À valider sur le matériel : chargement de la DLL/pilote installé, visibilité des deux colonnes dans DCS, tous les boutons dans `joy.cpl`, sens du trim, déconnexion/reconnexion et rotations rapides. Ensuite décider si l’on souhaite mémoriser le mode/les ID, ajouter une synchronisation initiale facultative ou exposer des axes. Ces fonctions ne sont pas encore implémentées.
