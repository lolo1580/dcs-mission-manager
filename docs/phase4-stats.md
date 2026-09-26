# Statistiques avancées (Phase 4)

## Deux portées

Toutes les statistiques existent en **deux portées** :

- **`career`** (défaut) : agrégation sur toutes les missions, **par UCID**, donc
  l'historique d'un pilote survit à ses changements de pseudo ;
- **`mission`** : limité à une mission (celle en cours, ou `?missionId=N`).

Côté API : `?scope=career` ou `?scope=mission` (+ `&missionId=N`). Côté interface :
le sélecteur **Carrière / Mission** en haut de l'onglet Statistiques.

## Sources et fusion

| Source | Ce qu'elle fournit de fiable |
|---|---|
| `net.get_stat` (snapshots) | Score, kills air/sol/navire, crashes, atterrissages, éjections, ping |
| `onGameEvent` | Détail des kills (armes, cibles), friendly-fire, morts |
| `debrief.log` | Chronologie complète de la mission |

Le score et les compteurs de kills viennent des **snapshots DCS** (source
autoritaire). Les détails (arme utilisée, type de cible) viennent des
**événements**. C'est la fusion annoncée depuis le début.

## Relier un événement à un joueur

Les arguments d'événement DCS sont des **identifiants numériques de joueur**, pas
des noms. Pour pouvoir attribuer une mort ou un friendly-fire à un pilote, le
backend enregistre l'ID DCS de session (`dcs_player_id`) à côté de chaque snapshot
de statistiques, puis **jointure** l'événement sur cet ID.

C'est ce qui permet, par exemple, de calculer le **K/D** par pilote alors que DCS
ne fournit pas directement le nombre de morts dans `net.get_stat`.

## Les modules

| Module | Route | Contenu |
|---|---|---|
| Vue d'ensemble | `/api/stats/overview` | Missions, pilotes, kills, morts, crashes, éjections, friendly-fire, coalitions |
| **4.1 Pilotes** | `/api/stats/pilots` | Score, kills, morts, K/D, atterrissages, éjections, crashes, FF, ping moyen |
| **4.2 Armes** | `/api/stats/weapons` | Kills et friendly-fire par arme, répartition des cibles et des plateformes |
| **4.4 Balance** | (dans overview) | Score/kills par coalition, barres comparatives |
| **4.6 Réseau** | `/api/stats/network` | Ping moyen/max et nombre d'échantillons par pilote |
| **4.7 Engins** | `/api/stats/engines` | Par **type DCS exact** : kills, pertes, sorties, K/D |

### Granularité des engins

Conformément au plan, les engins sont agrégés par **type DCS exact**
(`F-16C_50`, `Su-27`, `T-72B`, `Mi-24P`…), sans regroupement par famille. Le type
est fourni par le hook Lua qui résout le slot du joueur via
`Sim.getAvailableSlots` (le `slotID` seul n'est qu'un identifiant opaque).

## Limites assumées

- **4.3 Cartes analytiques** (heatmaps, traces) et **4.5 Analyse de sortie**
  (télémétrie : altitude/vitesse/G max) nécessitent l'enregistrement des positions
  par unité et l'export ownship. Ils s'appuieront sur les positions lat/lng déjà
  reçues par la live map plutôt que sur les coordonnées internes du débrief, ce qui
  évitera toute projection par théâtre. À venir dans un incrément dédié.
- Le **friendly-fire** n'est compté que depuis les événements ; le débrief ne le
  détaille pas toujours.
- Les **morts** (`pilot_death`) ne sont attribuées que si le joueur a un snapshot
  de statistiques dans la même mission.

## Tests

`internal/stats` couvre : pilotes (carrière et mission), armes, engins, coalitions,
réseau, vue d'ensemble, et la résolution du type d'appareil.
