# Débriefs (Phase 3)

## Le fichier `debrief.log`

À la fin de chaque mission, DCS écrit
`%USERPROFILE%\Saved Games\DCS\Logs\debrief.log`. C'est un **dump de table Lua**,
pas du JSON. Il contient notamment :

| Clé | Contenu |
|---|---|
| `mission_file_path` | Chemin du `.miz` joué |
| `mission_time` | Durée de la mission (secondes) |
| `result` | Résultat |
| `world_state` | **État final** de toutes les unités (type, coalition, x/y, alt, mort ou non) |
| `events` | **Chronologie ordonnée** : `mission start`, `takeoff`, `land`, `engine shutdown`, `mission end`, et selon les missions `kill`, `crash`, `eject`, `pilot dead`… |

Exemple d'événement :

```lua
[4] =
{
    type = "takeoff",
    initiatorPilotName = "Cellar",
    place = "Mineralnye Vody",
    t = 43.42,
    initiator_unit_type = "M-2000C",
    event_id = 36,
    initiator_coalition = 2,
    initiatorMissionID = "25",
},
```

### Précision importante

DCS écrit ce fichier **à la fin de la mission**, et il est **écrasé** à chaque
nouvelle mission. C'est pourquoi le hook Lua le capture et l'envoie
immédiatement : sinon l'historique serait perdu.

## Parsing

`debrief.log` n'est pas du JSON, donc plutôt qu'une expression régulière fragile,
le backend embarque un **parseur Lua minimal** (`internal/lua`) : il lit les
assignations `nom = valeur` et les tables imbriquées, **sans exécuter de Lua**.
C'est sûr (aucune évaluation de code) et suffisant pour ce format.

Le paquet `internal/debrief` transforme ensuite la table brute en structure
typée (événements, état du monde, agrégats) via `ToModel()`.

## Transport réseau

Le fichier peut dépasser 1 Mo. Il est envoyé en **morceaux** :

```
Hooks/dcsmm.lua                      Backend
  lit debrief.log
  → découpe en morceaux de 32 Ko
  → base64 par morceau
  → {"type":"debrief", transferId, chunk, chunks, size, data}
                                    → internal/debriefstore réassemble
                                    → internal/debrief parse
                                    → SQLite (internal/db)
```

- **base64** : garantit que n'importe quel octet traverse le canal NDJSON intact.
- **`transferId`** : plusieurs transferts peuvent être en cours sans se mélanger.
- **Réassemblage dans l'ordre des chunks**, même s'ils arrivent désordonnés.
- Un morceau invalide est journalisé et ignoré : il ne fait jamais planter le backend.

## Stockage

Table `debriefs` : métadonnées + `parsed` (JSON structuré) + `raw` (texte
original, pour re-parser plus tard si le format évolue).

## API Web

| Route | Description |
|---|---|
| `GET /api/debriefs` | Liste des débriefs (métadonnées) |
| `GET /api/debriefs/{id}` | Débrief complet (structuré) |
| `GET /api/debriefs/{id}?raw=1` | Ajoute le texte original |

## Interface

L'onglet **Débriefs** affiche la liste à gauche et, pour le débrief sélectionné :

- des compteurs (décollages, atterrissages, kills, crashes, éjections, durée) ;
- la liste des pilotes ;
- la **chronologie** minute par minute, colorée par type d'événement.

## Test sans DCS

```bash
node tools/send-debrief.mjs
# ou avec un fichier et une cible explicites :
node tools/send-debrief.mjs "%USERPROFILE%\Saved Games\DCS\Logs\debrief.log" 127.0.0.1 7779
```

## Limites connues

- Le `debrief.log` ne contient pas toujours les kills (cela dépend de la mission
  et de la version) ; les événements temps réel de la Phase 2 comblent ce manque,
  ce qui justifie la **fusion des deux sources** prévue pour les statistiques.
- Le fichier est écrit en fin de mission uniquement : pas de débrief « en cours ».
