# Intégration PZ55 / PZ70 — reprise et validation

Mise à jour : 6 octobre 2026. Ce document décrit l’état du code après la reprise de l’intégration. Les essais logiciels ne remplacent pas les essais dans DCS sur les panneaux réels.

## Ce qui était déjà intégré au début de cette reprise

Le projet contenait déjà ces ajouts ; ils ont été conservés :

- profils par avion avec commandes, voyants et affectations LCD ;
- lecture du sélecteur ALT / VS / IAS / HDG / CRS ;
- sens des encodeurs et pas proposé par les métadonnées DCS-BIOS ;
- mises à jour des sorties après chaque trame DCS-BIOS ;
- commutateur des sorties séparé du commutateur d’envoi des commandes ;
- rapport PZ70 commun aux deux lignes LCD et aux huit voyants ;
- formulaire LCD, aperçu numérique et test des chiffres sur le matériel ;
- prévention du double envoi en mode test ;
- cache des rapports réinitialisé à la reconnexion et publié après succès USB.

L’attribution de ces modifications à un assistant particulier n’est pas vérifiable à partir du code seul.

## Corrections apportées pendant cette reprise

| Correction | Résultat |
|---|---|
| Premier rapport USB | Sert de référence initiale ; les positions déjà actives ne sont plus envoyées comme des actions de l’utilisateur |
| AUTO THROTTLE | Identifié comme un interrupteur maintenu ON/OFF |
| FLAPS avec retour au centre | Le relâchement ne renvoie plus une commande inverse pour `set_state`, `fixed_step` ou `variable_step` ; `action` conserve son relâchement |
| Sélecteurs moteur, train et mode | La désactivation de l’ancienne position ne déclenche plus une deuxième commande de position |
| Boutons avec commandes incrémentales | Le relâchement n’annule plus le pas envoyé à la pression |
| Inversion des encodeurs | Inverse réellement le sens de la commande du trim ou de la molette |
| Configuration d’une nouvelle affectation | Case d’inversion ajoutée au formulaire |
| Changement de profil dans l’interface | Affectations précédentes effacées pendant le chargement ; sauvegarde bloquée pendant cette transition ; réponse de sauvegarde tardive ignorée si l’avion a changé |
| Mode LCD non affecté ou source disparue | Rapport vide construit, au lieu de conserver les anciens chiffres ou voyants |
| Export LCD numérique | Exports textuels et mots incomplets identifiés comme inutilisables |
| Valeur LCD hors plage | Ligne laissée vide au lieu de montrer une valeur tronquée ou un nombre négatif transformé en positif |
| Aperçu de la ligne inférieure | Cinq positions, conformément aux cinq octets de cette ligne dans le rapport |
| Changement d’avion DCS-BIOS | Anciennes valeurs des adresses cockpit supprimées ; exports de la nouvelle trame conservés |
| Lecture du sélecteur par le pilote de sorties | Accès au callback protégé contre une modification concurrente |

## Comportement physique attendu

### PZ70

| Contrôle | Usage du mapping |
|---|---|
| PITCH TRIM | Une impulsion positive ou négative par cran ; l’inversion permet d’adapter le sens |
| AUTO THROTTLE | Une valeur pour ON et une valeur pour OFF, selon la commande de l’avion |
| FLAPS UP / DOWN | Commande au déplacement ; retour au centre ignoré pour les interfaces de position ou de pas, relâchement transmis pour `action` |
| Boutons AP/HDG/NAV/IAS/ALT/VS/APR/REV | Pression et relâchement pour `action` ; un seul pas à la pression pour les interfaces incrémentales |
| Sélecteur ALT/VS/IAS/HDG/CRS | Sélection du mode d’affichage LCD ; une affectation d’entrée éventuelle n’agit que sur la position qui devient active |
| Molette de réglage | Deux sens distincts ; commande configurable par mode ALT/VS/IAS/HDG/CRS, avec affectation générale de secours |

Le sélecteur LCD choisit aussi la commande affectée à la molette. Une affectation propre au mode prend la priorité sur l’affectation générale ; une seule commande est envoyée par impulsion.

## Fonctions complétées

- Édition des commandes, LED et lignes LCD existantes, avec annulation et contrôle des doublons.
- Affectation de la molette PZ70 par mode, sans modifier les anciens profils sans contexte.
- Règles LED ordonnées : source numérique, index d’export, comparaison, valeur et couleur. La première règle satisfaite décide ; sans résultat le voyant est éteint.
- Export JSON du profil sélectionné et import avec aperçu des quantités et de l’avion cible avant remplacement.
- Vérification périodique des sorties : une connexion DCS-BIOS devenue inactive efface LED/LCD lorsque les sorties sont activées. Une modification du profil est appliquée sans attendre une nouvelle trame.

Le [guide des profils](profils-panels.fr.md) explique la configuration et la sauvegarde.

### PZ55

Les interrupteurs maintenus conservent les transitions ON/OFF. Les positions du sélecteur moteur et du levier de train n’envoient une commande que lorsqu’elles deviennent actives. Les trois voyants restent des sorties indépendantes des commandes de train.

## Configuration pratique

1. Recompiler puis lancer le gestionnaire.
2. Ouvrir Paramètres → Panneaux et choisir le profil de l’avion.
3. Garder l’envoi des commandes désactivé pendant les premiers essais.
4. Vérifier que les mouvements apparaissent dans le journal des entrées.
5. Ajouter les commandes utiles en choisissant l’interface réellement acceptée par l’avion.
6. Pour le trim, choisir `variable_step` si disponible et régler l’inversion selon le sens voulu.
7. Pour les volets, distinguer une commande de bouton `action` d’une commande de position `set_state`. Le retour au centre n’est pas une position de volets.
8. Configurer les voyants depuis les exports correspondant à l’état réel du cockpit.
9. Configurer chaque ligne LCD par mode, avec la source numérique, l’index d’export, l’échelle, le décalage et l’unité descriptive.
10. Activer les sorties pour observer LED/LCD sans envoyer de commandes, puis activer l’envoi lorsque les affectations ont été vérifiées.

Le test matériel LCD écrit directement un rapport de chiffres et n’active pas les commandes. Faire ce test avec les sorties automatiques désactivées : sinon une trame DCS-BIOS peut rétablir immédiatement l’affichage configuré. Ce test remet les voyants du rapport à zéro.

Ne pas tester simultanément le même matériel avec un autre gestionnaire de panneaux.

## Limites qui restent à connaître

- Les commandes exactes et conversions LCD dépendent de l’avion ; les profils intégrés sont partiels.
- `set_state` permet des positions explicites ON/OFF ; sans ces valeurs, il conserve le calcul zéro/maximum. Les positions doivent correspondre au catalogue de l’avion.
- Une affectation par modèle/contrôle : deux exemplaires identiques ne disposent pas encore de profils distincts.
- Le LCD prend en charge les exports numériques ; les chaînes de caractères ne sont pas encore converties.
- Désactiver les sorties arrête leur synchronisation ; cela ne constitue pas une commande explicite d’extinction matérielle.
- Aucun essai physique USB ou vol DCS n’a été effectué pendant cette reprise.

## Vérifications logicielles

Les tests ajoutés couvrent :

- première lecture sans commandes parasites ;
- classification ON/OFF de l’auto-throttle ;
- volets au centre et relâchement `action` ;
- inversion du trim dans les deux sens ;
- désactivation de l’ancienne position du train ;
- effacement du LCD lors d’un changement de mode ou retrait du profil ;
- rejet des exports textuels et des mots incomplets ;
- suppression des anciennes valeurs DCS-BIOS lors d’un changement d’avion ;
- chargements concurrents, sauvegarde pendant une transition et réponse tardive dans l’interface ;
- priorité du mode de la molette sur l’affectation générale et persistance des contextes ;
- couleurs des LED selon la valeur exportée, extinction à la déconnexion et rejet des règles invalides ;
- modification d’un contexte et conservation des règles à l’aller-retour JSON.

Commandes PowerShell depuis la racine du projet :

```powershell
Push-Location backend
go test ./...
go vet ./...
Pop-Location

Push-Location frontend
node scripts/test-panel-mappings.mjs
npm run build
Pop-Location
```

La suite Go, l’analyse statique, le test JavaScript et la compilation de l’interface passent. La compilation signale un sélecteur CSS inutilisé dans `DebriefPanel.svelte`, sans rapport avec les panneaux.

Un exécutable de test `dcsmanager-panels.exe` a aussi été compilé à la racine, sans remplacer `dcsmanager.exe`. Fermer l’autre instance avant de le lancer et sauvegarder les profils : les deux exécutables utilisent les mêmes données du gestionnaire.

Pour reconstruire l’application complète :

```powershell
.\build.ps1
```

## Essais matériels à effectuer

1. Brancher les panneaux avec l’envoi activé uniquement après configuration ; vérifier que leur connexion seule ne commande ni train ni volets.
2. Tourner le trim dans les deux sens, puis vérifier l’inversion.
3. Passer AUTO THROTTLE de OFF à ON puis de ON à OFF.
4. Actionner FLAPS UP, relâcher au centre ; refaire avec DOWN. Vérifier l’absence d’action opposée au relâchement.
5. Vérifier les boutons AP et leurs voyants séparément, puis ensemble.
6. Tester les chiffres LCD et leur signe ; vérifier les cinq positions de chaque ligne.
7. Passer par les cinq modes ; un mode non affecté doit laisser la ligne vide.
8. Modifier les valeurs directement dans le cockpit ; vérifier l’actualisation des LED et chiffres.
9. Changer d’avion : aucune valeur de l’ancien cockpit ne doit réapparaître sans export du nouvel avion.
10. Débrancher et rebrancher le PZ70 ; vérifier le réaffichage complet.

## Retour arrière

Avant les essais, conserver une copie de `mappings.json`, situé à côté de la base du gestionnaire. Conserver également l’exécutable précédent si disponible. Pour revenir à cette version, fermer le gestionnaire, remettre l’ancien exécutable et restaurer la copie du profil si nécessaire.

Les extensions ajoutent les champs optionnels `mode`, `state_on` et `state_off` aux commandes et `rules` aux LED. Les profils existants sans ces champs conservent leur fonctionnement, sauf les commandes `fixed_step` désormais transmises correctement comme INC/DEC. Avant de réutiliser un ancien exécutable, restaurer aussi les anciens profils : cette version peut ignorer ces champs et interpréter plusieurs contextes comme plusieurs commandes simultanées. Aucun commit ni push n’a été réalisé pendant cette reprise.

## Complément F/A-18C — 7 octobre 2026

Voir [la matrice complète F/A-18C](integration-fa18-panels.fr.md) : 19 attributions, trois LED de train, molette HDG/CRS avec recentrage et liste explicite des commandes encore sans équivalent. Le champ optionnel JSON pulse_reset définit la valeur envoyée 50 ms après chaque impulsion d’encodeur set_state. Les positions state_on/state_off suivent le sens de rotation (et son inversion).


## Plugin DCS Manager sans pilote supplémentaire

À la demande de l’utilisateur, la passerelle vJoy a été retirée. Le [plugin Lua DCS Manager](plugin-panels-dcs.fr.md) complète DCS-BIOS pour le trim longitudinal Hornet, avec diagnostic, installation et relâchement automatique. Les autres touches et LED conservent leur liaison BIOS. Les attributions restent dans l’application. Tests simulés réussis ; validation du trim en cockpit encore nécessaire.
