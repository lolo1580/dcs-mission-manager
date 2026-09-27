# Changelog

[🇬🇧 English](CHANGELOG.md) | 🇫🇷 Français

Toutes les modifications notables de ce projet sont documentées dans ce fichier.

Le format suit [Keep a Changelog](https://keepachangelog.com/fr/1.1.0/) et le projet adhère
au [versionnage sémantique](https://semver.org/lang/fr/).

## [Non publié]

### Corrigé

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
  `DCSMM_HTTP_ADDR=0.0.0.0:8080` — auquel cas le README précise que l'API est sans
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

C'est aussi la première version qui **exécute `dcsmm.exe` sous Windows en CI**, et
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
- La CI et le workflow de release construisent et testent maintenant **`dcsmm.exe`
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
    scan garde sa licence. `DCSMM_CHARTS_DIR` change le dossier.
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
- **`DCSMM_SAVED_GAMES`** permet de forcer le dossier Saved Games. L'installation
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

- **Suivi de la source des sessions (`live` / `test`) et commande `dcsmm purge`.**
  Une session enregistrée pendant que les outils de test tournent est
  indiscernable d'un vrai vol, car ces outils parlent exactement le même
  protocole que DCS. Chaque mission porte désormais une `source`, détectée
  automatiquement à partir des indicatifs des fixtures (`DCSMM_SOURCE` force le
  verdict), et les statistiques excluent les sessions `test` sauf avec
  `?includeTest=1`. Nouvelles commandes CLI et HTTP (`dcsmm purge`,
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

Corrige un premier lancement cassé en beta.1 : le `dcsmm.exe` téléchargé ne
pouvait pas exécuter `install-lua`.

### Corrigé

- **`install-lua` ne fonctionnait pas depuis un binaire téléchargé.** Les scripts
  devaient se trouver dans un dossier `dcs-lua/` à côté de l'exécutable, ce qu'une
  archive de release ne contient jamais : la commande documentée
  `.\dcsmm.exe install-lua` échouait avec « dcs-lua directory not found ». Les
  scripts sont désormais **embarqués dans le binaire** (générés par
  `tools/gen-lua-embed.mjs`), le dossier sur disque gardant la priorité en
  développement. Couvert par `TestEmbeddedFallback`, et la CI échoue si
  `dcs-lua/` change sans régénérer la copie embarquée.
- Bug de séparateur de chemin : `Hooks/dcsmm.lua` se résolvait sous Linux mais
  `Hooks\dcsmm.lua` échouait sous Windows. Les chemins relatifs sont désormais
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
- **Les valeurs de protocole sont volontairement inchangées** (clés `dcsmm_*`,
  clés JSON, noms d'événements DCS, valeurs de coalition, `optview_*`, ids de
  théâtre, ids de fond de carte) : les installations existantes continuent de
  fonctionner.
- Les marqueurs `DCSMM-BEGIN`/`DCSMM-END` ont été traduits **des deux côtés**
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

- `Scripts/Export.lua` et `Scripts/Hooks/dcsmm.lua` exécutés dans DCS.
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
  - `internal/config` : `DCSMM_REVEAL_ALL_UNITS` (défaut `false`).

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

- **CLI (`dcsmm`)**
  - `install-lua` : installe/fusionne les scripts dans Saved Games ;
  - `uninstall-lua` : retire le bloc et `Hooks/dcsmm.lua` ;
  - `status` : `installed` / `outdated` / `missing` par fichier ;
  - `version` / `help`.
  - Détection automatique de `Saved Games` (`DCS.openbeta` prioritaire) et du
    dossier `dcs-lua` ; `--saved-games`, `--lua-dir`, `--dry-run`.

- **Injecteur Lua (`internal/install`)**
  - **Ne remplace jamais** un `Export.lua` existant (Tacview, SRS, DCS-BIOS…) :
    fusion d'un bloc délimité par `>>> DCSMM-BEGIN >>>` / `<<< DCSMM-END <<<`.
  - **Sauvegarde horodatée** avant toute modification.
  - **Idempotent** : une seconde exécution met à jour le bloc en place.
  - Marqueurs ajoutés dans `Export.lua` et `Hooks/dcsmm.lua`.

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
  - Configuration : `DCSMM_TRACK_INTERVAL`, `DCSMM_TRACK_GRACE`,
    `DCSMM_TRACK_RETENTION`.
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
  - `Hooks/dcsmm.lua` : résolution du **type d'appareil** par joueur via
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
  - `Hooks/dcsmm.lua` : lit `debrief.log` en fin de mission et l'envoie en morceaux
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
  - `internal/config` : `DCSMM_DB_ENABLED`.

- **Scripts DCS**
  - `Hooks/dcsmm.lua` : envoi TCP des événements (`onGameEvent`), du chat
    (`onChatMessage`), des joueurs et de leurs statistiques (`net.get_player_list`,
    `net.get_player_info`, `net.get_stat`), et des transitions de mission. Connexion
    persistante, reconnexion automatique, appels protégés par `pcall`, jamais bloquant.
  - `Config/dcsmm.cfg` : `dcsmm_players_interval`.

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
  - `internal/config` : nouvelles options (`DCSMM_UNIT_TTL`, `DCSMM_TILES_DIR`,
    `DCSMM_BASEMAP_URL`, `DCSMM_CATEGORIES`, `DCSMM_MAX_UNITS`).

- **Scripts DCS**
  - `Export.lua` : export de **tous les objets du monde** en plus du joueur, filtrable
    par rayon (`dcsmm_world_radius`), plafonné (`dcsmm_max_objects`), coalitions
    sélectionnables ; accès défensif aux fonctions `LoGet*`/`Export.*`.
  - `Config/dcsmm.cfg` : nouvelles options documentées.

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
  - `Config/dcsmm.cfg` : modèle de configuration (IP backend, ports, intervalle d'envoi).
  - `Export.lua` : export de la position du joueur vers le backend en UDP/JSON,
    échantillonné une fois par seconde via `LuaExportActivityNextEvent` (aucun impact
    sur les performances du simulateur).

- **Backend Go (`backend/`)**
  - `internal/config` : configuration par variables d'environnement (`DCSMM_*`) avec défauts.
  - `internal/udp` : récepteur UDP décodant le JSON des positions.
  - `internal/state` : store en mémoire des unités (avec péremption).
  - `internal/api` : serveur HTTP (REST `GET /api/state`, flux **Server-Sent Events**
    `/api/events`) + service de l'interface web embarquée.
  - `cmd/dcsmm/main.go` : point d'entrée assemblant les briques.

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
