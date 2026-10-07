# Vérification des profils de panneaux — 7 octobre 2026

Ce rapport décrit l’état avant correction. Les profils ont ensuite été corrigés et trois nouveaux profils ajoutés ; consulter [les profils actuels](../profiles/panels/README.fr.md).

## Périmètre et résultat

Vérification des quatre profils réellement enregistrés dans `data/mappings.json`, comparés aux catalogues DCS-BIOS installés dans `Saved Games/DCS/Scripts/DCS-BIOS/doc/json` et, pour les points ambigus, aux modules Lua locaux. Aucun essai dans DCS ni écriture USB n’a été effectué. Le fichier des profils n’a pas été modifié.

Les 18 attributions d’entrée utilisent des commandes existantes et des interfaces acceptées par leurs catalogues. Les 12 sources LED existent et exportent des valeurs. Les 4 sources LCD existent, disposent de l’index demandé et exportent des entiers. Cela valide les identifiants, mais ne suffit pas à garantir que les positions commandées correspondent à l’intention de l’utilisateur.

Empreinte SHA256 du fichier vérifié : `D31617EAA873A0E9AB94B31F921F0908053E67DE11DFDE300A34EBFEE9F716C3`.

## F/A-18C : erreur confirmée sur la batterie

Le catalogue déclare `BATTERY_SW` avec les positions `0 = ON`, `1 = OFF`, `2 = ORIDE`.

L’attribution `MASTER_BAT` utilise `set_state`, sans inversion. Le gestionnaire calcule les valeurs à partir de zéro et du maximum, ici 2 :

| Interrupteur PZ55 | Commande actuelle | Résultat cockpit |
|---|---|---|
| Activé | `BATTERY_SW 2` | ORIDE |
| Coupé | `BATTERY_SW 0` | ON |

Cette attribution ne réalise donc pas une commande batterie ON/OFF. Une simple inversion échangerait ON et ORIDE et ne corrigerait pas le problème. Il faut permettre deux valeurs explicites dans le mapping : activation 0, désactivation 1. Ce réglage n’est pas encore disponible ; ne pas considérer l’attribution batterie du Hornet comme validée.

Les deux commandes de train existent avec un maximum de 1. Le profil génère `GEAR_LEVER 0` pour UP et `GEAR_LEVER 1` pour DOWN. Le sens final doit être contrôlé dans le cockpit, car le catalogue ne fournit pas de libellés de positions pour ce sélecteur.

Les volets produisent `FLAP_SW 0` pour UP et `FLAP_SW 2` pour DOWN : ce sont respectivement AUTO et FULL. La position HALF, valeur 1, est inaccessible avec ces deux attributions. Le retour au centre ne produit pas de nouvelle position. Cela peut être un choix de raccourci, mais ce n’est pas une commande progressive des trois positions.

Les trois LED vertes lisent bien les voyants de train correspondants. Aucun LCD n’est configuré pour cet avion.

## F-5E-3 : volets incomplets et position EMER UP

La batterie et la pompe gauche sont des commandes à deux positions compatibles avec le mapping actuel. Les deux positions de train et leurs trois LED sont présentes ; vérifier le sens du train dans le cockpit.

Les volets lisent les positions du sélecteur `FLAPS` :

| Contrôle PZ70 | Commande | Position du sélecteur cockpit |
|---|---|---|
| FLAPS UP | `FLAPS 0` | EMER UP |
| FLAPS DOWN | `FLAPS 2` | FULL |
| Retour au centre | Aucune | Position conservée |

La position THUMB SW, valeur 1, n’est pas disponible. UP active donc la position nommée EMER UP, et non une simple impulsion vers la position précédente. Confirmer si ce comportement est voulu avant utilisation. Le module possède également un sélecteur `A_FLAPS`, mais sa présence seule ne permet pas de décider quelle commande de volets l’utilisateur souhaite affecter.

HDG et CRS utilisent les sources HSI existantes, sur la ligne haute. Leur export est normalisé sur 0..65535 et les modules Lua utilisent des limites 0..1 ; l’échelle `360/65535` est cohérente pour convertir en degrés. Vérifier les angles affichés dans DCS ; l’extrémité 65535 produit 360, sans remise à zéro automatique.

## F-16C : attribution fonctionnelle mais libellé trompeur

Le contrôle physique `MASTER_BAT` commande `FUEL_MASTER_SW`, décrit comme « FUEL MASTER Switch, MASTER/OFF ». Il commande le carburant principal, pas la batterie. L’identifiant et l’interface sont valides ; la correspondance fonctionnelle est à confirmer avec l’utilisateur.

Le trim `PITCH_TRIM` accepte `variable_step` avec un pas suggéré de 3200. Le gestionnaire utilise ce pas dans les deux sens. Vérifier le sens souhaité sur le panneau réel et utiliser l’inversion si nécessaire.

Les trois sources vertes de train existent. En revanche, aucune commande de train n’est enregistrée dans ce profil local. Le profil fourni dans le code possède UP/DOWN, mais les profils existants ne sont pas remplacés automatiquement. Il ne faut donc pas confondre le profil fourni et celui réellement utilisé.

Aucune affectation LCD n’est enregistrée.

## Mirage 2000C : compatibilité logicielle confirmée, couverture partielle

La batterie, la pompe gauche et le train utilisent des commandes à deux positions compatibles. Le profil génère `LDG_LEV 0` pour UP et `LDG_LEV 1` pour DOWN ; contrôler le sens final dans le cockpit.

Le bouton AP transmet `AP_MASTER_BTN 1` à la pression et `AP_MASTER_BTN 0` au relâchement. Malgré une interface `action` décrite comme TOGGLE dans les métadonnées, le module définit ce contrôle avec `definePushButton`, dont le processeur accepte aussi les valeurs numériques. La commande de pression/relâchement est donc compatible avec le code Lua installé ; elle n’a pas été essayée en vol.

Le voyant rouge du train et les voyants verts AP/ALT lisent des exports LED existants. Le voyant rouge correspond à la lampe du levier, pas à trois indications indépendantes de verrouillage des jambes du train.

HDG et CRS affichent les sources HSI documentées, sur la ligne haute, avec la même conversion angulaire que le F-5. Aucun trim, volet, auto-throttle ou réglage de molette n’est configuré dans ce profil.

## Ce qui manque dans les quatre profils

- Aucun contexte de molette ALT/VS/IAS/HDG/CRS.
- Aucun auto-throttle attribué.
- Aucune règle LED à plusieurs couleurs : seuls les allumages simples sont configurés.
- Aucun affichage ALT/VS/IAS ni ligne LCD basse.
- Aucun profil ne couvre toutes les touches des deux panneaux.

L’absence d’une fonction dans le profil ne signifie pas qu’elle existe sur l’avion. Ajouter les commandes seulement après identification des fonctions réellement disponibles.

## Vérifications reproductibles

Depuis la racine du projet :

```powershell
.\tools\verify-panel-profiles.ps1

Push-Location backend
go test ./internal/mapping -run TestStarterProfilesMatchDCSBIOS -v
Pop-Location
```

Le script vérifie le fichier local ; le test Go vérifie les profils fournis dans le code. Tous deux passent sur les métadonnées installées. Le script affiche cinq avertissements relatifs aux attributions à trois positions. Le test Go n’a pas été ignoré : il a réellement chargé les catalogues locaux.

Le script est une vérification d’identifiants et d’interfaces, complétée ici par la lecture du code de génération des commandes et des modules Lua. Il ne certifie pas la polarité de chaque interrupteur ni la fidélité des angles en cockpit.

## Ordre des corrections à prévoir

1. Ajouter les valeurs explicites de `set_state` et corriger la batterie du Hornet en ON/OFF.
2. Choisir le comportement souhaité pour les sélecteurs de volets Hornet et F-5, avec accès aux positions intermédiaires si nécessaire.
3. Confirmer l’attribution carburant du F-16 et compléter les commandes de train si souhaité.
4. Compléter les commandes de molette et sorties de l’avion principal, puis effectuer les essais physiques.

La présente vérification laisse les profils et l’exécutable inchangés. Avant toute correction des profils, conserver un export JSON pour permettre le retour arrière.
