# Cartes de référence (scans)

Ce dossier contient des **scans de cartes aéronautiques** (JNC/ONC) de plusieurs
théâtres DCS, à différentes résolutions. Ils **ne sont pas versionnés** dans git
(~1,3 Go) et restent locaux.

## Contenu

### Cartes générales (scans pleine page)

| Dossier | Thématique |
|---|---|
| `DCS Caucasus Maps/` | Caucase (le scan principal couvre en réalité une zone bien plus large) |
| `DCS Nevada Maps/` | Nevada / NTTR |
| `DCS Normandy Maps/` | Normandie |
| `DCS PersianGulfMaps/` | Golfe Persique |
| `DCS_Syria_High_Detail_Maps/` | Syrie |
| `DCS_The_Channel_High_Detail_Map/` | La Manche |
| `Marianas_High_Detail_Maps/` | Mariannes (contient aussi `The Channel 8M.jpg`) |

### Cartes aérodromes et procédures (Caucase)

`DCS Caucasus Maps/` contient en plus une série de cartes **par aérodrome** :

- `00_*` : carte générale et légendes.
- `NN_GND_*` : plans au sol (*ground movement*).
- `NN_VAD_*` : cartes d'approche/départ à vue (*Visual Operation Chart*).
- `NN_PAR_*` : procédures.

Chaque carte indique les coordonnées (CRP), fréquences (Tower, Radar, TACAN, ILS)
et la piste. Utile pour une future section **briefings / charts** — ce ne sont pas
des fonds de carte.

## Important : ces scans ne sont pas utilisables tels quels

1. **Non géoréférencés** — aucune métadonnée de calibration (pas de `.jgw`, pas de
   GeoTIFF). Les coordonnées des coins sont inconnues du logiciel.
2. **Projection conique** (type Lambert) — les bords sont courbes, alors que
   Leaflet attend du **Web Mercator (EPSG:3857)**. Une superposition directe est
   impossible sans reprojection.
3. **Cartes du monde réel** — ce ne sont pas les tuiles F10 de DCS. Le rendu
   correspond approximativement, mais pas au pixel.

### Cartes aérodromes et procédures

Les cartes d'aérodrome (`NN_VAD_*`, `NN_GND_*`) sont des **documents** à consulter,
pas des fonds de carte géoréférencés. Une future section « briefings / charts »
pourra les afficher telles quelles. `tools/inspect-maps.py` peut les recenser.

## Voies d'utilisation

- **A. GeoTIFF** — si tu obtiens ces cartes en GeoTIFF géoréférencé (EPSG:4326 ou
  3857), elles se transforment en tuiles proprement (gdal2tiles ou un script dédié).
- **B. Image + calibration** — fournir **4 coins (lat/lng)** par image permet un
  mapping linéaire via `tools/export-tiles.py`. Suffisant pour un fond indicatif,
  insuffisant pour corriger la courbure conique.
- **C. Fond de carte réel** — c'est ce qui est actif aujourd'hui dans l'UI
  (Satellite, Relief, Routier, Sombre) : aucun pré-traitement, suffisamment précis
  pour superposer les unités.

## Droits

Les cartes aéronautiques scannées peuvent être soumises à des droits d'auteur ou à
des conditions d'utilisation selon leur source. Elles restent **locales** et ne sont
ni redistribuées ni intégrées au binaire. Vérifie la licence avant tout partage.
