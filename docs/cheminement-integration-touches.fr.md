# Cheminement de l’intégration des touches PZ55 / PZ70

## Objectif et état au 7 octobre 2026

Rendre les attributions utilisables depuis une représentation des panneaux et conserver les fonctions existantes de commandes, voyants et LCD. La compilation et les essais logiciels sont effectués. Les essais USB réels et les commandes dans DCS restent à valider par l’utilisateur.

## 1. Reprise du fonctionnement des entrées

La première passe a vérifié le trajet USB → décodage → profil de l’avion → commande DCS-BIOS. Le premier rapport USB sert maintenant de référence initiale, sans envoyer les positions déjà présentes comme des actions nouvelles.

L’auto-throttle est traité comme un interrupteur maintenu ON/OFF. Le trim produit des impulsions dans les deux sens, avec inversion possible. Les volets UP/DOWN avec retour au centre ne renvoient pas une action opposée au relâchement pour les commandes de position ou de pas ; les interfaces `action` conservent leur relâchement. Les anciennes positions des sélecteurs ne déclenchent plus une deuxième commande lorsqu’elles deviennent inactives.

Ces changements et leurs limites sont détaillés dans [la reprise de l’intégration](integration-panels.fr.md).

## 2. Compléments des profils

Les attributions de la molette PZ70 peuvent dépendre du mode ALT/VS/IAS/HDG/CRS. Une attribution spécifique au mode prend la priorité sur l’attribution générale. Les règles des voyants sont évaluées dans l’ordre et la première condition satisfaite choisit la couleur. L’édition directe et l’import/export JSON sont disponibles.

La synchronisation périodique des sorties traite aussi la déconnexion DCS-BIOS, sans attendre une nouvelle trame, lorsque les sorties sont activées. Les fonctions de commandes et de sorties restent activées séparément.

## 3. Problème d’ergonomie constaté

Le premier écran empilait trois listes et trois formulaires. Pour configurer une touche, il fallait connaître son identifiant, retrouver le formulaire et sélectionner son panneau. Les noms DCS-BIOS étaient présentés sans recherche et le bouton d’édition utilisait aussi le mot « enregistrer ».

L’utilisateur a signalé que cet écran n’était pas pratique. La suite de l’intégration porte donc sur le parcours de configuration, en conservant les profils déjà créés.

## 4. Vue des panneaux et liens vers les attributions

Un composant `PanelBoard.svelte` présente une représentation schématique, adaptée aux petites largeurs. Le PZ55 expose les interrupteurs, les éclairages, les cinq positions du sélecteur moteur, les deux positions du train et ses trois voyants. Le PZ70 expose les cinq modes, les deux lignes LCD, la molette, les huit boutons et voyants, l’auto-throttle, les volets et le trim.

Le clic est une action de configuration : il n’envoie aucune commande DCS et ne commande pas directement le matériel.

| Action | Résultat |
|---|---|
| Cliquer sur une touche | Ouvre son attribution existante ou prépare une nouvelle attribution |
| Cliquer sur un voyant | Ouvre la source, la couleur et les règles du voyant sélectionné |
| Cliquer sur une ligne LCD | Ouvre le réglage de cette ligne pour le mode affiché |
| Choisir ALT/VS/IAS/HDG/CRS dans le dessin | Change le contexte de configuration de la molette et des lignes LCD |
| Cliquer sur la roue dentée d’un mode | Configure une commande éventuelle pour cette position du sélecteur physique |

Le mode choisi dans le dessin est un contexte d’édition ; il ne déplace pas le sélecteur réel. Le LCD dessiné affiche le nom de sa source, pas une valeur live. Les repères verts indiquent des attributions enregistrées, pas l’état du cockpit. Une affectation générale de molette apparaît comme secours ; cliquer dans un mode sans attribution spécifique prépare une nouvelle attribution pour ce mode, sans modifier le secours.

## 5. Réduction des manipulations

Quatre sections limitent l’affichage aux touches, aux voyants, au LCD ou au diagnostic. Les tableaux complets sont conservés dans des listes repliables. Un clic dans le dessin choisit automatiquement la bonne section et les champs correspondants.

La recherche filtre les commandes ou sources par identifiant, description et catégorie DCS-BIOS. Les descriptions sont affichées avec les identifiants. Une commande déjà sélectionnée reste disponible pendant la recherche, afin de ne pas modifier une attribution silencieusement. La liste des interfaces suit la commande choisie.

Les boutons **Modifier**, **Enregistrer** et **Annuler** ont des fonctions distinctes. Choisir manuellement un avion évite qu’un changement d’avion DCS-BIOS remplace ensuite automatiquement le profil en cours d’édition pendant cette ouverture de l’écran. Au changement de profil, les champs et sélections précédents sont effacés.

## 6. Vérifications et corrections pendant les essais

- La compilation Svelte et les essais JavaScript des profils passent.
- Les essais dans le navigateur utilisent `panel-ui-fixture.cjs`, qui intercepte les requêtes API avec deux profils fictifs et quatre contrôles. Aucun profil utilisateur ni matériel n’est modifié par cette vérification.
- Le premier essai visuel a découvert que le dessin affichait « Non attribué » après le chargement d’un profil contenant des commandes : les lectures dans les fonctions du composant n’étaient pas recalculées lors de la réception des attributions. Le dessin est désormais reconstruit lors du remplacement de la liste des commandes, du modèle ou de la langue.
- Le parcours `panel-ui-review.cjs` vérifie : édition AP, recherche, changement d’interface, création d’un contexte HDG, attribution LED, attribution LCD, sélection du train PZ55, passage entre profils et retour au profil sauvegardé.
- Le même parcours vérifie une fenêtre de 760 pixels de large sans débordement horizontal de la page. Des captures sont enregistrées dans `output/playwright/`.
- La lecture des captures a conduit à élargir le seuil de disposition compacte à 1 000 pixels : la barre latérale de l’application réduit la largeur disponible pour le dessin.
- La suite Go, `go vet` et la reconstruction de l’exécutable passent. L’avertissement CSS existant dans `DebriefPanel.svelte` est sans rapport avec les panneaux.

Ces essais de navigateur valident le parcours de l’interface sur des données simulées. Ils ne remplacent pas un essai du transport USB ou DCS-BIOS.

## 7. Vérifications reproductibles

Depuis la racine du projet, dans PowerShell :

```powershell
Push-Location frontend
node scripts/test-panel-mappings.mjs
npm run build
Pop-Location

Push-Location backend
go test ./...
go vet ./...
Pop-Location
```

Compiler l’interface avant les essais Go : la compilation régénère les fichiers intégrés à l’exécutable.

Pour rejouer le contrôle visuel, lancer `npm run dev -- --host 127.0.0.1` dans `frontend`, puis utiliser une session Playwright séparée depuis la racine :

```powershell
npx.cmd --yes --package @playwright/cli playwright-cli -s=panels open http://127.0.0.1:5173
npx.cmd --yes --package @playwright/cli playwright-cli -s=panels run-code --filename frontend/scripts/panel-ui-fixture.cjs
npx.cmd --yes --package @playwright/cli playwright-cli -s=panels run-code --filename frontend/scripts/panel-ui-review.cjs
npx.cmd --yes --package @playwright/cli playwright-cli -s=panels close
```

Le profil fictif peut associer une commande peu réaliste à un bouton pour vérifier le mécanisme d’édition ; ne pas utiliser ces données comme configuration de vol.

## 8. Suite et retour arrière

Après la vérification, les corrections ont été appliquées aux profils locaux et fournis. Les champs optionnels `state_on`/`state_off` et leur éditeur permettent les positions intermédiaires. Les commandes de pas sont désormais transmises sous forme `INC`/`DEC`, comme l’exige le processeur Lua installé, au lieu de nombres interprétés comme des positions absolues. Les nouveaux profils A-10C, JF-17 et AV-8B sont fournis dans [la bibliothèque](../profiles/panels/README.fr.md). Le script de mise à jour sauvegarde l’ancien fichier et conserve les attributions personnalisées non concernées.

La [vérification des quatre profils locaux](verification-profils-panels.fr.md) a ensuite comparé leurs identifiants et leurs positions aux métadonnées et modules Lua installés. Elle confirme une erreur d’attribution de la batterie du Hornet et des limites de sélection des volets, sans modifier les profils. Le script `tools/verify-panel-profiles.ps1` permet de reproduire les contrôles d’identifiants et d’interfaces.

L’exécutable `dcsmanager-panels.exe` est reconstruit avec cette interface. L’exécutable habituel `dcsmanager.exe` est conservé. Fermer toute autre instance avant de lancer la version de test. Les deux versions utilisent les mêmes données du gestionnaire ; exporter les profils avant les essais. Pour revenir à une ancienne version qui ignore les contextes et règles, restaurer aussi les profils précédents.

Restent à vérifier : tous les mouvements physiques, le sens du trim, les volets au centre, les valeurs LED réelles de l’avion et les cinq modes LCD. Des profils distincts par exemplaire USB identique ne sont pas encore implémentés. Aucun commit ni push n’a été demandé.

## 9. Profil F/A-18C et matériel confirmé (7 octobre 2026)

Les deux photos et fiches Logitech confirment PZ55/PZ70. Le journal fourni détecte des entrées mais ne signale aucun avion et désactive l’envoi. Le profil Hornet passe de 5 à 19 attributions ; les rockers HDG/CRS sont recentrés après une impulsion de 50 ms. L’éditeur expose ce relâchement et signale l’absence d’avion. Les fonctions disponibles, réaffectations, limites, tests et procédure de retour arrière sont détaillés dans [le suivi F/A-18C](integration-fa18-panels.fr.md). La migration locale sauvegarde les anciens profils et conserve les personnalisations.


## 10. Expérience vJoy, abandonnée (7 octobre 2026)

À la demande de l’utilisateur, une passerelle Windows x64 pour les PZ55/PZ70 avait été expérimentée. Cette passerelle et sa documentation d’installation ont été retirées après la décision d’abandonner vJoy. L’intégration actuelle est décrite dans [le plugin DCS Manager](plugin-panels-dcs.fr.md).

## 11. Plugin DCS Manager sans vJoy (7 octobre 2026)

Nouvelle décision de l’utilisateur : abandonner vJoy et tout pilote supplémentaire. Retrait de la passerelle, de son API et de l’écran associé. Ajout de `PanelCommands.lua`, chargé par Export.lua et installé avec les trois fichiers existants. Récepteur UDP local, commandes autorisées uniquement, session et accusés de réception ; pression du trim Hornet suivie d’un relâchement après au moins 50 ms, sans attendre dans une frame.

Les identifiants HOTAS ont été recherchés dans les fichiers DCS installés. `DCSM_PITCH_TRIM` rejoint le catalogue de l’éditeur uniquement pour le Hornet. La roue est inversée dans le profil pour suivre les directions physiques UP/DN. Les autres commandes et les trois LED restent DCS-BIOS. Le profil fourni et local passe à 20 attributions ; sauvegarde locale `data/mappings.before-profile-fixes-20261007-204527-939.json`.

Tests Go complets, analyse statique, vérification Lua 5.1 et essais Lua simulés réussis. Interface construite et essais navigateur avec API fictive. Exécutable `dcsmanager-panels.exe` reconstruit. Installation réelle non effectuée : prévisualisation uniquement. Essai en cockpit encore requis ; un accusé de réception ne prouve pas le mouvement du trim. [Architecture, protocole, limites, installation, essais et retour arrière](plugin-panels-dcs.fr.md).

## 12. Installateur Lua dans l’application (7 octobre 2026)

L’écran Installation DCS était jusqu’ici un état en lecture seule ; l’indication donnée précédemment à l’utilisateur était donc incorrecte. Ajout d’un bouton Installer / mettre à jour les scripts, de l’API POST `/api/scripts/install`, de la lecture actualisée de l’état sur disque et de la liste des résultats/sauvegardes. L’installateur utilise les fichiers embarqués et conserve une configuration utilisateur existante.

Vérification Windows du processus DCS.exe avant écriture, y compris au menu, et sérialisation des installations. Le dossier vient de la configuration détectée de l’application ; aucun chemin envoyé par le navigateur n’est accepté. Tests dans des dossiers temporaires pour exports/configuration/sauvegardes/répétition/blocage, puis interface avec API fictive. Aucun fichier Saved Games réel modifié pendant le développement. La procédure et les limites sont ajoutées dans le document du plugin.

## 13. Signalement de cabrage et isolement du trim

L’utilisateur signale un cabrage nouveau au manche. Cause non confirmée : le journal DCS de la dernière mission consultée ne montre pas le listener du plugin trim, et celui de l’application ne montre pas de DCSM_PITCH_TRIM envoyé. Les fichiers de commandes X56 consultés ne présentent aucune modification d’axe ; cela ne vérifie ni les valeurs physiques du joystick ni toutes les attributions par défaut.

Défauts identifiés dans le code : couper les envois ne vidait pas explicitement les impulsions Lua déjà acceptées ; un relâchement échoué était oublié après journalisation. Ajout de CANCEL, d’une activation trim indépendante et désactivée au démarrage, du relâchement lors de la fermeture de l’application et d’une tentative renouvelée en cas d’échec du relâchement. Tests de non-envoi sans activation, annulation et échec/reprise du relâchement. Vérification physique et essai sans application encore requis pour isoler le signalement initial.

## 14. Installateur Windows personnalisé (7 octobre 2026)

À la demande de l’utilisateur, ajout d’un Setup Inno par compte Windows, sans droits administrateur : logo et thème sombre, français/anglais, dossier/raccourcis, détection DCS/Saved Games avec correction, option Lua désactivée par défaut et reprise facultative d’une portable. La configuration sélectionnée est enregistrée et utilisée par l’application. Un marqueur à côté de l’exécutable distingue le mode installé ; ses données et son cache sont séparés du programme sous LOCALAPPDATA.

La copie portable passe par un dossier temporaire, conserve la source et refuse de remplacer les données installées existantes. Les cartes restent référencées dans leur dossier original. DCS Manager doit être fermé ; DCS doit aussi l’être pour l’option Lua. Les mises à jour et la désinstallation conservent les données et scripts DCS.

Premier essai silencieux bloqué par les champs de dossiers standard, qui refusaient les valeurs vides : remplacement par des champs facultatifs avec boutons Parcourir. Correction de l’INI de détection en UTF-16 pour les chemins accentués. Tests Go et analyse statique réussis ; cycle installation/réinstallation/désinstallation isolé réussi. Contrôle visuel de l’image de bienvenue ; contrôle interactif complet et essais DCS encore requis. Aucun vrai script DCS installé et aucune version utilisateur remplacée pendant ces essais. [Procédure, construction, tests, limites et retour arrière](installation-windows.fr.md).
