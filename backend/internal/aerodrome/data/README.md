# Aerodrome dataset — provenance

This directory holds the airfield reference data used by the **Airfields** tab
when DCS itself cannot be read (no installation found). One file per theatre:

| File | Theatre | Entries | With position | Source |
|---|---|---:|---:|---|
| `caucasus.json` | `Caucasus` | 21 | 21 | Approach charts (VAD/GND), transcribed by hand |
| `germanycw.json` | `GermanyCW` (`Cold War Germany`) | 119 | 77 | Extracted from DCS's own terrain files |

Each entry keeps the fields the UI shows: coordinates, elevation, runway,
Tower, TACAN, ILS, VOR, RSBN and NDB.

## Source

### `caucasus.json`

The values were **transcribed from official aerodrome approach charts**
(ARR/DEP, VAD and GND plates) covering the Caucasus theatre. DCS does not expose
radio frequencies through its Lua API at runtime, so a curated dataset is the
only reliable source.

Each entry keeps a `charts` field listing the plate file names it came from, so
any value can be traced back to its document.

### `germanycw.json`

The values were **extracted from the Cold War Germany terrain** shipped with DCS
(`Mods/terrains/GermanyColdWar/radio.lua` and `beacons.lua`). Nothing was typed
by hand: regenerate the file with

```sh
go run ./cmd/gen-aerodrome "<DCS>/Mods/terrains" GermanyCW internal/aerodrome/data/germanycw.json
```

DCS gives every field a Tower frequency in `radio.lua`, but a position only
comes from `beacons.lua`, which has no entry for an airfield without a
navigation aid. **77 of the 119 fields therefore have coordinates; the other 42
are listed with their frequency but cannot be placed on the map.** That gap is
DCS's own, not a transcription shortcut. Runway and ILS runway designators are
also absent for this theatre (DCS does not map them here), so they are left
empty rather than guessed.

Two DCS quirks are handled while extracting, because both silently cost a field
its TACAN: `BEACON_TYPE_AIRPORT_TACAN` (the field's own TACAN, e.g. Nordholz
118X NDO) is treated like any other TACAN, and a `world_*` beacon that names its
field instead of carrying an airfield id (Hamburg 78X HAM, Fulda 58X FUL) is
attached by name. The map ends up with 22 TACAN and 15 VOR entries.

Because this file is embedded in the binary, it should be refreshed when DCS
patches the terrain. The generator can be pointed at any theatre.

## Nature of the data

- The **individual values are facts** (a frequency, a coordinate) and are not
  themselves copyrightable.
- The **selection and arrangement** may however fall under the European Union's
  *sui generis* database right. These files are therefore distributed as part of
  the project, with the source stated here, and not as a standalone product.

## Accuracy

The Caucasus plates are dated 2012. Most frequencies still match DCS, but a few
may have changed. The Germany data follows DCS directly. Corrections are
welcome: edit the JSON (or regenerate it) and open a pull request.

## Adding a theatre

Drop a new `<theatre>.json` in this directory following the same shape; the
loader picks up every `*.json` file automatically. See
`docs/phase6-aerodromes.md` for the fields.
