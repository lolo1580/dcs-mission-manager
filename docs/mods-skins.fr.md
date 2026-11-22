# Mods & skins

DCS Manager peut gérer les mods et livrées installés dans `Saved Games\DCS`.
C’est un outil réservé au poste desktop : le profil serveur Docker n’expose
jamais ces fichiers.

## Mods

Les mods sont lus dans :

- `Saved Games\DCS\Mods\<catégorie>\<mod>` — activés ;
- `Saved Games\DCS\Mods.disabled\<catégorie>\<mod>` — désactivés ;
- `Saved Games\DCS\Mods.quarantine\...` — retirés, restaurables depuis l’onglet
  Corbeille.

Actions :

- **Activer / désactiver** déplace le dossier entre `Mods` et `Mods.disabled`.
- **Mettre à la corbeille** conserve le dossier hors du jeu et écrit un
  manifeste pour pouvoir le restaurer.
- **Ouvrir le dossier** lance l’Explorateur sur le dossier du mod.

DCS doit être fermé pour toute action. Le gestionnaire ne touche jamais à
l’installation DCS ni aux modules officiels.

## Livrées

Le gestionnaire scanne :

- `Saved Games\DCS\Liveries\<Appareil>\<Livrée>` ;
- les livrées à plat directement sous `Saved Games\DCS\Liveries\<Livrée>` ;
- les packs mal placés du type
  `Liveries\<pack>\CoreMods\aircraft\<module>\Liveries\<Appareil>\<Livrée>` ;
- les livrées incluses dans les mods activés ou désactivés.

Il lit `description.lua` pour le nom, les pays, les textures et les dépendances
`../`. Une livrée à plat peut être **classée** dans la structure standard.
Lorsque plusieurs livrées partagent des textures, sélectionne ou déplace le
groupe entier pour ne pas casser les références relatives.

## Import

Le dialogue d’import accepte les archives ZIP, 7z et RAR. L’archive est d’abord
analysée ; rien n’est écrit dans le jeu avant le clic sur Installe. Une archive
peut contenir un ou plusieurs mods ou livrées.

L’archive est extraite dans un dossier temporaire, les traversées de chemin et
les liens symboliques sont refusés, puis la cible est déplacée seulement après
validation. Les éléments remplacés sont conservés dans la corbeille.

## Sécurité

- DCS doit être fermé.
- Seul `Saved Games\DCS` est modifié.
- Le contenu retiré est mis en quarantaine, pas supprimé.
- Les archives protégées par mot de passe sont refusées.
- Les mods sont du code tiers ; garde une sauvegarde du contenu important.

## Sauvegarde

La sauvegarde du profil propose désormais une catégorie **Livrées** et inclut
les mods désactivés avec la catégorie **Mods**.
