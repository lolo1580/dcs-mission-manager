# Changelog

[🇬🇧 English](CHANGELOG.md) | 🇫🇷 Français

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) et le projet adhère
au [versionnage sémantique](https://semver.org/lang/fr/).

## [Non publié]

### Modifié

- **Le projet est renommé DCS Manager.** Il a dépassé le cadre des missions, donc le
  nom, le binaire et tous les identifiants qui portaient l'ancien ont changé :
  `dcsmm.exe` → `dcsmanager.exe`, le module Go `dcsmm` → `dcsmanager`, le nom de la
  CLI, les variables d'environnement `DCSMM_*` → `DCSMANAGER_*`, la config Lua
  `dcsmm.cfg` → `dcsmanager.cfg` et le hook `Hooks/dcsmm.lua` →
  `Hooks/dcsmanager.lua`. Les marqueurs d'installation dans `Export.lua` sont
  désormais `DCSMANAGER-BEGIN` / `-END`.
  - Installation neuve : rien n'est repris de l'ancien nom, et aucune migration
    n'est fournie. `dcsmanager install-lua` écrit les nouveaux fichiers.

### Retiré

- **L'onglet Session a été retiré.** Il affichait les joueurs connectés, les
  événements de jeu en direct et le chat. Le gestionnaire ouvre maintenant sur
  **Débriefs**, le relevé durable d'un vol. Le backend reçoit et stocke toujours
  les événements, les joueurs et le chat : ils alimentent les statistiques et les
  débriefs, donc rien n'est perdu — seule la vue en direct disparaît.
  `ChatPanel.svelte`, `PlayerPanel.svelte`, `EventPanel.svelte` et les stores de
  session devenus inutiles ont été supprimés, et le bundle passe d'environ
  204 Ko à 187 Ko.
- **Les couches de compatibilité avec nos propres anciennes versions sont parties.**
  Maintenant que le projet démarre à neuf, le code qui n'existait que pour lire des
  artefacts produits par une ancienne version de chez nous est du poids mort :
  - **Les alias de théâtre** (`Marianas` → `MarianaIslands`, `Sinai` → `SinaiMap`)
    corrigeaient des identifiants que nous avions publiés faux. `theatre.Resolve`
    disparaît et seuls les identifiants de DCS sont acceptés.
  - **La section « restes de l'ancien nom »** de l'onglet Installation DCS, et la
    liste `legacy` derrière elle, disparaissent : il n'y a pas d'ancienne
    installation à nettoyer.
  - Le paragraphe du README sur la migration depuis une version antérieure à la
    colonne `source` disparaît aussi.
  - Ce qui est conservé, c'est la **robustesse face à l'entrée externe**, pas face à
    notre passé : un fichier DCS malformé, une archive illisible ou un
    `options.lua` édité à la main dégradent toujours en silence, et les migrations
    additives de la base tournent toujours.
- **La carte temps réel a été retirée.** Le gestionnaire se concentre désormais
  sur la session (joueurs, événements, chat), les débriefs, les statistiques,
  l'analyse et les aérodromes. Tout ce qui n'existait que pour servir la carte
  disparaît : la vue cartographique et ses filtres, les fonds de carte
  (satellite, relief, routier, aéronautique, sombre), l'imagerie F10 importée
  (importateurs MBTiles/image/jeu de tuiles et dossier `tiles/`), les vecteurs de
  terrain DCS (`vectors/`), le contour d'étendue, les routes API de tuiles et de
  vecteurs, et le filtrage fog of war.
  - La télémétrie des unités est **toujours reçue et échantillonnée** : elle
    alimente les traces de vol, les heatmaps et l'analyse de sortie, ainsi que
    la recherche de l'aérodrome le plus proche. Seul le dessin disparaît.
  - Les aérodromes, leurs fréquences et leurs **cartes aéronautiques** (lues
    depuis `maps_dcs/`) sont inchangés.
  - `dcsmanager import-tiles`, `import-image`, `fetch-tiles` et `import-vectors`
    disparaissent, ainsi que les variables `DCSMANAGER_TILES_DIR`,
    `DCSMANAGER_TILES_ATTRIBUTION`, `DCSMANAGER_VECTORS_DIR`, `DCSMANAGER_BASEMAP`,
    `DCSMANAGER_BASEMAP_URL` et `DCSMANAGER_REVEAL_ALL_UNITS`. Leaflet n'est plus une
    dépendance du frontend.

### Ajouté

- **Un logo.** L'en-tête affiche désormais l'emblème DCS Manager à côté du titre,
  et le README s'ouvre dessus. L'art source vit dans `Logo/` ; les copies prêtes
  pour le build sont `frontend/src/assets/logo.png` (l'emblème transparent,
  128 px, redimensionné et embarqué dans l'UI) et `docs/logo.png` (l'emblème à
  fond sombre pour le README). Les originaux font 1,2 Mo chacun : ils ne sont
  jamais livrés ni servis directement, et `Logo/` est ignoré par git (seuls les
  dérivés optimisés sont versionnés).
- **L'exécutable porte désormais une icône et les métadonnées de version.**
  Windows tire l'icône d'un programme (et le FileDescription / ProductName / la
  version affichés dans la barre des tâches, Alt-Tab et les Propriétés du
  fichier) d'un objet de ressources lié au binaire.
  `backend/cmd/dcsmanager/rsrc_windows_amd64.syso` le fournit : l'emblème en
  16/32/48/64/128/256 px plus le bloc de version, donc `dcsmanager.exe` ressemble
  à une application et non à un binaire générique. Le `.syso` est versionné, donc
  un `go build` ordinaire l'embarque sans outil supplémentaire ; régénère-le avec
  `.\build.ps1 -Target winres` (ou `make winres`) après un changement d'icône ou
  de `winres/winres.json`, ce qui demande
  `go install github.com/tc-hib/go-winres@latest`.
- **Le volet matériel de cockpit du DCS Panel Manager fait désormais partie du
  gestionnaire.** Un Switch Panel PZ55 et un Multi Panel PZ70 de Logitech/Saitek
  peuvent être pilotés directement, et DCS-BIOS est parlé plutôt que réimplémenté :
  un cockpit qui tourne déjà avec lui n'a besoin d'aucun second outil. Cela a
  absorbé une application C#/Avalonia séparée dans celle-ci, en Go, en gardant le
  build binaire unique `CGO_ENABLED=0`.
  - **`internal/hid`** lit les périphériques HID Windows sans cgo : énumérer,
    ouvrir, lire les capacités, lire les rapports avec un délai, écrire des
    rapports de sortie. Quatre pièges ont dû être trouvés sur du vrai matériel — le
    chemin d'interface est à l'offset 4 (pas 8) dans
    `SP_DEVICE_INTERFACE_DETAIL_DATA_W`, `SetupDiEnumDeviceInterfaces` prend cinq
    arguments, `HidP_GetCaps` veut les *preparsed data* et non le handle du
    périphérique (passer le handle fait planter le processus), et le handle doit
    être ouvert avec `FILE_FLAG_OVERLAPPED`, sinon `ReadFile` bloque indéfiniment
    sur un panneau inactif.
  - **`internal/panel`** décode et encode le protocole PZ55/PZ70 : interrupteurs,
    boutons et encodeurs (rapportés sur le front montant, donc un cran = un
    événement), LEDs de train, LCD et LEDs d'autopilote.
  - **`internal/panelservice`** entretient un lecteur par panneau, gère le
    branchement à chaud et publie les entrées sous forme d'événements. Un panneau
    occupé par un autre outil, ou une lecture qui échoue en boucle, est signalé
    plutôt que fatal.
  - **`internal/dcsbios`** décode le flux d'export de DCS-BIOS (le sync `0x55` et
    ses blocs adresse/longueur/données), resynchronise après un paquet perdu,
    extrait l'appareil actif de l'image mémoire, et renvoie des commandes.
  - **`internal/biosmeta`** lit le catalogue de commandes que DCS-BIOS publie par
    appareil, et **`internal/mapping`** associe un contrôle de panneau à une de ces
    commandes. L'envoi est **désactivé par défaut et de nouveau à chaque
    redémarrage** : piloter un cockpit vivant doit être délibéré, et ne doit pas
    survivre à un redémarrage sans qu'on le sache.
  - Un onglet **Panneaux de cockpit** affiche la liaison DCS-BIOS, une carte par
    panneau, un moniteur temps réel des entrées, et un éditeur d'associations (286
    commandes associables sur le F-16C, lues depuis ses propres métadonnées).
  - La télémétrie du gestionnaire passe sur **UDP 7776** pour cohabiter avec
    DCS-BIOS, qui possède 7778.

- **Un onglet Configuration affiche les réglages de DCS lui-même.** Le gestionnaire
  lit `Saved Games\DCS\Config` et présente les options du jeu telles que DCS les
  stocke : les groupes d'`options.lua` (graphismes, difficulté, VR, son, vues,
  cockpit, divers, plugins) avec leurs clés/valeurs, les **terrains et modules
  désactivés** d'après `pluginsEnabled.lua` (ce qui explique souvent une carte qui
  refuse de se charger), la langue de l'interface et le dossier inspecté.
  - Chaque section se parcourt séparément ; `GET /api/config?section=graphics` en
    renvoie une, `GET /api/config` le tableau complet.
  - Les booléens s'affichent `on`/`off` plutôt que `true`/`false`, et une table
    imbriquée est montrée par son nombre d'éléments, donc rien n'est perdu en
    silence.
  - Couvert par un test lisant le vrai dossier `Config` de la machine quand il est
    présent, un `options.lua` + `pluginsEnabled.lua` synthétiques, l'aide de
    formatage des valeurs, et les cas vide, filtré et section inconnue de l'API.

- **Un onglet Installation DCS montre ce qui vit du côté du jeu.** Il rapporte les
  mods installés dans `Saved Games\DCS\Mods` et l'état des scripts, pour qu'une
  installation cassée ou à moitié faite soit visible au lieu de silencieuse.
  - **Mods** : catégorie, nom, nombre de fichiers et taille, et si le mod livre un
    `entry.lua` (son absence signale un mod incomplet). La taille totale est
    additionnée.
  - **Scripts du gestionnaire** : chaque fichier géré, à jour / périmé / non installé.
  - **Export.lua** : quels autres outils le partagent (Tacview, DCS-BIOS, SRS, LotAtc,
    VAICOM, BattleHub…), détectés d'après les lignes `dofile`/`require`.
  - Un bandeau résumé signale quand un fichier géré demande attention.
  - `internal/dcsdata` lit l'arbre Mods et inspecte le dossier Scripts ; il réutilise
    le statut de `internal/install` pour les fichiers gérés au lieu de dupliquer la
    comparaison. `GET /api/mods` et `GET /api/scripts` l'exposent.
  - Couvert par des tests construisant un arbre Mods et un dossier Scripts dans un
    dossier temporaire (y compris les sauvegardes, et l'exclusion de nos propres
    fichiers de la liste des tiers), plus les cas vide et rempli de l'API.

- **Un onglet Missions est une bibliothèque des `.miz` enregistrés dans Saved
  Games.** Un `.miz` est un ZIP ; le gestionnaire lit les données d'éditeur qu'il
  contient et liste chaque mission avec son **théâtre**, sa **date et son heure de
  début** en jeu, sa **météo et sa température**, sa **taille** et sa dernière
  modification — sans lancer le jeu.
  - `internal/dcsdata` ouvre l'archive, ne lit que l'entrée `mission` (bornée, pour
    qu'une archive hostile ne puisse pas épuiser la mémoire) et analyse les
    métadonnées avec le parseur Lua existant. Une archive cassée ou vide est
    quand même listée ; un dossier absent donne une liste vide, jamais une erreur.
  - `GET /api/missions` renvoie la bibliothèque ; `?theatre=` la filtre.
  - Testé en fabriquant un `.miz` en mémoire, sur les vraies missions de la machine,
    et sur les cas dossier absent et API vide.

- **Un onglet Carrière affiche le logbook du joueur.** DCS tient un carnet de
  carrière dans `Saved Games\DCS\MissionEditor\logbook.lua` ; le gestionnaire le lit
  désormais et affiche le **grade**, l'**escadrille**, les **décorations** et
  l'**invulnérabilité** du pilote, les totaux de carrière (heures de vol, missions,
  atterrissages, score), et un **détail par appareil** : heures de vol,
  atterrissages, morts, éjections et kills air-air / air-sol, avec une barre à
  l'échelle de l'appareil le plus volé.
  - `GET /api/career` renvoie les profils ; l'onglet permet de basculer entre eux
    quand une machine en a plusieurs.
  - C'est le carnet du pilote tel que DCS le tient lui-même, en complément des
    statistiques que le gestionnaire construit à partir de ses propres sessions.
  - Couvert par un test sur le vrai `logbook.lua` de la machine quand il est présent,
    un document synthétique, et les cas vide et rempli de l'API.

- **Un onglet Modules liste ce que DCS lui-même déclare installer.** Le gestionnaire
  lit l'inventaire de DCS (`Saved Games\DCS\MissionEditor\modules.lua`) au lieu d'un
  catalogue maintenu à la main : terrains, appareils, systèmes de navigation, packs
  techniques, campagnes et lots, chacun avec son **type**, son développeur, son id
  DCS et les **versions installées**, et s'il est **possédé**.
  - `internal/dcsdata` analyse le fichier avec le parseur Lua existant (aucune
    nouvelle dépendance) et normalise les sections que DCS écrit tantôt en listes,
    tantôt en tables numérotées. Il dégrade proprement : un fichier absent donne un
    inventaire vide, pas une erreur.
  - `GET /api/modules` renvoie la liste, avec `?owned=1` pour le sous-ensemble
    possédé.
  - L'onglet filtre par **recherche** (titre, développeur, id), par **catégorie** et
    par **possédés uniquement**, et affiche le compte possédés/total. Sur la machine
    de test il trouve 183 entrées, 22 possédées, 17 terrains dont 5 possédés.
  - Couvert par un test sur le vrai `modules.lua` de la machine quand il est présent,
    plus un document synthétique et les cas vide et filtré de l'API.

- **Le chat peut désormais être envoyé dans DCS (canal de commandes).** La liaison
  TCP était déjà bidirectionnelle par construction ; le backend écrit maintenant des
  commandes sur la même connexion que celle utilisée par le hook pour remonter, et
  le hook les exécute.
  - `POST /api/chat` ne répond plus `501` : il remet une ligne
    `{"type":"command","command":"chat",…}` au listener TCP, qui la diffuse à tous
    les hooks connectés. Il répond `503` quand aucun hook n'est connecté (jeu non
    lancé, ou scripts non installés) — un état normal, affiché comme tel dans
    l'interface, pas une erreur.
  - `Hooks/dcsmanager.lua` lit les commandes en attente depuis `onSimulationFrame`, avec
    un **délai nul**, pour ne jamais bloquer une image, et injecte le message via
    `net.send_chat`. Une commande inconnue est journalisée et ignorée, donc un
    backend plus récent ne peut pas casser un hook plus ancien.
  - `internal/tcp` suit ses connexions et expose `Connected()` / `SendCommand()`,
    derrière une petite interface `api.Commander` pour que la couche HTTP reste
    indépendante du transport.
  - Couvert par des tests des deux côtés : la diffusion backend et le cas « aucun
    hook », et les réponses `503`/`200`/`400` de `POST /api/chat`.

- **L'onglet Analyse dessine ses données au lieu de seulement lister des
  chiffres.** La carte disparue, la heatmap s'était réduite à un compteur et les
  stats de sortie à un tableau. L'analyse dispose maintenant d'un **graphique vu
  de dessus**, en SVG pur (aucune bibliothèque cartographique) : la heatmap en
  cellules de grille colorées (du bleu au rouge, à l'échelle racine carrée pour
  que quelques cellules denses ne masquent pas le reste) et les trajectoires
  enregistrées en polylignes, sur une même emprise avec les coordonnées des coins.
  - Le graphique est **sans projection mais non déformé** : un degré de longitude
    est mis à l'échelle par `cos(latitude)`, donc l'image garde ses vraies
    proportions — l'écart est important sur Kola ou The Channel, où un rendu carré
    naïf étire la carte horizontalement.
  - Deux interrupteurs (densité, trajectoires) activent chaque calque ; leur état
    est mémorisé entre les redémarrages. Basculer entre « Trafic » et « Pertes »
    ne recharge que la heatmap, en laissant les trajectoires en place.
  - Le tableau de sortie gagne une **barre de distance** par ligne, à l'échelle de
    la plus longue sortie, pour lire le classement d'un coup d'œil.
  - Les helpers de projection vivent dans `frontend/src/lib/plots.js` et traitent
    explicitement les cas dégénérés : un point unique, une trajectoire parfaitement
    rectiligne et un résultat vide se dessinent sans division par zéro.

- **Le manager s'ouvre désormais dans sa propre fenêtre, et non dans un onglet de
  navigateur.** Lancer `dcsmanager.exe` affiche l'UI embarquée dans une fenêtre WebView2
  native : plus de navigateur à ouvrir, plus de `localhost:8080` à retenir, et
  fermer la fenêtre arrête le manager. La fenêtre s'appuie sur
  `github.com/jchv/go-webview2`, une liaison **pur Go** : le build n'a toujours
  besoin d'aucune chaîne C (`CGO_ENABLED=0` inchangé).
  - Le serveur HTTP est conservé, pas remplacé : la fenêtre charge `http://<addr>/`,
    la page atteint donc l'API exactement comme avant, et `dcsmanager serve` expose
    toujours l'UI à un navigateur (second écran, tablette). Le manager bascule
    seul dans ce mode sans fenêtre si le runtime WebView2 est absent.
  - `DCSMANAGER_HTTP_ADDR` accepte maintenant un port `0` : le système en choisit un
    libre et la fenêtre pointe sur l'adresse résolue.
  - Le câblage du manager est passé dans `internal/app`, pour que la fenêtre et le
    mode sans interface partagent une seule implémentation.

- **L'Allemagne Guerre froide a désormais son jeu de données embarqué.** Le
  manager lit déjà les aérodromes de chaque carte installée depuis les fichiers de
  DCS, et l'Allemagne Guerre froide ne fait pas exception (119 aérodromes, avec
  Tower/TACAN/ILS/NDB). Ce qui manquait, c'était le repli hors ligne : sans DCS
  trouvé, le binaire retombait sur le seul jeu du Caucase.
  `internal/aerodrome/data/germanycw.json` couvre maintenant ce théâtre aussi,
  pour que l'onglet Aérodromes reste utile sans installation.
  - Le fichier est **généré depuis DCS**, pas saisi à la main :
    `go run ./cmd/gen-aerodrome <Mods/terrains> <Théâtre> <sortie.json>` extrait
    `radio.lua` et `beacons.lua` et écrit le jeu pour n'importe quel théâtre.
  - DCS donne une fréquence Tower à chaque aérodrome mais une position seulement
    quand une aide à la navigation existe : **77 des 119 aérodromes ont des
    coordonnées, 42 n'en ont pas.** L'interface affiche maintenant « non
    positionné par DCS » au lieu d'écrire `0.0000°` comme si c'était un lieu, et
    une coalition vide se lit « inconnue ».
  - Un test dédié verrouille le jeu (119 aérodromes, le TACAN et la position de
    Francfort, 70+ plaçables) ; le test d'intégrité n'exige plus de coordonnées
    pour chaque aérodrome, ce que DCS ne peut pas fournir.

### Corrigé

- **Les endpoints de statistiques paniquaient sans base de données.** Avec la
  persistance désactivée (`DCSMANAGER_DB_ENABLED=false`, le mode des outils de
  test), `stats.New` renvoie un service non nul mais dont la base est nulle : le
  garde ne testait que `stats == nil`, jamais la base, et chaque route de stats
  déréférençait un `*db.DB` nil. Elles répondent maintenant `{"enabled": false}`
  en 200, et l'onglet affiche un message clair au lieu d'un tableau vide ou d'une
  erreur. Couvert par un test sur les cinq routes.
- **Les heures de vol de la Carrière étaient complètement faussées.** L'onglet
  Carrière affichait des milliers d'heures pour quelques vols (5 477 h pour
  6 sorties), parce que DCS stocke les champs `flightHours`, `daytime` et
  `nighttime` en **secondes** malgré leur nom — un travers bien connu de DCS (le
  jeu compte le temps en secondes, donc une valeur brute se lit comme des
  milliers d'« heures »). Le manager affichait le nombre brut comme s'il
  s'agissait d'heures.
  - Le parseur du logbook divise désormais ces trois champs par 3 600, pour les
    lignes par appareil comme pour les totaux de carrière, donc tout le payload
    est cohérent en heures. Le même joueur affiche maintenant **1,5 h**
    (M-2000C 1,1 h), ce qui correspond aux vols.
  - Un test sur le `logbook.lua` réel de la machine rejette tout appareil
    au-dessus de 5 000 h (signe d'un reste en secondes) et vérifie que le total
    égale la somme des appareils ; le document synthétique utilise lui aussi des
    secondes.
- **L'onglet Modules confondait « possédé » et « installé ».** Le
  `MissionEditor/modules.lua` de DCS est le catalogue du magasin : `have="1"`
  signifie que le joueur a *acheté* le module, pas qu'il est sur le disque. Une
  carte achetée puis **désinstallée pour libérer de la place garde `have="1"`
  indéfiniment**, donc l'onglet continuait de l'afficher comme installée — Kola et
  le Golfe Persique, par exemple, après les avoir désinstallés pour faire de la
  place à l'Allemagne Guerre froide.
  - Le manager lit désormais aussi **`autoupdate.cfg`** à la racine du jeu, la
    liste par DCS des modules présents dans l'installation (`GERMANYCW_terrain`,
    `CAUCASUS_terrain`…), et apparie chaque module sur tous ses identifiants
    (`modulId`, `update_id`, `code`). Les deux notions sont affichées séparément :
    **Possédé** et **Installé**.
  - Seules les unités de contenu (terrains, appareils) portent un état
    d'installation ; une campagne ou un lot vient avec un module et affiche
    « n/d » plutôt qu'une valeur devinée. Quand aucune installation n'est trouvée,
    l'état reste inconnu au lieu d'être supposé.
  - Un nouveau filtre **« Installés uniquement »** rejoint « Possédés
    uniquement » ; `/api/modules?installed=1` le sert. Couvert par des tests
    utilisant les fichiers de cette machine.
- **Passe de recherche de bugs sur le backend, les scripts Lua et l'interface.**
  Les trouvailles les plus lourdes, toutes vérifiées contre la doc API réellement
  installée de DCS (`API/Sim_ControlAPI.md`) ou les fichiers sur disque :
  - **Chaque message de chat en jeu était perdu.** DCS passe à `onChatMessage`
    un id joueur **numérique** comme `from`, or le backend décode `from` en
    chaîne : `json.Unmarshal` échouait sur toute la ligne et le message était
    jeté en silence (vue live et base). Le hook résout maintenant le nom du
    joueur (repli sur l'id en texte), donc le chat n'est plus perdu.
  - **Le fichier de configuration n'était jamais appliqué.** `dcsmanager.cfg`
    commençait par des commentaires `#`, or il est chargé par `loadfile()` et
    doit être du Lua valide ; `#` est une erreur de syntaxe, donc tout le bloc
    était rejeté et **tous** les réglages — dont `dcsmanager_host` et les ports —
    étaient ignorés en silence. Les commentaires sont désormais `--`,
    `tools/check-lua.mjs` parse la config (un test la verrouille), et la copie
    embarquée a été régénérée.
  - **Tous les minuteurs périodiques du hook étaient bloqués.**
    `LoGetModelTime` n'est pas un global dans l'état Lua des Hooks (l'API export
    vit dans le namespace `Export.`), donc `t` valait toujours 0 et les minuteurs
    « joueurs », « types de slots » et « lecture des commandes backend » ne se
    déclenchaient jamais : `POST /api/chat` ne pouvait pas atteindre DCS. Le hook
    appelle maintenant `Export.LoGetModelTime`.
  - **Les stats d'armes, de victimes et de types de tueurs étaient vides.**
    `onGameEvent` était déclaré avec quatre paramètres, mais DCS en passe jusqu'à
    sept (`kill` = id/type/camp du tueur, id/type/camp de la victime, arme). La
    fin — dont le nom de l'arme — était perdue. Tous les arguments sont
    maintenant transmis.
  - **Le théâtre de la mission n'était jamais envoyé,** donc chaque mission était
    enregistrée comme « Caucasus » quelle que soit la carte. Le hook le lit
    désormais via `Sim.getCurrentMission()`.
  - **Le chat du canal de commandes n'atteignait que la coalition du serveur.**
    `net.send_chat` exige `(message, true)` pour diffuser ; sans le second
    argument, c'est limité au camp.
  - **Les cartes aéronautiques étaient inaccessibles.** Rien ne sélectionnait les
    cartes d'un aérodrome ni n'ouvrait la visionneuse : toute la fonctionnalité
    était du code mort. Le volet de détail charge maintenant les cartes à la
    sélection et les ouvre dans la visionneuse.
  - **L'extraction TACAN/VOR embarquée pouvait garder la mauvaise aide.** Le VOR
    d'un aérodrome était écrasé par un `world_*` rattaché par nom ; il garde
    désormais le premier (celui de l'aérodrome).
  - **Courses UI et données périmées :** sélectionner un débrief, changer la
    portée des stats ou la source de la heatmap pouvait faire arriver une
    ancienne réponse en dernier et afficher les mauvaises données ; les réponses
    périmées sont maintenant écartées. Changer de théâtre ne garde plus
    l'aérodrome de la carte précédente sélectionné, « Proches de moi » rafraîchit
    le badge de source, un statut HTTP en erreur remonte comme erreur au lieu
    d'un résultat vide, et la substitution i18n ne casse plus les valeurs
    contenant `$&`.
  - **Les outils de test plantaient sur un mauvais argument.** `node
    send-telemetry.mjs host abc` levait un `ERR_SOCKET_BAD_PORT` non rattrapé ;
    un port ou une durée invalide est désormais refusé avec un message clair, et
    les erreurs UDP sont gérées.
  - Suppression de deux déclarations mortes signalées par staticcheck ; un test
    ne contient plus d'affectation « valeur jamais utilisée ». `staticcheck ./...`
    et `go vet ./...` sont maintenant propres.
- **Un build non-Windows inondait d'erreurs de panneaux.** `hid.Enumerate`
  renvoie `ErrUnsupported` sur une plateforme sans support HID Windows, et le
  service de panneaux le publiait comme événement d'erreur à chaque cycle — l'UI
  se remplissait de « hid: only supported on Windows ». Le scanneur traite
  désormais ce signal comme « aucun panneau ici », arrête de scanner, et ne le
  remonte jamais comme erreur. C'est ce qui faisait échouer le `go test -race ./...`
  sous Linux : `TestServiceStartsAndStops` exigeait l'absence d'événement
  d'erreur. Couvert par un test indépendant de la plateforme qui injecte l'erreur
  « non supporté ».
- **Deux sortes de TACAN manquaient dans les données d'aérodrome.** Les deux ont
  été trouvées sur l'Allemagne Guerre froide, et les deux faisaient disparaître
  le TACAN silencieusement :
  - DCS livre deux variantes de TACAN : `BEACON_TYPE_TACAN` (souvent couplé à un
    VOR) et `BEACON_TYPE_AIRPORT_TACAN`, l'équipement propre à l'aérodrome. Seul
    le premier était géré, donc **Nordholz (118X NDO)** perdait son TACAN, et
    était même affiché comme un NDB car l'entrée non reconnue matchait un cas de
    repli.
  - Certaines aides sont modélisées comme une balise `world_*` **sans id
    d'aérodrome**, qui nomme l'aérodrome dans `display_name`. Le lecteur de
    balises écartait toute entrée `world_*`, donc les VORTAC de **Hamburg
    (78X HAM)** et **Fulda (58X FUL)** n'atteignaient jamais leur aérodrome. Une
    balise `world_*` **nommée** est désormais conservée et rattachée à
    l'aérodrome du même nom ; une balise sans nom reste ignorée.
  - L'Allemagne Guerre froide compte maintenant **22** aérodromes avec TACAN au
    lieu de 19, et le jeu embarqué a été régénéré. Le Caucase est inchangé
    (6 TACAN, 5 VOR, 4 RSBN) ; couvert par un test qui soumet les deux cas à
    l'extracteur.
- **La fenêtre native pouvait faire planter tout le manager au démarrage.** Le
  composant WebView2, ses objets COM et sa boucle de messages doivent vivre sur
  **un seul** thread système, or la fenêtre était créée depuis une goroutine que
  Go peut déplacer d'un thread à l'autre : la fonction de rappel s'exécutait alors
  sur un autre thread et déréférençait un objet à moitié initialisé, ce qui tuait
  le processus (violation d'accès). La goroutine de la fenêtre est désormais
  verrouillée sur son thread (`runtime.LockOSThread`). Vérifié sur cinq
  lancements consécutifs.
- **`SouthEastAsia` était proposé comme théâtre, or DCS n'a pas ce terrain.** La
  liste des théâtres contenait une entrée qui n'est pas l'un des 14 terrains
  vendus par DCS : l'UI annonçait donc une carte qui ne peut pas être volée. La
  liste correspond désormais exactement à celle de DCS (Caucasus, Syria, Nevada,
  Persian Gulf, Marianas, Marianas WWII, Sinai, Kola, Afghanistan, Iraq, South
  Atlantic → `Falklands`, Normandy, The Channel, Cold War Germany), et un test
  refuse tout identifiant que DCS n'a pas.
- **La plupart des aides à la navigation de DCS manquaient dans les données
  d'aérodrome.** Le lecteur de balises ne gérait que
  `BEACON_TYPE_AIRPORT_HOMER` et quelques types de VOR : tout
  `BEACON_TYPE_HOMER`, `BEACON_TYPE_ILS_FAR_HOMER`,
  `BEACON_TYPE_ILS_NEAR_HOMER`, `BEACON_TYPE_VORTAC` et `BEACON_TYPE_DME` du
  `beacons.lua` de DCS était donc ignoré en silence. Les homers simples et les
  marqueurs externe/interne d'ILS sont des radiobalises non directionnelles (un
  ADF, ou les marqueurs d'un ILS), et un VORTAC est un VOR qui porte aussi un
  TACAN : tous ont leur place sur la fiche. Mesuré sur les cinq cartes installées :
  - NDB : Caucase 0 → **47** (sur 18 aérodromes), Golfe persique 0 → **10**,
    Kola 0 → **1**, Marianas 0 → **1**.
  - VOR : Golfe persique 1 → **16**, Kola 4 → **9**.
  - TACAN : Golfe persique 8 → **10** (les VORTAC).
- **Lancer le manager deux fois échouait en silence.** Un second `dcsmanager.exe`
  échouait au bind UDP et se terminait, en écrivant dans une console que
  l'utilisateur ne voit jamais en double-clic : on avait l'impression que rien ne
  se passait. Le manager interroge désormais `GET /api/health` sur son propre nom
  de service avant de démarrer : en mode fenêtre, il affiche un message et
  s'arrête ; `dcsmanager serve` sort avec le code 3 et une raison en une ligne. Un
  double lancement le dit maintenant au lieu de disparaître.
- **Le mode fenêtre n'écrivait aucun journal.** Un exécutable lancé au
  double-clic n'a pas de console : un échec au démarrage ne laissait donc aucune
  trace. Le mode fenêtre écrit désormais dans `data/dcsmanager.log` (à côté de
  `DCSMANAGER_DB_PATH`), là où « ça ne démarre pas » trouve enfin une réponse.

- **Le débrief et le message de fin de mission étaient perdus : `conn:send`
  n'écrivait qu'une partie de la ligne, et l'écriture partielle passait pour un
  succès.** Le `send` de LuaSocket peut n'écrire qu'une fraction du tampon tout en
  renvoyant une valeur : le test `if not ok` laissait donc passer, alors que le
  reste de la ligne — dont le retour à la ligne final — était abandonné. Le
  backend, qui lit du JSON délimité par des retours à la ligne, jetait la ligne
  incomplète comme malformée. Le journal affichait « debrief sent » alors que le
  message avait seulement été *tenté*.
  - Les envois bouclent désormais jusqu'à ce que tous les octets soient écrits, et
    un échec réel est signalé dans `dcs.log` au lieu d'être annoncé comme envoyé.
    Reproduit de bout en bout : le même débrief envoyé à la main sur le même
    socket est bien stocké, ce qui a permis d'identifier le transport, et non le
    parseur, comme coupable.
- **Une installation par-dessus un ancien marqueur ajoutait un second bloc au
  lieu de mettre à jour le premier.** Un fichier écrit par un installeur
  précédent portait un marqueur dont la ponctuation avait été abîmée par un
  aller-retour d'encodage (le tiret cadratin était devenu `â€"`) : la recherche
  octet pour octet ne le reconnaissait plus et le bloc était ajouté de nouveau.
  Les deux blocs définissaient les mêmes globales Lua, et l'ancien gardait le bug
  `setpayloadsize`. L'installeur reconnaît maintenant un bloc par son mot-clé sur
  une ligne de commentaire, fusionne les doublons en un seul bloc, conserve le
  contenu qui les sépare, et retire tous les blocs à la désinstallation. Couvert
  par des tests, dont un vérifiant qu'un mot-clé simplement mentionné dans du code
  utilisateur n'est jamais pris pour une borne de bloc — une erreur ici
  supprimerait du vrai contenu.
- **La carte live ne recevait rien : `setpayloadsize` n'existe pas dans la
  LuaSocket de DCS.** La séquence de connexion était `socket.udp()` →
  `setpayloadsize(65507)` → `setpeername(host, port)`. L'appel du milieu lève une
  erreur, et comme la connexion passait par un `pcall`, l'erreur était avalée :
  **`setpeername` n'était jamais atteint**, le socket n'avait donc aucune
  destination et chaque envoi échouait en silence. Vérifié dans
  `bin/lua-socket.dll`, où le symbole est absent.
  - Le socket est désormais configuré défensivement, une connexion ratée est
    signalée dans `dcs.log` au lieu d'être cachée, et un échec d'envoi est
    journalisé plutôt qu'avalé. C'est ce silence qui a coûté une session entière.
  - Les messages « world » sont découpés en lots sous la limite du datagramme,
    puisque la taille de charge utile par défaut s'applique et qu'un gros message
    échoue d'emblée.
  - `tools/check-lua.mjs` analyse les scripts Lua avant une session : une erreur de
    syntaxe est attrapée ici au lieu de coûter un redémarrage du jeu.
- **Le bandeau de pause cassait toute la mise en page.** L'ajouter comme
  quatrième enfant d'une grille qui n'en déclarait que trois lignes le plaçait sur
  une ligne implicite : la zone principale se retrouvait dimensionnée sur son
  contenu et le bandeau s'étirait sur toute la page. La mise en page est
  désormais une colonne flex, qui gère n'importe quel nombre de bandeaux
  optionnels — ce qu'une grille à nombre de lignes fixe ne peut pas faire.
  Vérifié avec les deux bandeaux affichés : en-tête 48 px, chaque bandeau 32 px,
  carte 1133 px, total exactement la hauteur du viewport.
- **Les options de vue de la mission n'atteignaient jamais le backend : seule ta
  propre coalition était donc affichée.** Deux bugs indépendants dans la même
  chaîne, tous deux trouvés en lançant le jeu pour de vrai :
  - le hook appelait `Sim.getMissionOptions`, qui **n'existe pas**. La bonne API
    est `DCS.getMissionOptions` (`MissionEditor/GameGUI.lua`).
  - le backend cherchait ensuite `optionsView` à la racine de la table, alors que
    DCS l'imbrique sous `difficulty`. Il restait donc sur son défaut restrictif,
    ce qui explique exactement pourquoi seuls les alliés — et en fait seulement
    ton propre appareil — apparaissaient.
  Le hook appelle désormais la bonne API, journalise la source utilisée, et le
  backend accepte les deux formes, imbriquée et à plat. Couvert par
  `TestOptionStringNested` et `TestApplyMissionOptionsSetsMode`.
- **Le débrief n'était jamais envoyé.** Le hook cherchait base64 dans
  `socket.base64`, qui ne fait pas partie de LuaSocket : il se trouve dans le
  module `mime`. Le journal le disait clairement (« base64 encoding unavailable,
  debrief not sent ») et le code ne se rabattait sur rien. `mime` est maintenant
  requis et utilisé en premier.
- **Mettre DCS en pause enregistrait toutes les unités comme détruites.**
  `LuaExportActivityNextEvent` n'est appelé que tant que le temps de simulation
  avance : une pause arrête donc complètement la télémétrie. Le backend lisait ce
  silence comme « toutes les unités ont disparu » et écrivait un lot de pertes à
  chaque pause — polluant l'analyse et la carte des pertes. Le tracker
  n'échantillonne plus rien et ne déclare plus aucune perte tant que le flux est
  arrêté, et conserve son état de suivi pour que la carte reprenne où elle
  s'était arrêtée. Couvert par `TestPausedFeedDoesNotReportLosses`.
  - L'état est désormais exposé au lieu d'être muet : la trame de session porte
    `paused` et `feedAgeMs`, et la carte affiche un bandeau expliquant que la
    simulation n'avance pas. Sans cela, un jeu en pause ressemblait à une
    application cassée — exactement la façon dont le problème a été signalé.
- **Les étendues des cartes étaient fausses pour tous les théâtres, et sont
  désormais mesurées au lieu d'être devinées.** Les bornes étaient des littéraux
  saisis à la main, et la comparaison avec les données de DCS montre qu'ils
  étaient faux partout : Kola couvre en réalité 11,7 à 40,1 degrés de longitude
  là où le littéral indiquait 19 à 34, les Mariannes atteignent la latitude 20,7
  là où le littéral s'arrêtait à 15,6, et le golfe Persique commence à 51,0 là où
  le littéral disait 47. L'étendue est maintenant calculée à partir des
  aérodromes et des localités du théâtre : elle est exacte et suit DCS quand une
  carte est mise à jour. Vérifié : aucun aérodrome ne tombe hors de la boîte de
  son théâtre.
  - Les bornes codées en dur ne servent plus que de repli pour une carte non
    installée, où il n'y a rien à mesurer.
- **Le terrain Marianas WWII était indexé sous le mauvais identifiant.** DCS
  livre le dossier sous `MarianasWWII` mais déclare le théâtre
  `MarianaIslandsWWII`, et le nom de dossier servait d'identifiant : le théâtre
  rangeait donc ses aérodromes sous un nom que l'interface ne demande jamais.
  L'identifiant déclaré dans `entry.lua` fait désormais foi, ce qui donne aussi à
  cette carte une étendue mesurée et rend ses 11 aérodromes accessibles.
- **Quatre défauts trouvés par une revue systématique**, aucun visible en usage
  normal :
  - **Un fichier de données tronqué pouvait faire planter le backend.** Le
    wrapper gettext `_(` est la seule forme de valeur qui atteignait le parseur de
    chaîne sans guillemet garanti : un fichier se terminant là provoquait un
    accès hors limites. C'est un panic sur un fichier DCS malformé, pas une
    erreur.
  - **Une table malformée pouvait corrompre des valeurs en silence.** Une table
    mélangeant entrées positionnelles et clés `[n]` explicites voyait ses valeurs
    positionnelles renumérotées à partir de 1 par-dessus les explicites. Le
    contrat de forme est désormais documenté par `TestTableShapes`, car il est
    structurant : `radio.lua`/`beacons.lua` sont positionnels et doivent rester
    une map.
  - **La position retenue d'un aérodrome et ses aides variaient d'un lancement à
    l'autre.** Les balises étaient lues en parcourant une map Go : laquelle de
    deux balises ILS l'emportait, et dans quel ordre les aides étaient listées,
    dépendait de l'ordre d'itération. Les entrées sont maintenant ordonnées.
    Vérifié sur trois lancements : résultats identiques.
  - **Deux aérodromes proches pouvaient adopter le même identifiant embarqué**,
    après quoi l'index n'en gardait qu'un, avec le nom et la piste de l'autre.
    Une entrée embarquée ne peut désormais être prise qu'une fois.
- **`internal/charts` divergeait de `internal/theatre` sur deux identifiants.**
  Les cartes d'un dossier Sinai étaient étiquetées `Sinai` (DCS dit `SinaiMap`)
  et Marianas WWII `MarianasWWII` (DCS dit `MarianaIslandsWWII`) : le filtrage
  par théâtre ne renvoyait rien et un identifiant inconnu du sélecteur était
  annoncé. Un test lie désormais les deux listes.
- **Une carte dont le dossier ne correspondait à aucun théâtre était invisible.**
  Elle était indexée et servie, mais une liste sans filtre ne parcourait que les
  théâtres : elle n'apparaissait jamais. Ces cartes sont maintenant renvoyées.
- **Un code d'aérodrome à deux caractères ne pouvait jamais correspondre.** L'index
  de recherche exigeait trois caractères, donc `H4_VAD.png` était introuvable,
  alors que le commentaire du code affirmait le contraire.
- **`install-lua` pouvait installer un script périmé en silence.** Un fichier
  existant mais illisible (permissions, verrou) retombait sur la copie embarquée
  sans rien dire. Seul un fichier réellement absent le fait désormais.
- **La recherche d'un nom de sauvegarde était une boucle non bornée.** Une erreur
  de `stat` autre que « n'existe pas » ne la cassait jamais : elle tournait et
  allongeait le chemin indéfiniment au lieu de signaler le problème.
- **Le tracker pouvait ouvrir deux missions pour la même session.** Le
  get-or-create s'exécutait hors du mutex du tracker : deux goroutines pouvaient
  chacune ne rien trouver et en créer une. Il s'exécute désormais sous le verrou,
  et l'id créé est mémorisé. Une unité dont la perte a été enregistrée est
  oubliée : les maps de suivi ne grossissent plus avec chaque unité jamais vue.
- **Une connexion TCP traitée avant le câblage de ses callbacks perdait son
  premier message.** Le listener n'est plus démarré qu'une fois tous les callbacks
  en place, ce qui supprime la fenêtre où un handler lisait un callback nil.

### Sécurité

- **L'API est désormais protégée contre le pilotage par une page web.** Elle
  contient un endpoint destructif (`/api/maintenance/purge`) et aucune
  authentification, et le serveur écoutait sur `0.0.0.0` :
  - n'importe quel site que tu visitais pendant que le manager tournait pouvait
    lui envoyer un `POST` — un POST simple atteint le serveur sans preflight — et
    supprimer ta base. Le problème a été reproduit avant d'être corrigé.
  - une page en DNS rebinding pouvait l'atteindre avec un `Origin` cohérent.
  L'origine est maintenant comparée au Host de la requête, le Host doit désigner
  la machine locale tant que le serveur est lié au loopback, et **l'adresse
  d'écoute par défaut est `127.0.0.1:8080`** au lieu de `0.0.0.0:8080`. Atteindre
  l'interface depuis un autre appareil reste possible, délibérément, avec
  `DCSMANAGER_HTTP_ADDR=0.0.0.0:8080` — auquel cas le README précise que l'API est sans
  authentification. Couvert par `TestOriginGuard` et `TestIsLoopbackAddr`.
- **Les noms de joueurs étaient injectés sans échappement dans les infobulles
  Leaflet.** Leaflet interprète leur contenu comme du HTML : un joueur nommé
  `<img src=x onerror=...>` dans une mission multijoueur aurait exécuté du script
  dans le navigateur de chaque spectateur. Les noms d'aérodromes, de villes et
  les ids d'unités sont échappés de même.

### Corrigé

- **De mauvais identifiants de théâtre masquaient des aérodromes pourtant lus
  correctement.** DCS déclare `MarianaIslands` et `SinaiMap` ; le code utilisait
  `Marianas` et `Sinai`, donc les 5 aérodromes des Mariannes étaient
  inaccessibles, et `MarianaIslandsWWII`, `GermanyCW` et `SouthEastAsia`
  manquaient purement et simplement. Les identifiants sont désormais ceux de DCS
  (15 théâtres), les anciennes graphies restent acceptées en alias pour qu'une
  préférence enregistrée ne bloque personne sur une carte vide, et un théâtre
  mémorisé qui n'existe plus retombe sur le Caucase. Couvert par
  `TestDCSIdentifiers`.
- **Un aérodrome sans position fournie par DCS est maintenant signalé.** 32 des
  101 aérodromes proviennent d'une entrée radio sans balise correspondante : ils
  n'ont pas de coordonnées et ne peuvent pas être placés (les 11 de Marianas
  WWII, 17 de Kola, Novorossiysk et Soganlug au Caucase). Ils étaient
  silencieusement inutilisables ; le journal de démarrage indique désormais
  combien par théâtre, pour que l'écart soit visible plutôt que mystérieux.
  **69 aérodromes sont plaçables, 101 sont listés.**
- **Les tuiles DCS authentiques ne pouvaient jamais se charger.** Le handler
  ajoutait « .png » au segment `y` alors que Leaflet l'envoie déjà (le template
  est `/{z}/{x}/{y}.png`) : chaque requête aboutissait à `11.png.png` et un 404.
  Le bug était invisible tant qu'aucune tuile n'existait sur disque ; il est
  maintenant couvert par `TestHandleTilesExtension`, qui vérifie aussi que les
  traversals restent refusés.
- **Une aide à la navigation pouvait s'afficher avec une fréquence impossible.**
  DCS déclare l'ILS d'Ivalo à 212 MHz et le VOR de Sas Al Nakheel à 128,925 MHz,
  hors des bandes de ces aides : aucun pilote ne peut les syntoniser. Les
  fréquences sont désormais validées contre leur bande (ILS 108,10–111,95 MHz,
  VOR 108–117,95 MHz, NDB 190–1750 kHz), et une aide hors bande est écartée
  plutôt qu'affichée. 1 ILS sur 122 était concerné. Un rejet est journalisé au
  démarrage pour qu'un problème de données en amont reste visible au lieu d'être
  avalé.
- **Les noms de cartes trop longs débordaient de la fiche d'aérodrome**, dont la
  largeur pouvait aussi dépasser la carte sur une fenêtre étroite. Les deux sont
  désormais bornés, le nom étant tronqué par des points de suspension.

### À venir

- Autres fonctions inspirées de MizMap / MovingMap : mesure BRA, cercles SAM,
  symboles MIL-STD-2525C

## [1.0.0-beta.3] — 2026-09-26

Le manager devient un **compagnon local** : il lit désormais les fichiers de DCS
lui-même. Ce seul changement a transformé les aérodromes d'un jeu de données
transcrit pour une carte en la vérité du simulateur pour chaque carte installée.

C'est aussi la première version qui **exécute `dcsmanager.exe` sous Windows en CI**, et
la première où les cartes aéronautiques que tu as déjà sur disque sont lisibles
depuis l'application.

Toujours une pré-version : les scripts Lua n'ont pas encore été exécutés sur une
session DCS réelle. Voir « Périmètre de la bêta » dans l'entrée précédente.

### Modifié

- **Les données d'aérodromes ne sont plus transcrites à la main pour le Caucase.**
  DCS est lu en priorité ; l'entrée curatée ne sert plus qu'à l'enrichir.
- **Le manager est désormais 100 % local.** Il tourne sur la même machine Windows
  que DCS. Le déploiement Docker/Linux est retiré : `deploy/`, `.dockerignore`,
  `docs/deployment.md`, les cibles `make docker` / `make docker-multiarch` et
  l'option `build.ps1 -Target docker` disparaissent, ainsi que toutes les mises en
  garde sur l'adresse LAN et les ports du conteneur. C'est ce qui rend possible la
  lecture des aérodromes : un backend local peut lire les fichiers de DCS, et cela
  élimine toute la classe de problèmes « mauvaise IP / pare-feu / publication de
  ports ».
- La CI et le workflow de release construisent et testent maintenant **`dcsmanager.exe`
  sous Windows** plutôt qu'un substitut Linux : l'artefact vérifié est donc celui
  que les utilisateurs téléchargent.

### Ajouté

- **Visualiseur de cartes aéronautiques.** Les cartes d'approche, plans de
  mouvement et cartes de procédure conservés dans `maps_dcs/` sont indexés par
  nom (fichier, type, piste) et listés sur la fiche de chaque aérodrome : on
  choisit un aérodrome, on voit ses cartes, on clique pour la lire en plein écran
  avec zoom, et on peut l'ouvrir dans un onglet. 114 cartes sur 8 théâtres sont
  trouvées dans le dossier actuel.
  - Elles sont associées à un aérodrome par leur nom, qui est la façon dont les
    scans sont nommés (« 01_VAD_UG5X_Kobuleti.png »,
    « NORWAY_LAKSELV-ILS-RWY34.jpg ») : les cartes d'instrument de Kola
    fonctionnent donc sans jeu de données supplémentaire.
  - Elles sont affichées **comme documents**, jamais superposées : les scans ne
    sont pas géoréférencés et sont en projection conique, les déformer sur la
    carte serait faux. Une note dans le visualiseur le dit.
  - Elles ne sont ni embarquées ni rediffusées ; le dossier est local et chaque
    scan garde sa licence. `DCSMANAGER_CHARTS_DIR` change le dossier.
  - L'endpoint de fichier ne sert que les fichiers présents dans l'index : une
    URL forgée ne peut donc rien lire d'autre.
- **Style de carte aéronautique.** Un cinquième fond, « Aéronautique », s'appuie
  sur la base relief — courbes de niveau, relief ombré et occupation du sol, déjà
  proche d'une carte — et affiche les aérodromes sous forme d'appels
  cartographiques permanents (nom, OACI, Tower, TACAN, ILS) qui apparaissent au
  fur et à mesure du zoom, pour que la vue ne devienne jamais un empilement de
  cadres. Choisir ce style révèle les aérodromes, puisque c'est sa raison d'être.
  - C'est une *base style carte avec du vrai contenu aéronautique*, pas une carte
    scannée. Il n'existe aucune source de tuiles aéronautiques gratuite,
    mondiale et sans clé : OpenAIP exige une clé d'API, open flightmaps n'a pas
    d'endpoint public, VFRMAP ne couvre que les États-Unis, et l'imagerie F10 de
    DCS est l'œuvre protégée d'Eagle Dynamics — lire des données factuelles de
    l'installation est une chose, rediffuser leur imagerie en est une autre. La
    raison est consignée dans le commentaire du paquet `basemap`.
  - Un rendu pâle et délavé a été essayé d'abord et a dû être remplacé : il
    rendait la carte illisible. La base relief n'a besoin que d'un léger
    apaisement, et le résultat est bien plus lisible.
- **Aérodromes lus depuis DCS lui-même.** Le manager étant local, il lit les
  fichiers de terrain du simulateur (`Mods/terrains/<carte>/radio.lua` et
  `beacons.lua`) au lieu de dépendre d'un jeu de données transcrit. Les deux
  fichiers partagent un identifiant (`airfield22_0` = Batumi dans les deux) : le
  premier donne le nom et la fréquence ATC, le second toutes les aides à la
  navigation (TACAN, ILS, VOR, RSBN, NDB, PRMG) avec de vraies coordonnées.
  - **101 aérodromes sur 5 cartes** au lieu de 21 sur une seule, et *plus
    complets* : DCS déclare 6 TACAN et 13 ILS pour le Caucase là où le jeu curaté
    n'en avait que 5 et 10.
  - Les données du simulateur font foi et se mettent à jour à chaque patch ; le
    jeu embarqué reste en repli (et fournit les codes OACI, pistes, coalitions et
    cartes que DCS n'expose pas, appariés par proximité).
  - L'onglet Aérodromes indique la provenance (« Lu depuis DCS » ou
    « Données embarquées »).
- **Couche villes.** `Mods/terrains/<carte>/map/towns.lua` contient des milliers
  de localités géolocalisées (1691 pour le Caucase, 385 pour le golfe Persique).
  Elles sont exposées via `/api/towns` et peuvent être affichées sur la carte,
  lui donnant du contexte sans aucune donnée de mission.
- **`DCSMANAGER_SAVED_GAMES`** permet de forcer le dossier Saved Games. L'installation
  DCS est localisée via le registre, avec un repli sur la ligne `Command line:`
  de `Logs/dcs.log`.
- Le parseur Lua comprend désormais les constructions des fichiers DCS : le
  wrapper gettext `_("…")` et les constantes d'énumération (`BEACON_TYPE_TACAN`,
  `MODULATIONTYPE_AM`, `VHF_HI`). Sans elles, lire les données de terrain est
  impossible.
- **Sélecteur de théâtre et étendue de la carte.** La carte live a désormais un
  sélecteur de théâtre (Caucasus, Syria, Nevada, Persian Gulf, Marianas, Sinai,
  Kola, Afghanistan, Iraq, Falklands, Normandy, The Channel). En choisir un cadre
  la carte sur la boîte englobante de cette carte, bascule la liste des
  aérodromes dessus, et propose un bouton « Limites » qui trace l'étendue de la
  carte DCS. Le choix est conservé.
- **Aérodromes sur la carte avec leurs données.** Un bouton « Aérodromes »
  affiche les aérodromes du théâtre sous forme de marqueurs ; cliquer l'un d'eux
  ouvre une fiche (OACI, coalition, coordonnées, élévation, piste, Tower, TACAN,
  ILS). La fiche propose aussi « Voir sur la carte », et cliquer un aérodrome
  dans l'onglet Aérodromes bascule maintenant sur la carte et le centre.

- **Suivi de la source des sessions (`live` / `test`) et commande `dcsmanager purge`.**
  Une session enregistrée pendant que les outils de test tournent est
  indiscernable d'un vrai vol, car ces outils parlent exactement le même
  protocole que DCS. Chaque mission porte désormais une `source`, détectée
  automatiquement à partir des indicatifs des fixtures (`DCSMANAGER_SOURCE` force le
  verdict), et les statistiques excluent les sessions `test` sauf avec
  `?includeTest=1`. Nouvelles commandes CLI et HTTP (`dcsmanager purge`,
  `DELETE /api/maintenance/purge`) pour supprimer des sessions, ce qui n'avait
  jusqu'ici aucune option en dehors de la suppression du fichier de base.

### Corrigé

- **La carte (et ses fiches superposées) pouvait sortir de l'écran.** Un en-tête
  trop large élargissait la colonne de la grille : sur une fenêtre étroite, une
  partie de la carte et les fiches unité/aérodrome se retrouvaient hors du
  viewport. L'en-tête passe maintenant à la ligne et la colonne est bornée.
- **Missions fantômes vides.** Le tracker de positions ouvrait une « Session
  without mission » dès son premier tick, même si aucune unité n'avait jamais été
  signalée : un backend inactif accumulait donc des sessions vides dans
  l'interface. Une mission n'est maintenant créée qu'après l'échantillonnage
  d'une vraie position.
- **Une session détectée comme simulée en cours de vol restait comptée comme
  réelle.** Le tracker peut ouvrir une mission avant l'arrivée du premier paquet
  de test ; la mission est maintenant promue en `test` aux ticks suivants, et ne
  peut jamais redevenir `live`.
- **L'analyse pouvait s'ouvrir sur une session simulée.** La heatmap et les
  traces se basent par défaut sur la mission `live` la plus récente, et non plus
  sur la plus récente toutes sources confondues.

## [1.0.0-beta.2] — 2026-09-26

Corrige un premier lancement cassé en beta.1 : le `dcsmanager.exe` téléchargé ne
pouvait pas exécuter `install-lua`.

### Corrigé

- **`install-lua` ne fonctionnait pas depuis un binaire téléchargé.** Les scripts
  devaient se trouver dans un dossier `dcs-lua/` à côté de l'exécutable, ce qu'une
  archive de release ne contient jamais : la commande documentée
  `.\dcsmanager.exe install-lua` échouait avec « dcs-lua directory not found ». Les
  scripts sont désormais **embarqués dans le binaire** (générés par
  `tools/gen-lua-embed.mjs`), le dossier sur disque gardant la priorité en
  développement. Couvert par `TestEmbeddedFallback`, et la CI échoue si
  `dcs-lua/` change sans régénérer la copie embarquée.
- Bug de séparateur de chemin : `Hooks/dcsmanager.lua` se résolvait sous Linux mais
  `Hooks\dcsmanager.lua` échouait sous Windows. Les chemins relatifs sont désormais
  normalisés avant la résolution.

### Notes

- `v1.0.0-beta.1` est remplacée. Son binaire ne pouvait pas installer les scripts
  Lua : préférez `beta.2`. Le tag est laissé en place plutôt que déplacé, un tag
  publié devant rester immuable.

## [1.0.0-beta.1] — 2026-09-26

Première **pré-version publique**. Toutes les fonctions sont implémentées et toute
la chaîne est testée, mais les scripts côté DCS n'ont pas encore été exécutés sur
une installation DCS réelle (voir « Périmètre de la bêta » ci-dessous).

**Version bilingue.** Tout le projet est en anglais, le français restant une option
de premier ordre.

### Added

- **Internationalisation de l'interface (`frontend/src/lib/i18n.js`)**
  - Dictionnaires EN/FR complets (186 clés chacun, synchronisation vérifiée).
  - **Anglais par défaut** ; le sélecteur de langue est dans l'en-tête, le choix
    est conservé (`localStorage`) et appliqué à `<html lang>`.
  - Traducteur réactif `$t()` dans le markup, `tNow()` dans les scripts.
  - Pluriels gérés explicitement (le français et l'anglais ne s'accordent pas
    de la même façon).

- **Documentation bilingue**
  - `README.md` / `CHANGELOG.md` en anglais (principaux), `README.fr.md` /
    `CHANGELOG.fr.md` conservés, avec liens croisés en tête.

- **Licence** : MIT (`LICENSE`), plus une mention de non-affiliation à Eagle
  Dynamics dans les deux README.

- **Note de provenance** pour les données d'aérodromes
  (`backend/internal/aerodrome/data/README.md`) : cartes sources, et nature
  factuelle / droit des bases de données.

### Changed

- Toutes les chaînes, commentaires, logs, messages CLI, docstrings et pages de
  documentation en français ont été traduits en anglais : backend (tous les
  paquets), scripts Lua, outils, workflow CI, fichiers Docker, scripts
  PowerShell, exemples.
- **Les valeurs de protocole sont volontairement inchangées** (clés `dcsmanager_*`,
  clés JSON, noms d'événements DCS, valeurs de coalition, `optview_*`, ids de
  théâtre, ids de fond de carte) : les installations existantes continuent de
  fonctionner.
- Les marqueurs `DCSMANAGER-BEGIN`/`DCSMANAGER-END` ont été traduits **des deux côtés**
  (`install.go` et les scripts Lua) pour rester identiques octet pour octet.
- Le bandeau de fog of war est désormais traduit côté client à partir du `mode`
  (valeur stable), au lieu d'afficher le libellé du backend.

### Fixed

- **Corruption d'encodage** dans `docs/phase2-events.md` : la traduction avait
  transformé chaque lettre `i` en `n` (`internal` → `nnternal`, `with` → `wnth`).
  La page a été réécrite ; un balayage a confirmé qu'aucun autre fichier n'était
  touché.
- `frontend/index.html` déclarait `lang="fr"` alors que l'application est
  désormais en anglais par défaut.
- Les commentaires de la CI et de Docker étaient encore en français.

### Périmètre de la bêta

Vérifié :

- Live map, de la télémétrie UDP jusqu'au navigateur (SSE), via le backend Go.
- Événements, joueurs et chat en TCP, persistés en SQLite et rejoués au
  redémarrage.
- Analyse de débrief sur un **vrai** `debrief.log`, transfert en morceaux inclus.
- Statistiques avancées, analyse (heatmap, traces, sorties) et jeu de données
  d'aérodromes du Caucase.
- Filtrage fog of war piloté par les valeurs officielles `optionsView` de DCS.
- Binaire unique et image Docker ; CI verte.

Non encore vérifié sur une session DCS réelle :

- `Scripts/Export.lua` et `Scripts/Hooks/dcsmanager.lua` exécutés dans DCS.
- `Sim.getMissionOptions()` sur une vraie mission (valeurs issues de
  `optionsDb.lua` de DCS).
- Comportement du fog of war avec de vraies unités et coalitions.
- Transfert du débrief depuis une vraie fin de mission.
- Disponibilité de LuaSocket dans l'état Lua GUI/hooks.

Les retours et rapports de bugs sont bienvenus via les Issues GitHub.

## [0.9.0] — 2026-09-26

**Phase 7 — Fog of war.** Le manager respecte désormais les options de visibilité
de la mission : il ne révèle plus ce que DCS cache.

### Ajouté

- **`internal/visibility`**
  - Politique de visibilité basée sur `optionsView` de la mission, **restrictive
    par défaut** : ne jamais montrer plus que DCS.
  - Filtrage **côté serveur** (le client ne reçoit jamais les unités masquées).
  - L'appareil du joueur reste toujours visible ; sans lui la carte serait
    inutilisable.
  - Mode « fog of war » (`optview_allies`) traité comme « alliés uniquement » :
    les contacts capteurs ne sont pas reproduits, par choix de sûreté.

- **Backend**
  - Le hook Lua transmet `Sim.getMissionOptions()` au démarrage de mission.
  - `internal/tcp` remonte les options ; le serveur en déduit la politique.
  - `GET /api/visibility` et clé `visibility` dans chaque trame d'état ;
    événement SSE `visibility` à chaque changement.
  - `internal/config` : `DCSMANAGER_REVEAL_ALL_UNITS` (défaut `false`).

- **Frontend**
  - Bandeau sous l'en-tête indiquant le mode actif et sa limite.

- **Documentation & tests**
  - `docs/fog-of-war.md` (valeurs DCS officielles, principe, limites).
  - Tests `internal/visibility` : correspondance des valeurs, filtrage par mode,
    **non-fuite des unités ennemies/neutres**, dérogation, absence d'appareil.

### Notes

- Les valeurs officielles viennent de `MissionEditor/modules/Options/optionsDb.lua`
  de DCS. Au passage, deux libellés avaient été mal interprétés au départ :
  `optview_allies` est le « FOG OF WAR », `optview_onlyallies` est « ALLIES ONLY ».

## [0.8.0] — 2026-09-26

**Phase 6 — Aérodromes.** Référentiel des terrains du Caucase : coordonnées,
fréquences radio et cartes d'approche.

### Ajouté

- **Données aérodromes (`internal/aerodrome`)**
  - 21 terrains du Caucase extraits des **cartes d'approche fournies** :
    coordonnées, élévation, piste, **Tower**, **TACAN**, **ILS** par piste, et
    références aux cartes VAD/GND.
  - Les données sont **embarquées** (`//go:embed data/*.json`) ; le format est
    générique, ajouter un théâtre = déposer un fichier JSON.
  - DCS n'exposant pas les fréquences à l'exécution, ce référentiel est la seule
    source fiable.

- **API**
  - `GET /api/aerodromes` (filtre `?theatre=`, tri par distance avec `?lat=&lng=`)
    et `GET /api/aerodromes/{code}`.

- **Frontend**
  - Onglet **Aérodromes** : recherche par nom/code OACI/TACAN, bouton
    « Proches de moi », fiche détaillée (Tower, TACAN, ILS, cartes disponibles).
  - Case **« Sur la carte »** : les terrains s'affichent en marqueurs, fréquences
    en infobulle.

- **Documentation & tests**
  - `docs/phase6-aerodromes.md` (source, périmètre, modèle, limites).
  - Tests `internal/aerodrome` : chargement, terrain connu, complétude, tri,
    filtrage par théâtre.

## [0.7.0] — 2026-09-26

**Phase 5 — Packaging.** CLI, injecteur Lua sûr, déploiement `.exe` / Docker.

### Ajouté

- **CLI (`dcsmanager`)**
  - `install-lua` : installe/fusionne les scripts dans Saved Games ;
  - `uninstall-lua` : retire le bloc et `Hooks/dcsmanager.lua` ;
  - `status` : `installed` / `outdated` / `missing` par fichier ;
  - `version` / `help`.
  - Détection automatique de `Saved Games` (`DCS.openbeta` prioritaire) et du
    dossier `dcs-lua` ; `--saved-games`, `--lua-dir`, `--dry-run`.

- **Injecteur Lua (`internal/install`)**
  - **Ne remplace jamais** un `Export.lua` existant (Tacview, SRS, DCS-BIOS…) :
    fusion d'un bloc délimité par `>>> DCSMANAGER-BEGIN >>>` / `<<< DCSMANAGER-END <<<`.
  - **Sauvegarde horodatée** avant toute modification.
  - **Idempotent** : une seconde exécution met à jour le bloc en place.
  - Marqueurs ajoutés dans `Export.lua` et `Hooks/dcsmanager.lua`.

- **Déploiement**
  - `Dockerfile` : `go.sum` copié (build reproductible), version injectée par
    `-ldflags`, `ca-certificates`, utilisateur non-root, volume `/data`.
  - `docker-compose.yml` : ports UDP/TCP publiés, rétention configurée.
  - `Makefile` : cibles `install` et `docker-multiarch` ; `build.ps1` injecte la
    version depuis `VERSION`.
  - `install-dcs.ps1` : installateur Windows en un clic avec `-DryRun`.

- **Documentation & tests**
  - `docs/deployment.md` : les deux modes, le piège de l'IP LAN en Docker, ports.
  - `VERSION` (0.7.0).
  - Tests `internal/install` : création, **fusion préservant le contenu tiers**,
    idempotence, remplacement en place, sauvegarde, `--dry-run`, désinstallation,
    statut (missing/installed/outdated).

### Corrigé

- `Export.lua` appelait `toJson()`, jamais défini : l'envoi du joueur échouait
  silencieusement. La charge JSON est désormais construite directement, avec les
  champs de télémétrie ajoutés conditionnellement.

## [0.6.0] — 2026-09-26

**Phase 4 bis — Cartes analytiques & analyse de sortie.** Historique des positions,
cartes de chaleur, traces de vol et télémétrie.

### Ajouté

- **Backend**
  - `internal/tracker` : échantillonnage périodique des positions (traces) et
    **détection de pertes** par disparition prolongée, avec sa dernière position.
    Rétention configurable et purge horaire. Crée une mission de session si aucune
    n'a été annoncée.
  - Tables `track_positions` et `losses`.
  - `internal/db` : `SaveSamples` (par lots), `SaveLoss`, `Heatmap` (agrégation en
    grille de degrés, **sans projection**), `Trails`, `PruneTracking`.
  - `internal/tracker.Analyse` : durée, distance (haversine), altitude/vitesse/G max.
  - Routes `GET /api/analytics/{heatmap,tracks,sorties}`.
  - Configuration : `DCSMANAGER_TRACK_INTERVAL`, `DCSMANAGER_TRACK_GRACE`,
    `DCSMANAGER_TRACK_RETENTION`.
  - Champs de télémétrie (`speed`, `g`, `aoa`) sur les unités suivies.

- **Scripts DCS**
  - `Export.lua` : télémétrie ownship conditionnelle (vitesse vraie/indiquée, Mach,
    incidence, altitude sol, facteur de charge) si le serveur l'autorise.

- **Frontend**
  - Onglet **Analyse** : source de carte de chaleur (Trafic / Pertes) et tableau
    d'analyse de sortie par unité.
  - Bouton **Historique** sur la carte : superpose la carte de chaleur (dégradé
    froid→chaud) et les traces enregistrées.

- **Documentation & tests**
  - `docs/phase4bis-analytics.md`.
  - Tests `internal/tracker` : échantillonnage, perte, non-duplication,
    réapparition, haversine, analyse de sortie.

## [0.5.0] — 2026-09-26

**Phase 4 — Statistiques avancées.** Sept modules d'analyse, en portée **carrière**
(toutes missions, par UCID) ou **mission**.

### Ajouté

- **Backend**
  - `internal/stats` : vue d'ensemble, pilotes (score, kills, K/D, FF, ping),
    armes (kills, friendly-fire, cibles et plateformes), engins (**type DCS exact** :
    kills, pertes, sorties, K/D), coalitions, réseau (ping moyen/max).
  - Jointure des événements aux joueurs via `dcs_player_id`, ce qui permet de
    dériver les **morts** et le **friendly-fire** par pilote (absents de
    `net.get_stat`).
  - Colonne `unit_type` dans `player_stats`.
  - Routes `GET /api/stats/{overview,pilots,weapons,engines,network}` avec
    `?scope=career|mission` et `?missionId=N`.

- **Scripts DCS**
  - `Hooks/dcsmanager.lua` : résolution du **type d'appareil** par joueur via
    `Sim.getAvailableSlots` (mise en cache, rafraîchie toutes les 30 s) ; le champ
    `unitType` remplace l'usage du `slotID` opaque.

- **Frontend**
  - Onglet **Statistiques** : sélecteur Carrière/Mission, cartes de synthèse, et
    cinq sous-vues (Pilotes, Armes, Engins, Balance, Réseau).
  - Classement des pilotes avec médailles, K/D, friendly-fire et ping.
  - Filtres par catégorie d'engin ; barres comparatives des coalitions.

- **Documentation**
  - `docs/phase4-stats.md`.
  - Tests `internal/stats` (portées, armes, engins, coalitions, réseau, overview).

### Notes

- Les modules **4.3 Cartes analytiques** (heatmaps, traces) et **4.5 Analyse de
  sortie** (télémétrie) sont reportés à un incrément dédié : ils s'appuieront sur
  les positions lat/lng de la live map, sans projection par théâtre.

## [0.4.0] — 2026-09-26

**Phase 3 — Débriefings.** À la fin de chaque mission, `debrief.log` est envoyé au
backend, analysé et archivé.

### Ajouté

- **Parsing Lua (`internal/lua`)**
  - Parseur du sous-ensemble Lua utilisé par les fichiers de données DCS
    (`nom = valeur`, tables imbriquées, chaînes, commentaires). **N'exécute pas
    de Lua** : lecture de données uniquement.

- **Analyse des débriefs (`internal/debrief`)**
  - Extraction typée : chemin du `.miz`, durée, `result`, état final du monde et
    **chronologie des événements** (`takeoff`, `land`, `engine shutdown`,
    `mission end`, `kill`, `crash`, `eject`…).
  - Agrégats (`Summarise`) : décollages, atterrissages, kills, crashes, éjections,
    pilotes, répartition par type d'événement.

- **Transport et stockage**
  - `internal/debriefstore` : réassemblage des transferts **multi-morceaux**
    (chunks base64, arrivée désordonnée tolérée, transferts concurrents isolés).
  - `Hooks/dcsmanager.lua` : lit `debrief.log` en fin de mission et l'envoie en morceaux
    de 32 Ko encodés en base64.
  - Table `debriefs` (métadonnées, `parsed` JSON, `raw` original).

- **API**
  - `GET /api/debriefs`, `GET /api/debriefs/{id}` et `?raw=1` pour le texte brut.

- **Frontend**
  - Onglet **Débriefs** : liste, compteurs (décollages, atterrissages, kills,
    crashes, éjections, durée), pilotes et chronologie colorée par type.

- **Outils & documentation**
  - `tools/send-debrief.mjs` : envoie un `debrief.log` au backend.
  - `docs/phase3-debriefs.md`.
  - Tests : parseur Lua (dont un vrai `debrief.log`), analyse, réassemblage
    multi-morceaux, ordre des chunks, transferts concurrents, base64 invalide.

## [0.3.0] — 2026-09-26

**Phase 2 — Événements & joueurs.** Le backend reçoit et historise les événements
de jeu, les joueurs connectés et le chat.

### Ajouté

- **Backend**
  - `internal/model` : types échangés (messages, joueurs, événements, chat, missions).
  - `internal/tcp` : récepteur TCP au format **JSON ligne par ligne** (NDJSON),
    avec reconnexion par connexion et journalisation.
  - `internal/live` : état de session en mémoire (événements, joueurs triés par camp,
    chat, mission courante) avec plafonds.
  - `internal/db` : **persistance SQLite pure Go** (`modernc.org/sqlite`, sans CGO),
    tables `missions`, `events`, `chat`, `players`, `player_stats`, `meta`.
  - `internal/ingest` : pont entre le direct et la base ; ouvre la mission courante,
    résout les joueurs par **UCID** (carrière qui survit aux renommages) et
    n'enregistre les stats que des joueurs actifs.
  - `internal/api` : `GET /api/game-events`, `/api/players`, `/api/chat`,
    `/api/mission`, `/api/history/{events,chat,missions}` ; trame SSE `session`.
  - `internal/config` : `DCSMANAGER_DB_ENABLED`.

- **Scripts DCS**
  - `Hooks/dcsmanager.lua` : envoi TCP des événements (`onGameEvent`), du chat
    (`onChatMessage`), des joueurs et de leurs statistiques (`net.get_player_list`,
    `net.get_player_info`, `net.get_stat`), et des transitions de mission. Connexion
    persistante, reconnexion automatique, appels protégés par `pcall`, jamais bloquant.
  - `Config/dcsmanager.cfg` : `dcsmanager_players_interval`.

- **Frontend**
  - Onglets **Carte** / **Session** ; nom de la mission en cours dans l'en-tête.
  - Panneau **Joueurs** : nom, score, kills air/sol/navire, atterrissages, ping
    (surligné au-delà de 250 ms), code couleur par camp.
  - Panneau **Événements** : filtres par type avec compteurs, horodatage et
    description lisible des arguments DCS.
  - Panneau **Chat** : historique et zone de saisie (envoi vers DCS à venir).

- **Outils & documentation**
  - `tools/send-events.mjs` : simulateur du canal TCP (mission, joueurs, événements, chat).
  - `docs/phase2-events.md`.
  - Tests : `internal/live` (plafonds, tri, mission) et `internal/db` (missions,
    événements, chat, UCID, stats).

## [0.2.0] — 2026-09-25

**Phase 1 — Live map complète.** Tous les objets de la mission sont suivis, classés
et affichés sur une carte interactive.

### Ajouté

- **Backend**
  - `internal/category` : classification des **types DCS exacts** en familles
    (avion, hélicoptère, sol, navire, structure), avec surcharge par fichier JSON.
  - `internal/theatre` : théâtres DCS (Caucase, Syrie, NTTR, Golfe, Mariannes, etc.)
    avec leur emprise géographique.
  - `internal/udp` : prise en charge des messages **`world`** (listes d'objets) en plus
    d'`ownship` ; datagrammes jusqu'à 1 Mo.
  - `internal/state` : identifiants stables, plafond d'unités (`MaxUnits`), âge par unité.
  - `internal/api` : filtres (`?category=`, `?coalition=`, `?ownship=`, `?q=`), résumé
    par catégorie/coalition, `GET /api/units/{id}`, `GET /api/theatres`, et service des
    **tuiles de carte** DCS (`/api/tiles/<theatre>/<z>/<x>/<y>.png`) avec protection
    contre la traversée de chemin.
  - `internal/config` : nouvelles options (`DCSMANAGER_UNIT_TTL`, `DCSMANAGER_TILES_DIR`,
    `DCSMANAGER_BASEMAP_URL`, `DCSMANAGER_CATEGORIES`, `DCSMANAGER_MAX_UNITS`).

- **Scripts DCS**
  - `Export.lua` : export de **tous les objets du monde** en plus du joueur, filtrable
    par rayon (`dcsmanager_world_radius`), plafonné (`dcsmanager_max_objects`), coalitions
    sélectionnables ; accès défensif aux fonctions `LoGet*`/`Export.*`.
  - `Config/dcsmanager.cfg` : nouvelles options documentées.

- **Frontend**
  - Carte : icônes SVG par catégorie, couleurs par coalition, **traces de vol** pour
    avions et hélicoptères, survol avec infobulle, recentrage sur le joueur.
  - **Sélecteur de fond de carte** : Satellite, Relief, Routier, Sombre — plus l'option
    « DCS » automatique si des tuiles authentiques sont présentes. Aucun fond ne
    nécessite de clé API (le mode sombre est un filtre CSS sur les tuiles OSM).
    Le choix est mémorisé dans le navigateur.
  - Panneau latéral : filtres coalitions/catégories avec compteurs en direct, recherche,
    « mon appareil uniquement », liste des unités sélectionnables.
  - Fiche unité : type, catégorie, coalition, pays, position, altitude, cap, âge.
  - Support des **tuiles DCS** avec repli automatique sur un fond de carte réel.

- **Outils & documentation**
  - `tools/send-telemetry.mjs` : émet aussi des objets de monde (sol, navires, IA).
  - `tools/export-tiles.py` : extraction de tuiles depuis une image de carte géoréférencée.
  - `tools/inspect-maps.py` : recense les scans de cartes et exporte les coins.
  - `categories.example.json`, `docs/live-map.md`, `maps_dcs/README.md`.
  - `maps_dcs/` (≈1,2 Go) exclu de git ; seul son README est suivi.
  - Tests unitaires supplémentaires (catégories, théâtres, plafond du store, messages monde).

## [0.1.0] — 2026-09-25

Première version : **PoC Phase 0** — validation de la chaîne DCS → Go → navigateur.

### Ajouté

- **Documentation**
  - `README.md` : vision, architecture, prérequis, démarrage rapide, configuration,
    installation Lua, déploiement `.exe`/Docker, structure, roadmap.
  - `CHANGELOG.md` : suivi des versions.
  - `docs/architecture.md` : flux de données, composants, choix techniques.
  - `docs/dcs-installation.md` : guide d'installation côté DCS et dépannage.

- **Outils**
  - `tools/send-telemetry.mjs` : émetteur de télémétrie factice pour tester sans DCS.
  - `Makefile` et `build.ps1` : commandes de build (frontend, backend, Docker).

- **Scripts DCS (`dcs-lua/`)**
  - `Config/dcsmanager.cfg` : modèle de configuration (IP backend, ports, intervalle d'envoi).
  - `Export.lua` : export de la position du joueur vers le backend en UDP/JSON,
    échantillonné une fois par seconde via `LuaExportActivityNextEvent` (aucun impact
    sur les performances du simulateur).

- **Backend Go (`backend/`)**
  - `internal/config` : configuration par variables d'environnement (`DCSMANAGER_*`) avec défauts.
  - `internal/udp` : récepteur UDP décodant le JSON des positions.
  - `internal/state` : store en mémoire des unités (avec péremption).
  - `internal/api` : serveur HTTP (REST `GET /api/state`, flux **Server-Sent Events**
    `/api/events`) + service de l'interface web embarquée.
  - `cmd/dcsmanager/main.go` : point d'entrée assemblant les briques.

- **Frontend (`frontend/`)**
  - Interface Leaflet minimale affichant la position des unités en temps réel via
    **Server-Sent Events**, avec une page de repli générée si le frontend n'est pas buildé.

- **Déploiement (`deploy/`)**
  - `Dockerfile` multi-stage (build frontend + backend → image Alpine minimale, non-root).
  - `docker-compose.yml` avec ports UDP/TCP publiés et volume SQLite.

### Notes

- DCS World reste sur Windows et ne tourne jamais dans le conteneur Docker ; le manager
  communique avec lui par le réseau.
- Le temps réel de la Phase 0 utilise **SSE** (flux descendant). Un canal TCP
  bidirectionnel (commandes vers DCS) est prévu en Phase 2.
