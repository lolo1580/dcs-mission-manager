# Fog of war (respect des options de mission)

## Le problème

Une mission DCS peut **limiter ce que la carte F10 révèle** (« F10 View Options »).
Un outil de live map qui ignore ces options **dévoile ce que la mission cache** :
c'est de la triche involontaire, en particulier sur un serveur multijoueur.

C'est le seul écart que nous avions vis-à-vis de la référence (Bergison
MovingMap, MizMap), et c'est un problème d'**intégrité**, pas de confort. Il est
donc traité en priorité.

## D'où viennent les valeurs

Directement de **DCS**, dans `MissionEditor/modules/Options/optionsDb.lua` :

| Valeur `optionsView` | Libellé DCS | Signification |
|---|---|---|
| `optview_onlymap` | MAP ONLY | aucune unité |
| `optview_myaircraft` | MY A/C | mon appareil seul |
| `optview_allies` | **FOG OF WAR** | alliés + contacts détectés par les capteurs |
| `optview_onlyallies` | ALLIES ONLY | alliés uniquement |
| `optview_all` | ALL | tout |

Le hook Lua transmet `Sim.getMissionOptions()` au démarrage de mission ; le
backend en déduit la politique de visibilité.

## Principe : restrictif par défaut

`internal/visibility` applique une règle simple : **ne jamais montrer plus que
ce que DCS montrerait**.

- `map_only` / `my_aircraft` → l'appareil du joueur uniquement ;
- `allies` / `only_allies` → l'appareil du joueur + sa **coalition** ;
- `all` → tout ;
- **options non reçues** → traité comme `allies` (le cas sûr).

Deux choix explicites :

- **Le filtrage est côté serveur.** Filtrer dans le navigateur serait
  contournable : les données ne doivent jamais quitter le backend si la mission
  les cache.
- **« FOG OF WAR » est approximé en « alliés uniquement ».** Les contacts
  réellement détectés dépendent des capteurs de chaque coalition à l'instant T,
  que l'export Lua ne fournit pas. On prend donc le **sous-ensemble sûr** : on
  sous-affiche plutôt que de sur-afficher.

L'appareil du joueur est **toujours** conservé : montrer sa propre position n'est
jamais une fuite, et sans elle la carte serait inutilisable.

## Configuration

| Variable | Défaut | Effet |
|---|---|---|
| `DCSMM_REVEAL_ALL_UNITS` | `false` | `true` désactive le filtrage (usage solo / conception de mission) |

Quand le filtrage est désactivé, l'interface l'indique clairement : c'est un mode
« dieu », assumé et visible.

## API

| Route | Description |
|---|---|
| `GET /api/visibility` | Politique active (mode, libellé, dérogation, note) |

La politique est aussi jointe à chaque trame d'état (`/api/state`, SSE) sous la
clé `visibility`, et un événement `visibility` est diffusé à chaque changement.

## Interface

Un bandeau apparaît sous l'en-tête dès que le filtrage est actif, avec le mode et
sa limite (par exemple : « Fog of war (alliés) — les contacts détectés par les
capteurs ne sont pas reproduits (restrictif) »).

## Limites assumées

- **Fog of war réel non reproduit** : sans les détections capteur, on ne peut pas
  montrer les contacts ennemis détectés. C'est une limite de l'export Lua, pas un
  choix de conception.
- Le filtrage ne dépend que de la **coalition** ; il ne tient pas compte des
  masques de couches par rôle (`visibleUnitLayersMask`), plus fins, que DCS
  applique aussi.

## Tests

`internal/visibility` : correspondance des valeurs DCS, filtrage par mode,
**non-fuite des unités ennemies et neutres** dans tous les modes restrictifs,
dérogation, absence d'appareil joueur, description.
