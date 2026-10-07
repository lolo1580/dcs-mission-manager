# Comment Logitech prend en charge les panels dans Flight Simulator

Recherche du 7 octobre 2026, pour comprendre la possibilité de réutiliser le fonctionnement du logiciel d’origine dans DCS.

## Ce que confirme Logitech

Le [guide officiel d’installation Radio/Switch/Multi](https://support.logi.com/hc/en-150/articles/360023194254-FSX-driver-and-plugin-setup-guide) précise que ces panels ne nécessitent pas de pilote propriétaire supplémentaire et utilisent un plugin spécifique au simulateur. Le Flight Instrument Panel, différent du PZ55/PZ70, nécessite pour sa part un pilote et DirectOutput.

Logitech fournit un [plugin Microsoft Flight Simulator 2020](https://support.logi.com/hc/en-us/articles/360055219574-Microsoft-Flight-Simulator-Plug-in), version documentée 8.0.285.0, qui ajoute la prise en charge des Instrument/Switch/Multi/Radio panels. La page décrit notamment l’effet de l’interrupteur avionique sur l’alimentation des Radio/Multi panels.

Le guide FSX explique que le plugin utilise les commandes du simulateur et SimConnect. [L’API SimConnect de Flight Simulator](https://docs.flightsimulator.com/html/Programming_Tools/SimConnect/API_Reference/Events_And_Data/SimConnect_TransmitClientEvent.htm) permet à un programme externe de transmettre des événements au simulateur.

L’architecture USB → plugin → commandes du simulateur explique donc comment ces panels peuvent fonctionner sans joystick virtuel. L’utilisation exacte de SimConnect par le binaire du plugin 2020 reste une inférence de l’architecture documentée : son exécutable n’a pas été extrait ni analysé ici. Le fonctionnement en cockpit ne permet pas, à lui seul, d’affirmer que les panels sont exposés comme joysticks Windows ni qu’ils apparaissent comme colonnes attribuables dans le simulateur. La présence éventuelle de ces colonnes chez l’utilisateur reste à vérifier.

## Conséquences pour notre application

Le package `backend/internal/hid` utilise déjà l’interface HID Windows pour lire directement le PZ55/PZ70. Il n’est pas nécessaire de développer un pilote noyau pour lire leurs touches ou envoyer leurs rapports de sortie.

Notre chemin actuel panel USB → application → DCS-BIOS → DCS reprend le principe d’un plugin spécifique au simulateur. Il permet les actions dans le cockpit, les LED et le LCD sans vJoy. Le mapping avion reste nécessaire dans ce chemin ; une fonction non exportée par le catalogue installé demande une autre interface DCS appropriée.

Ce fonctionnement ne crée pas de périphérique de jeu virtuel. Pour l’objectif distinct « deux nouvelles colonnes de boutons directement attribuables dans DCS », il faut encore une exposition compatible avec les contrôleurs de jeu Windows, par exemple vJoy ou un pilote virtuel dédié.

Le logiciel Logitech pour Flight Simulator ne suffit donc pas à fournir les colonnes de commandes souhaitées dans DCS. Examiner son exécutable pourrait préciser sa communication USB/SimConnect ; cela ne prouverait pas qu’il expose des joysticks réutilisables par DCS.

Aucun pilote ni plugin Logitech n’a été installé ou exécuté pendant cette recherche. Aucune modification des modes d’entrée existants dans l’application.
