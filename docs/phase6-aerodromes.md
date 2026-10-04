# Aerodromes (Phase 6)

## Where the data comes from

DCS **does not expose radio frequencies at runtime** via its Lua API. The only
reliable source is the **official aerodrome documentation** (VAD
approach/departure charts, GND ground plans), which carries for each airfield:

- the **coordinates** (CRP, runway thresholds);
- **Tower**, **Radar**, **Final/Precision**;
- **TACAN** (channel + identifier, e.g. `16X BTM`);
- **ILS** per runway (e.g. `110.30 MHz`).

The embedded dataset (`internal/aerodrome/data/*.json`) was **extracted from the
charts provided** in `maps_dcs/`, not invented. Each entry references the file
names of its charts, so the original document can be found again.

## Current scope

**Caucasus**: 21 airfields (Kobuleti, Senaki, Kutaisi, Batumi, Tbilissi Lochini and
Soganlug, Vaziani, Gudauta, Sukhumi, Anapa, Gelendzhik, Maykop, Krasnodar
Pashkovsky and Center, Novorossiysk, Krymsk, Mineralnye Vody, Nalchik, Beslan,
Sochi-Adler, Mozdok).

**Cold War Germany** (`GermanyCW`): 119 airfields, extracted from DCS's own
terrain files. DCS gives every field a Tower frequency but a position only when
a navigation aid exists, so 77 carry coordinates and 42 are listed without one.
Regenerate the dataset with
`go run ./cmd/gen-aerodrome "<DCS>/Mods/terrains" GermanyCW internal/aerodrome/data/germanycw.json`.

The format is **generic**: adding a theatre = dropping a
`internal/aerodrome/data/<theatre>.json` following the same model, nothing else to
change.

## Model

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
| `GET /api/aerodromes` | All airfields (all theatres) |
| `GET /api/aerodromes?theatre=Caucasus` | Filtered by theatre |
| `GET /api/aerodromes?lat=..&lng=..` | Annotated with a **distance** and sorted by proximity |
| `GET /api/aerodromes/UGSB` | A single airfield by code (case-insensitive) |

## Interface

New **Aerodromes** tab:

- list filterable by **name, ICAO code or TACAN**;
- **“Near me”** button: sorts by distance to the player's aircraft;
- detail sheet: coalition, coordinates, elevation, runway, **Tower**, **TACAN**,
  **ILS** per runway, and the list of available **charts** (shown in a viewer).

## Charts

The scans are **not embedded** in the binary (≈1.2 GB). The index references
their file name in `maps_dcs/`; this is deliberate, to keep the binary
light and not redistribute documents potentially under copyright.

## Accepted limitations

- **Radar** and **Final/Precision** frequencies are present on the charts
  but often **empty** in 2012 (they had not yet been assigned): the
  corresponding entries are omitted rather than filled arbitrarily.
- The coordinates come from the `RWY` line of the charts and were converted
  to decimal degrees; the **CRP** (reference point) may differ slightly from the
  visual centre of the airfield.
- Only the **Caucasus** and **Cold War Germany** are populated in the bundled
  dataset; every other map is read live from DCS when it is installed.
- **42 of the 119 Cold War Germany airfields have no coordinates**: DCS records a
  position only for a field that carries a navigation aid, and those fields have
  a tower frequency but no beacon.

## Tests

`internal/aerodrome`: loading, known airfield (Batumi: Tower 131.0, TACAN
`16X BTM`, ILS 110.3), completeness (id/name/theatre/coordinates/frequency
present), sorting by name, filtering by theatre.
