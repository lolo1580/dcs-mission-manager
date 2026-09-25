# Live map

## Ce qui est affiché

La carte reçoit l'état complet des unités via **Server-Sent Events** (`GET /api/events`),
rafraîchi côté serveur une fois par seconde. Chaque unité porte :

| Champ | Description |
|---|---|
| `id` | Identifiant runtime DCS (`ownship` pour le joueur) |
| `type` | **Type DCS exact** (`F-16C_50`, `T-72B`, `SA-10`, `USS_Arleigh_Burke`…) |
| `label` | Nom optionnel (pseudo du pilote pour l'appareil du joueur) |
| `category` | Famille : `plane`, `heli`, `ground`, `ship`, `structure`, `other` |
| `coalition` | `blue`, `red`, `neutral` |
| `country` | Pays DCS |
| `lat`, `lng`, `alt` | Position |
| `heading` | Cap en degrés |
| `ownship` | `true` pour l'appareil du joueur |
| `ageMs` | Ancienneté de la dernière mise à jour |

## Classification des engins

La famille (`category`) est déduite du **type DCS** par des règles heuristiques
(`backend/internal/category`). Ces règles couvrent l'essentiel, mais peuvent être
**surchargées** précisément, par un fichier JSON :

```jsonc
// categories.json (chemin via DCSMM_CATEGORIES, défaut ./categories.json)
{
  "F-16C_50": "plane",
  "SA-10": "ground",
  "USS_Arleigh_Burke": "ship"
}
```

Un modèle est fourni : `categories.example.json`.

## Filtres et recherche

- **Coalitions** : Bleu / Rouge / Neutre, avec compteurs en direct.
- **Catégories** : Avions / Hélicoptères / Sol / Navires / Structures / Autres.
- **Recherche** : porte sur le type, le nom et le pays.
- **Mon appareil uniquement** : ne garde que le joueur.
- **Traces de vol** : polylignes pour avions et hélicoptères.

Tous les filtres sont appliqués **côté navigateur** ; l'API expose aussi les mêmes
critères en paramètres de requête (`?category=`, `?coalition=`, `?ownship=true`, `?q=`).

## Traces de vol

Les traces sont conservées côté client (120 points glissants par appareil), donc
elles se remplissent au fil de la session et ne survivent pas à un rechargement.
L'historique persistant (rejouable) relève de la Phase 3/4.

## Tuiles de carte

Par ordre de priorité :

1. **Tuiles DCS authentiques** — si un dossier `<tilesDir>/<theatre>/` existe, le
   backend les sert via `/api/tiles/<theatre>/<z>/<x>/<y>.png` et la carte les utilise.
2. **Fond de carte réel** — sinon, un fond OSM (ou l'URL de `DCSMM_BASEMAP_URL`) sert
   de repli.

L'extracteur `tools/export-tiles.py` découpe une image de carte géoréférencée en
tuiles au format attendu. L'export direct depuis DCS (captures F10 multi-zoom) est
prévu comme évolution.

```bash
python tools/export-tiles.py \
  --image caucasus.png --theatre Caucasus \
  --bounds 41.0,36.5,45.5,45.0 \
  --min-zoom 2 --max-zoom 6 --out ./tiles
```

## Choix : SSE plutôt que WebSocket

Les mises à jour de la Phase 1 sont **descendantes uniquement**. SSE s'intègre
nativement au navigateur (`EventSource`), se reconnecte tout seul et évite la
complexité d'un WebSocket. Le canal **bidirectionnel** (commandes vers DCS) arrivera
en Phase 2 via un socket TCP dédié côté Lua.
