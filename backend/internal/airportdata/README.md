# Airport dataset

`airports.json.gz` is generated from [OurAirports](https://ourairports.com/data).
The data is released to the **public domain** (Unlicense); attribution is not
required, but the source is kept in the payload and in `tools/gen-airports.py`.

- 48,082 large, medium and small airports with a position;
- fields: ident, ICAO, IATA, name, type, latitude, longitude, elevation, country,
  municipality;
- compact JSON, gzip-compressed to about 1.6 MB and embedded in the binary.

Regenerate after an OurAirports update:

```bash
python tools/gen-airports.py
```

or, from an already downloaded CSV:

```bash
python tools/gen-airports.py --csv airports.csv --date 2026-11-21
```

The DCS airfields are not in this file: the API merges them at request time from
`internal/aerodrome`, so they always match the installed simulator and the
selected theatre.
