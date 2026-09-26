# Aérodromes (Phase 6)

## D'où viennent les données

DCS **n'expose pas les fréquences radio à l'exécution** via son API Lua. La seule
source fiable est la **documentation aérodrome officielle** (cartes
d'approche/départ VAD, plans au sol GND), qui porte pour chaque terrain :

- les **coordonnées** (CRP, seuils de piste) ;
- **Tower**, **Radar**, **Final/Precision** ;
- **TACAN** (canal + identifiant, ex. `16X BTM`) ;
- **ILS** par piste (ex. `110.30 MHz`).

Le jeu de données embarqué (`internal/aerodrome/data/*.json`) a été **extrait des
cartes fournies** dans `maps_dcs/`, pas inventé. Chaque entrée référence les noms
de fichiers de ses cartes, pour retrouver le document d'origine.

## Périmètre actuel

**Caucase** : 21 terrains (Kobuleti, Senaki, Kutaisi, Batumi, Tbilissi Lochini et
Soganlug, Vaziani, Gudauta, Sukhumi, Anapa, Gelendzhik, Maykop, Krasnodar
Pashkovsky et Center, Novorossiysk, Krymsk, Mineralnye Vody, Nalchik, Beslan,
Sochi-Adler, Mozdok).

Le format est **générique** : ajouter un théâtre = déposer un
`internal/aerodrome/data/<theatre>.json` sur le même modèle, rien d'autre à
changer.

## Modèle

```json
{
  "id": "UGSB",
  "name": "Batumi",
  "theatre": "Caucasus",
  "coalition": "blue",
  "lat": 41.610278, "lng": 41.599722,
  "elevationM": 10,
  "runway": "12/30",
  "tower": 131.0,
  "tacan": "16X BTM",
  "ils": [{ "runway": "12", "mhz": 110.3 }],
  "charts": ["07_VAD_UGSB_Batumi.png", "07_GND_UGSB_Batumi_18.png"]
}
```

## API

| Route | Description |
|---|---|
| `GET /api/aerodromes` | Tous les terrains (tous théâtres) |
| `GET /api/aerodromes?theatre=Caucasus` | Filtré par théâtre |
| `GET /api/aerodromes?lat=..&lng=..` | Annoté d'une **distance** et trié par proximité |
| `GET /api/aerodromes/UGSB` | Un terrain par code (insensible à la casse) |

## Interface

Nouvel onglet **Aérodromes** :

- liste filtrable par **nom, code OACI ou TACAN** ;
- bouton **« Proches de moi »** : trie par distance à l'appareil du joueur ;
- fiche détaillée : coalition, coordonnées, élévation, piste, **Tower**, **TACAN**,
  **ILS** par piste, et la liste des **cartes** disponibles ;
- case **« Sur la carte »** : affiche les terrains comme marqueurs, avec les
  fréquences en infobulle.

## Cartes

Les scans ne sont **pas embarqués** dans le binaire (≈1,2 Go). Le JSON référence
leur nom de fichier dans `maps_dcs/` ; c'est volontaire, pour garder le binaire
léger et ne pas redistribuer des documents potentiellement sous droits.

## Limites assumées

- Les fréquences **Radar** et **Final/Precision** sont présentes sur les cartes
  mais souvent **vides** en 2012 (elles n'étaient pas encore attribuées) : les
  entrées correspondantes sont omises plutôt que remplies arbitrairement.
- Les coordonnées proviennent de la ligne `RWY` des cartes et ont été converties
  en degrés décimaux ; le **CRP** (point de référence) peut différer légèrement du
  centre visuel du terrain.
- Seul le **Caucase** est renseigné pour l'instant.

## Tests

`internal/aerodrome` : chargement, terrain connu (Batumi : Tower 131.0, TACAN
`16X BTM`, ILS 110.3), complétude (id/nom/théâtre/coordonnées/fréquence
présents), tri par nom, filtrage par théâtre.
