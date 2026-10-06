# Recherche générale de bugs — 6 octobre 2026

> **État au 6 octobre 2026 (soir).** Les sept problèmes ci-dessous sont désormais
> corrigés : 1 et 2, 3, 4 et 5, 6, 7. Les sections qui suivent sont conservées telles
> quelles comme relevé d’origine ; voir le CHANGELOG pour le détail des corrections.

## Périmètre et méthode

Revue de l’état de travail actuel, y compris les modifications locales existantes : backend Go, interface Svelte, sauvegarde/restauration, aérodromes et cartes, échanges DCS-BIOS et panneaux. Les modifications existantes ont été conservées. Aucune correction fonctionnelle n’a été appliquée.

Les reproductions ont utilisé des dossiers temporaires, des requêtes simulées et des trames DCS-BIOS construites en mémoire. Aucun fichier utilisateur DCS ni panneau physique n’a été manipulé. Les fichiers de test temporaires ont été retirés après vérification.

Ce rapport présente les problèmes vérifiés pendant cette passe ; il ne garantit pas l’absence d’autres bugs. Les interactions avec une session DCS réelle et les périphériques USB n’ont pas été testées.

## Résultats

| N° | Priorité | Problème | Validation |
|---|---|---|---|
| 1 | P1 — haute | Restauration poursuivie après échec de la copie de secours | Reproduction Go |
| 2 | P1 — haute | Mélange de profils lors d’un changement rapide d’avion | Reproduction JavaScript, y compris la requête de sauvegarde |
| 3 | P2 — moyenne | Voyants non synchronisés sur les changements ordinaires DCS-BIOS | Reproduction Go et lecture du branchement applicatif |
| 4 | P2 — moyenne | Cartes d’un ancien aérodrome affichées sous le nouvel aérodrome | Reproduction JavaScript |
| 5 | P2 — moyenne | Recherche des aérodromes proches sans filtre de théâtre | Reproduction de la requête et lecture du backend |
| 6 | P2 — moyenne | Molettes : les deux directions produisent le même argument | Lecture du chemin complet décodage/mapping |
| 7 | P2 — moyenne | Commandes doublées si envoi normal et mode test sont activés | Lecture du branchement applicatif |

### 1. La restauration continue sans copie de secours

**Localisation :** `backend/internal/backup/backup.go`, lignes 469–477.

La création de la sauvegarde préalable est facultative dans le chemin d’exécution : si `createTo` échoue, l’erreur est ignorée et l’extraction continue. Une restauration peut donc écraser les réglages ou les affectations DCS sans préserver leur état précédent, tout en retournant un succès.

**Reproduction :** sauvegarder un fichier `Config/options.lua`, modifier son contenu, puis provoquer l’échec de création de la copie de secours en plaçant un dossier à l’emplacement de son fichier temporaire. `Restore` retourne un succès, le fichier actuel est remplacé par celui de l’archive et `SafetyBackup` est vide.

**Correction proposée :** rendre la copie préalable obligatoire avant toute écriture et remonter explicitement son erreur. Définir également le comportement des archives sans manifeste ou sans catégories, pour éviter une restauration silencieuse sans possibilité de retour arrière.

### 2. Un chargement tardif mélange les profils d’avions

**Localisation :** `frontend/src/lib/mappings.js`, lignes 48–64.

`loadMappings` change immédiatement `mappingAircraft`, mais applique ensuite les résultats de la requête sans vérifier s’ils correspondent toujours au dernier avion choisi.

**Reproduction :** lancer le chargement du F/A-18C, puis celui du F-16C. Faire arriver la réponse F-16C en premier et la réponse F/A-18C en dernier. L’avion sélectionné reste `F-16C_50`, mais les affectations affichées contiennent `BATTERY_SW` du Hornet. La sauvegarde suivante envoie ces affectations à l’URL du profil F-16C.

Le backend accepte ces identifiants sans validation systématique contre le catalogue de l’avion lors de la sauvegarde. Le problème peut donc être persisté.

**Correction proposée :** utiliser un compteur de requêtes comme dans `debriefs.js` et `stats.js`, ou annuler les anciennes requêtes. Publier ensemble le nom de l’avion et ses données lorsqu’un chargement valide est terminé ; désactiver la sauvegarde pendant la transition.

### 3. Les changements de voyants DCS-BIOS ne déclenchent pas leur synchronisation

**Localisation :** `backend/internal/dcsbios/client.go`, lignes 213–237 ; `backend/internal/app/app.go`, lignes 339–356.

Le callback DCS-BIOS est appelé à la première trame et lors d’un changement du nom de l’avion. Une trame qui change uniquement une valeur de voyant met la mémoire à jour sans appeler ce callback. Or la synchronisation des voyants est branchée sur ce callback et sur certains événements des panneaux.

**Impact :** un train ou un mode de pilote automatique modifié dans le cockpit peut laisser un voyant physique dans son état précédent. Une action ultérieure sur un panneau peut déclencher une synchronisation, ce qui masque le problème.

**Reproduction :** envoyer une première trame donnant le nom d’avion, puis une seconde qui modifie une adresse d’export. La nouvelle valeur apparaît en mémoire, mais le nombre d’appels au callback reste identique.

**Correction proposée :** ajouter un signal pour chaque trame appliquée, distinct du callback de changement visible, et y raccorder le pilote de voyants. Celui-ci évite déjà les écritures USB lorsque le résultat ne change pas.

### 4. Les cartes peuvent appartenir au mauvais aérodrome

**Localisation :** `frontend/src/lib/aerodromes.js`, lignes 19–34.

`loadAerodromeCharts` ne protège pas contre les réponses arrivant dans un ordre différent des sélections.

**Reproduction :** sélectionner A puis B ; répondre à B puis à A. Le stockage des cartes termine avec les cartes de A alors que le composant conserve B comme aérodrome sélectionné.

**Impact :** l’utilisateur peut ouvrir une carte d’approche ou une carte de piste qui ne correspond pas à l’aérodrome affiché.

**Correction proposée :** associer chaque chargement à une sélection ou à un compteur, et ignorer les résultats périmés. Invalider également les chargements lors d’un changement de théâtre.

### 5. La recherche des aérodromes proches ignore le théâtre sélectionné

**Localisation :** `frontend/src/lib/aerodromes.js`, ligne 87 ; `backend/internal/api/aerodrome.go`, lignes 26–38.

`loadNearest` transmet uniquement la latitude et la longitude. Sans paramètre `theatre`, le backend assemble les aérodromes de tous les théâtres.

**Reproduction :** sélectionner `Syria`, fournir une position d’avion et lancer la recherche de proximité. L’URL produite ne contient pas `theatre=Syria`. Le backend sert donc l’ensemble des théâtres.

**Impact :** la liste peut contenir des aérodromes absents de la carte sélectionnée, notamment lorsque les zones géographiques de plusieurs terrains se recouvrent.

**Correction proposée :** inclure le théâtre pertinent dans la requête et maintenir la cohérence entre le théâtre affiché et celui de la position utilisée.

### 6. Le mapping ignore le sens des molettes

**Localisation :** `backend/internal/panel/panel.go`, définition des encodeurs ; `backend/internal/mapping/store.go`, lignes 350–381.

Le décodage distingue les deux directions avec `Clockwise`, mais ne produit un événement d’encodeur que sur le front actif. Le mapping utilise `Active` et ignore `Clockwise`.

**Impact :** les deux directions de `LCD_WHEEL` et `PITCH_TRIM` reçoivent le même argument. Avec `variable_step`, elles produisent toutes deux `+1` sans inversion, ou toutes deux `-1` avec inversion. La molette ne permet donc pas de régler dans les deux sens.

**Correction proposée :** traiter explicitement les encodeurs et dériver le signe de leur direction, avec une inversion éventuelle. Respecter le pas proposé par les métadonnées lorsque nécessaire.

### 7. Le mode test et l’envoi normal doublent les commandes

**Localisation :** `backend/internal/app/app.go`, lignes 399–410.

Le gestionnaire appelle `mappings.Apply`, puis appelle aussi `mappings.ApplyForced` si le mode test est activé. Les deux options peuvent être actives simultanément.

**Impact :** un même événement physique transmet deux fois sa commande. Les commandes incrémentales peuvent avancer de deux pas ; une action à bascule peut s’annuler selon son comportement dans DCS.

**Correction proposée :** choisir un seul chemin d’envoi pour chaque événement, selon l’état du mode test et de l’envoi normal.

## Vérifications exécutées

- `go test ./...` : tous les packages testés passent sur l’état initial de cette revue ; plusieurs résultats proviennent du cache Go.
- `go vet ./...` : aucune anomalie signalée.
- `npm run build` : compilation réussie. Un avertissement de sélecteur CSS inutilisé dans `DebriefPanel.svelte`, sans échec de compilation.
- Reproductions Go : échec de la copie de secours et absence de notification sur un changement d’export DCS-BIOS, confirmés.
- Reproductions JavaScript : mélange de profils, sauvegarde vers le mauvais profil, réponses de cartes dans le désordre et absence du filtre de théâtre, confirmés.

Les tests Go ciblés et la compilation de l’interface ont nécessité une exécution hors du bac à sable après des refus d’accès au cache ou de lancement du compilateur. Ces refus ne sont pas des bugs de l’application.

## Ordre de correction conseillé

1. Bloquer la restauration si la copie de secours échoue.
2. Empêcher le mélange et l’enregistrement des profils d’avions.
3. Corriger les cartes d’aérodrome et le filtrage par théâtre.
4. Synchroniser les voyants sur les trames DCS-BIOS.
5. Corriger les directions de molettes et éviter les doubles envois.
