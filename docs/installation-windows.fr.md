# Installation Windows personnalisée de DCS Manager

L’installateur `DCSManager-Setup-<version>.exe` est le seul format publié. Il installe l’application pour le compte Windows courant, sans administrateur, avec logo DCS Manager, thème sombre, français/anglais et raccourcis.

À l’accueil, il détecte une installation existante du même compte Windows et affiche sa version ainsi que celle à installer. Si aucune installation n’est trouvée, il annonce une nouvelle installation. La même identité Inno Setup permet de mettre le programme à jour dans son dossier existant, sans créer une seconde entrée de désinstallation. Les données et profils restent dans leur dossier séparé.

## Parcours

1. Présentation et licence.
2. Dossier du programme, par défaut `%LOCALAPPDATA%\Programs\DCS Manager`, puis choix du raccourci Bureau.
3. Détection du dossier du jeu DCS et de Saved Games, avec correction manuelle possible. L’option scripts Lua est désactivée par défaut.
4. Reprise facultative des données d’une ancienne version portable : choisir son dossier contenant `data`. Fermer les applications avant la copie. L’original reste intact et les données installées existantes ne sont pas remplacées.
5. Installation puis lancement facultatif de DCS Manager.

Les scripts Lua sont installés seulement si l’option est cochée et si DCS est fermé. Les exports des autres outils et la configuration utilisateur sont conservés ; les fichiers modifiés sont sauvegardés. DCS-BIOS n’est pas installé par cet assistant. Le trim expérimental reste désactivé au démarrage.

Pour appliquer la correction des statistiques lors d’une sortie directe au bureau,
mettre aussi à jour les scripts Lua après l’installation : fermer DCS, cocher
l’option **scripts Lua** dans l’installateur ou utiliser **Paramètres →
Installation DCS → Installer / mettre à jour les scripts**, puis relancer DCS.
L’installateur seul ne remplace pas un hook déjà présent si cette option est
laissée désactivée.

Si le bandeau **Export DCS interrompu** apparaît pendant une mission active, vérifier l’état des scripts dans **Paramètres → Installation DCS**. Mettre les scripts à jour après avoir fermé DCS, puis relancer la mission. Le bandeau indique une absence de données récentes ; il ne confirme pas à lui seul une pause du simulateur.

## Données et mises à jour

Le programme installé contient le marqueur `installed-mode`. La base, les profils et le journal sont dans `%LOCALAPPDATA%\DCS Manager\data`, la configuration dans `settings.json`, et le cache WebView2 dans `webview`. Les variables `DCSMANAGER_*` explicites restent prioritaires.

Les mises à jour remplacent les fichiers du programme sans remplacer les données utilisateur. Les cartes volumineuses de la portable sont référencées dans leur dossier d’origine, pas copiées : conserver ce dossier ou changer `DCSMANAGER_CHARTS_DIR`. La migration copie le dossier `data` seulement, pas les cartes ni le cache navigateur.

À chaque lancement puis toutes les 12 heures, l’application vérifie les releases GitHub. Elle compare sa version aux releases qui contiennent réellement `DCSManager-Setup-<version>.exe` ; une version stable ignore les préversions, tandis qu’une version bêta peut proposer une bêta plus récente. Une mise à jour disponible apparaît dans la barre latérale et dans **Paramètres → Installation DCS**. Le bouton **Vérifier les mises à jour** force un nouveau contrôle. Le lien ouvre le téléchargement de l’installateur ; l’application ne l’installe pas automatiquement. Sans réseau, un message indique que la vérification est indisponible et l’application continue de fonctionner.

La désinstallation retire le programme et ses raccourcis. Elle conserve données, profils et scripts DCS. Pour retirer les scripts, utiliser la commande de désinstallation Lua décrite dans [la documentation du plugin](plugin-panels-dcs.fr.md), avec sauvegarde et DCS fermé.

## Construction

Prérequis : Go, Node.js et Inno Setup 6.7 ou supérieur. Depuis le projet, PowerShell :

```powershell
.\build.ps1
# Également disponible depuis le script dédié :
.\tools\build-installer.ps1
# Ou choisir explicitement le compilateur :
.\tools\build-installer.ps1 -ISCC 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe'
```

Le script construit l’interface, régénère les scripts Lua embarqués, compile l’application dans `dist/staging` et produit l’installateur dans `dist`. Seul `DCSManager-Setup-<version>.exe` est publié. Les profils fournis sont installés sous le programme ; les profils du joueur restent dans ses données.

Sources de construction : [Inno Setup](https://jrsoftware.org/isdl.php), [thème personnalisé](https://jrsoftware.org/ishelp/topic_setup_wizardstyle.htm) et [contrôles de l’assistant](https://jrsoftware.org/ishelp/topic_scriptclasses.htm). Les champs facultatifs utilisent des pages de saisie avec boutons Parcourir : les pages standard de sélection de dossier imposaient une valeur non vide.

## Vérification du 7 octobre 2026

Les tests Go complets et `go vet ./...` passent. Ils vérifient notamment la copie de la portable, la conservation des données installées, les dossiers invalides, la priorité des variables d’environnement et l’encodage UTF-16 des chemins accentués pour l’assistant Windows.

Le cycle silencieux installation → réinstallation → désinstallation a réussi avec un AppId de test distinct, sans raccourci ni entrée de désinstallation dans le registre, et sans écriture dans les vrais dossiers DCS ou données utilisateur. Le programme est retiré, la configuration et un fichier de données témoin sont conservés. Pour reproduire :

```powershell
.\tools\build-installer.ps1 -SkipBuild -TestBuild
.\tools\test-installer.ps1
```

Les journaux et données témoins restent dans `dist/smoke-*`. Le contrôle porte sur le fonctionnement silencieux ; la présentation interactive de toutes les pages, le lancement WebView2 et les scripts dans une vraie mission restent à vérifier. L’image de bienvenue a été contrôlée visuellement.

## Limites et retour arrière

Cet installateur n’est pas signé avec un certificat de publication DCS Manager. Windows peut afficher un avertissement lors de sa distribution. WebView2 doit être disponible sur la machine ; son installation automatique n’est pas incluse. Une erreur de préparation après copie du programme est signalée dans `%LOCALAPPDATA%\DCS Manager\setup.log` : ne pas considérer les scripts ou la migration comme terminés dans ce cas.

En cas de problème après une mise à jour, réinstaller l’installateur de la version précédente. La désinstallation conserve les données installées pour une réinstallation ultérieure.
