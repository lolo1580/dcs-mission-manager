# Fonctions manquantes ou incomplètes — 6 octobre 2026

> Inventaire historique avant la reprise des panneaux. Certaines fonctions, notamment le LCD configurable et l’inversion à l’ajout, sont maintenant présentes. Voir [l’état actuel de l’intégration](integration-panels.fr.md).

## Périmètre

Inventaire établi à partir du code actuel, de l’interface Svelte, des routes API et de la documentation du projet. Aucune fonctionnalité n’a été développée pendant cette revue.

Une fonction absente de l’interface peut déjà exister dans l’API ou la CLI. Les fonctions retirées volontairement sont signalées séparément. Les propositions ci-dessous constituent des pistes d’évolution, pas des exigences déjà acceptées.

Les bugs de fonctionnement sont décrits dans [le rapport de bugs](audit-bugs-general.fr.md).

## Fonctions à compléter

| Fonction | État constaté | Ce qu’il manque | Priorité proposée |
|---|---|---|---|
| Accès aux débriefings | Composant et API présents ; entrée de navigation marquée `hidden: true` | Un accès visible à l’historique des débriefings, si leur réactivation est souhaitée | Haute |
| Statistiques d’une ancienne mission | Une liste des missions réelles enregistrées permet désormais de choisir `missionId` | Rien pour la sélection de base ; filtres avancés éventuels | Terminée |
| Modification d’une affectation de panneau | L’interface propose ajout et suppression ; l’inversion existante est seulement affichée | Édition directe, choix de l’inversion et conservation des autres champs | Haute |
| Création de profils pour d’autres avions hors session | Le sélecteur liste les profils déjà enregistrés ; un avion actif peut fournir un autre nom | Création explicite d’un profil à partir d’un catalogue, même sans mission DCS active | Haute |
| Sauvegarde des données du gestionnaire | Les archives couvrent le profil DCS dans Saved Games | Sauvegarde/restauration de la base SQLite et de `mappings.json` avec une méthode adaptée aux données en cours d’utilisation | Haute |
| Affichage LCD du PZ70 | Encodeur USB présent, mais aucune affectation LCD dans les profils | Source DCS-BIOS, conversion, unité, format et affectation à chaque ligne d’affichage | Moyenne |
| Molette du PZ70 dépendant du sélecteur | Les positions ALT/VS/IAS/HDG/CRS et la molette sont exposées séparément | Affectations conditionnelles selon la position du sélecteur | Moyenne |
| Actions avancées de mapping | Une commande par contrôle ; argument dérivé d’un état booléen | Valeurs explicites, positions intermédiaires, pas réglable et éventuellement plusieurs actions par contrôle | Moyenne |
| Import/export et duplication des profils | API de lecture/sauvegarde présente, sans commandes dédiées dans l’interface | Export JSON, import validé, duplication entre profils et aperçu avant remplacement | Moyenne |
| Installation/réparation des scripts depuis l’interface | L’interface lit l’état ; la CLI possède `install-lua` et `uninstall-lua` | Bouton d’installation ou de mise à jour utilisant la fusion et les sauvegardes existantes | Moyenne |
| Paramètres généraux dans l’interface | La configuration passe principalement par variables d’environnement | Édition persistante des chemins, de la conservation des données et des autres réglages utiles, avec indication du besoin de redémarrer | Moyenne |
| Entretien des données depuis l’interface | Purge disponible dans l’API et la CLI | Gestion visible des sessions, suppression ciblée et aperçu de la portée avant suppression | Basse |

## Points importants

### Débriefings : présents mais masqués volontairement

Dans `frontend/src/App.svelte`, l’entrée `debriefs` est marquée `hidden: true`. Le composant `DebriefPanel.svelte` et les routes `/api/debriefs` existent toujours. Le commentaire du code indique que cette dissimulation est volontaire et temporaire : rétablir l’accès est donc une décision de produit, pas une fonction à reconstruire entièrement.

### Statistiques : le backend sait déjà filtrer une mission précise

`backend/internal/api/stats.go` accepte `scope=mission&missionId=N`. L’interface propose les missions réelles enregistrées et transmet l’identifiant choisi. La vue carrière affiche aussi l’évolution par mission du score, des kills et des atterrissages, avec un filtre par UCID.

### Panneaux : l’éditeur couvre seulement les associations simples

`frontend/src/lib/PanelsPanel.svelte` propose des formulaires d’ajout et des boutons de suppression. Le modèle Go possède déjà `invert`, mais aucun champ de saisie correspondant n’est proposé. Une modification exige donc actuellement de supprimer puis recréer l’affectation, et l’inversion doit être gérée hors de ce formulaire.

Le modèle ne contient pas de valeur d’argument explicite, de condition sur le sélecteur ni d’affectation LCD. Ces fonctions nécessitent une extension du format des profils, de leur validation et du moteur de mapping.

Le sens des molettes ignoré et le double envoi en mode test sont des bugs déjà recensés ; ils ne doivent pas être confondus avec ces extensions fonctionnelles.

### Sauvegarde DCS et sauvegarde du gestionnaire

Les catégories définies dans `backend/internal/backup/backup.go` ciblent le dossier Saved Games : logbook, commandes, configuration DCS, scripts, kneeboard, missions et mods. Elles ne sauvegardent pas automatiquement la base du gestionnaire ni son fichier `mappings.json`, situés à côté de cette base.

Une sauvegarde complète doit préserver les deux ensembles. La base SQLite doit être sauvegardée de façon cohérente ; une copie brute d’un fichier actif ne suffit pas à définir une procédure fiable.

### Installation : un état visible, des actions en CLI

`InstallPanel.svelte` et `install.js` affichent les scripts et les mods détectés. Les actions d’installation et de désinstallation existent dans `backend/cmd/dcsmanager/main.go`, mais ne sont pas exposées comme actions dans ce panneau. Il serait préférable de réutiliser la logique de fusion existante plutôt que d’introduire une deuxième méthode d’installation.

## Détail : affectations des contrôles et des LED des panneaux

### Ce qui existe

- Affectation par avion d’un contrôle du PZ55 ou du PZ70 à une commande DCS-BIOS.
- Ajout et suppression des affectations depuis l’interface.
- Inversion d’une entrée dans le format JSON, mais sans champ d’édition dans l’interface.
- Affectation des trois voyants de train du PZ55 et des huit voyants de boutons du PZ70 à des exports DCS-BIOS.
- Choix d’une couleur verte, rouge ou jaune pour un voyant du PZ55 lorsqu’il est actif ; les voyants du PZ70 sont monochromes.
- Journal des événements physiques et des commandes calculées.

Les commandes et les voyants sont configurés séparément. Affecter `AP_BUTTON` à une commande de pilote automatique ne configure pas automatiquement `LIGHT_AP` : ce voyant doit lire l’export qui indique l’état réel du mode dans le cockpit.

Les profils intégrés sont des points de départ partiels : ils ne couvrent pas toutes les commandes physiques ni tous les voyants pour chaque avion.

### Affectations des boutons et interrupteurs à compléter

| Fonction absente | Utilité |
|---|---|
| Vue graphique interactive du panneau | Cliquer sur le contrôle physique correspondant au lieu de choisir seulement son identifiant dans une liste |
| Apprentissage par manipulation | Actionner un bouton ou interrupteur pour le sélectionner automatiquement dans le formulaire |
| Édition directe d’une affectation | Changer commande, interface ou inversion sans supprimer puis recréer |
| Recherche dans le catalogue DCS-BIOS | Retrouver une commande par nom ou description dans un grand catalogue |
| Valeurs distinctes pour chaque position | Définir les valeurs exactes ON/OFF ou les positions d’un sélecteur au lieu de zéro et du maximum |
| Choix des événements de déclenchement | Choisir pression, relâchement ou changement de position selon le contrôle |
| Actions multiples | Affecter plusieurs commandes à un même contrôle, si le cockpit l’exige |
| Conditions sur le sélecteur PZ70 | Faire régler l’altitude, le cap ou une autre valeur par la même molette selon ALT/VS/IAS/HDG/CRS |
| Identification d’un exemplaire de panneau | Affecter différemment deux périphériques du même modèle ; le profil actuel ne stocke que le modèle |

La recherche par manipulation est une amélioration proposée : le journal reçoit déjà les événements nécessaires, mais ne les utilise pas pour sélectionner automatiquement le contrôle du formulaire.

### Affectations des voyants à compléter

| Fonction absente ou limitée | Utilité |
|---|---|
| Édition directe d’une sortie | Changer sa source ou sa couleur sans supprimer puis recréer |
| Sélection de tous les exports utilisables | Le formulaire propose les contrôles sans entrée ; un contrôle possédant à la fois des entrées et une sortie exploitable n’est pas proposé |
| Choix de l’export précis | Le pilote lit uniquement `Outputs[0]` ; un contrôle peut nécessiter un autre export |
| Inversion et comparaisons | Allumer selon `valeur == 1`, `valeur > seuil` ou une condition inversée, plutôt que seulement non-zéro |
| Plusieurs conditions de couleur pour le PZ55 | Vert si verrouillé sorti, rouge si état dangereux, éteint si rentré, selon les exports disponibles pour l’avion |
| Combinaison de sources | Déduire un état à partir de plusieurs informations, par exemple position de train et verrouillage |
| Clignotement configuré | Signaler une transition ou une alarme ; aucune temporisation de clignotement n’est implémentée |
| Test matériel des LED | Allumer chaque voyant et tester les couleurs indépendamment d’une mission et d’une affectation DCS-BIOS |
| Aperçu de la valeur source | Afficher la valeur exportée et expliquer pourquoi la règle allume ou éteint le voyant |
| Affichage LCD | Affecter les valeurs du cockpit aux deux lignes du PZ70 |

**Limite actuelle du PZ55 :** une cible de voyant accepte une seule affectation avec une couleur fixe. Le code sait physiquement produire les trois couleurs, mais le format des profils ne décrit pas une règle permettant de passer du vert au rouge selon plusieurs états. Le jaune correspond à l’activation des deux composantes lumineuses.

**Limite actuelle du PZ70 :** les huit voyants AP/HDG/NAV/IAS/ALT/VS/APR/REV sont des sorties configurables. Ne pas proposer de couleur RGB : ces voyants sont monochromes.

### Corrections préalables

Avant d’étendre ces fonctions, corriger les bugs déjà recensés : mélange des profils, sens des molettes ignoré, double envoi en mode test et synchronisation des voyants non déclenchée par les mises à jour ordinaires DCS-BIOS.

L’ordre conseillé pour les panneaux est : fiabiliser l’envoi et le retour des états, ajouter l’édition et l’apprentissage des contrôles, puis étendre les règles LED et l’affichage LCD.

## Fonctions retirées volontairement

- **Carte en temps réel :** retrait explicitement documenté dans le README.
- **Analyse des trajectoires, heatmaps et sorties :** la vue, les routes `/api/analytics/*` et leurs calculs ont été retirés. La collecte des positions et des pertes reste présente.
- **Onglet Session, événements et chat :** l’interface a été retirée ; la collecte et certaines routes backend subsistent.

Ces fonctions peuvent devenir des évolutions futures si elles sont souhaitées. Leur absence actuelle ne prouve pas une implémentation oubliée.

## Ordre de travail proposé

1. Corriger les bugs prioritaires de restauration et de mélange des profils.
2. Ajouter l’édition des affectations et la création de profils hors session.
3. Compléter la sauvegarde du gestionnaire.
4. Ajouter le choix des anciennes missions ; décider de la visibilité des débriefings.
5. Étendre le PZ70 : sélection contextuelle, valeurs réglables et LCD.
6. Compléter les actions de paramètres, d’installation et d’entretien.

## Fichiers examinés

- `README.fr.md`
- `docs/architecture.md`
- `frontend/src/App.svelte`
- `frontend/src/lib/SettingsPanel.svelte`
- `frontend/src/lib/CareerPanel.svelte`
- `frontend/src/lib/StatsPanel.svelte`
- `frontend/src/lib/stats.js`
- `frontend/src/lib/PanelsPanel.svelte`
- `frontend/src/lib/mappings.js`
- `frontend/src/lib/InstallPanel.svelte`
- `frontend/src/lib/install.js`
- `frontend/src/lib/BackupPanel.svelte`
- `backend/internal/api/api.go`
- `backend/internal/api/stats.go`
- `backend/internal/api/mappings.go`
- `backend/internal/api/maintenance.go`
- `backend/internal/backup/backup.go`
- `backend/internal/mapping/store.go`
- `backend/internal/panel/output.go`
- `backend/cmd/dcsmanager/main.go`
