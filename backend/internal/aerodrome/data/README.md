# Aerodrome dataset — provenance

`caucasus.json` holds the Caucasus airfield reference data (coordinates,
elevation, runway, Tower, TACAN, ILS) used by the **Airfields** tab.

## Source

The values were **transcribed from official aerodrome approach charts**
(ARR/DEP, VAD and GND plates) covering the Caucasus theatre. DCS does not expose
radio frequencies through its Lua API at runtime, so a curated dataset is the
only reliable source.

Each entry keeps a `charts` field listing the plate file names it came from, so
any value can be traced back to its document.

## Nature of the data

- The **individual values are facts** (a frequency, a coordinate) and are not
  themselves copyrightable.
- The **selection and arrangement** may however fall under the European Union's
  *sui generis* database right. This file is therefore distributed as part of
  the project, with the source stated here, and not as a standalone product.

## Accuracy

The plates are dated 2012. Most frequencies still match DCS, but a few may have
changed. Corrections are welcome: edit the JSON and open a pull request.

## Adding a theatre

Drop a new `<theatre>.json` in this directory following the same shape; the
loader picks up every `*.json` file automatically. See
`docs/phase6-aerodromes.md` for the fields.
