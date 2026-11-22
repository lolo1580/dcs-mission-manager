#!/usr/bin/env python3
"""Generate the embedded airport dataset from OurAirports.

OurAirports publishes its CSV files to the public domain (Unlicense). This tool
keeps only the fields flight planning needs, so the binary grows by about 1.7 MB
instead of 12.7 MB.

Usage:
  python tools/gen-airports.py                 # downloads airports.csv
  python tools/gen-airports.py --csv path.csv  # uses a local copy
  python tools/gen-airports.py --out other.gz
"""

from __future__ import annotations

import argparse
import csv
import gzip
import io
import json
import os
import sys
import urllib.request
from datetime import date as date_module

SOURCE_URL = "https://davidmegginson.github.io/ourairports-data/airports.csv"
DEFAULT_OUT = os.path.join(
    os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
    "backend", "internal", "airportdata", "airports.json.gz",
)

KEEP_TYPES = {
    "large_airport": "large",
    "medium_airport": "medium",
    "small_airport": "small",
}


def download(url: str) -> bytes:
    print(f"downloading {url}", file=sys.stderr)
    with urllib.request.urlopen(url, timeout=120) as response:
        return response.read()


def compact(row: dict[str, str]) -> dict | None:
    kind = KEEP_TYPES.get(row.get("type", ""))
    if kind is None:
        return None
    try:
        lat = float(row.get("latitude_deg") or "")
        lon = float(row.get("longitude_deg") or "")
    except ValueError:
        return None
    ident = (row.get("ident") or "").strip()
    if not ident:
        return None
    elevation = None
    raw_elevation = (row.get("elevation_ft") or "").strip()
    if raw_elevation:
        try:
            elevation = int(float(raw_elevation))
        except ValueError:
            elevation = None
    icao = (row.get("icao_code") or "").strip().upper()
    gps = (row.get("gps_code") or "").strip().upper()
    if not icao and len(gps) == 4 and gps.isalpha():
        icao = gps
    if not icao and len(ident) == 4 and ident.isalpha():
        icao = ident.upper()
    return {
        "ident": ident.upper(),
        "icao": icao,
        "iata": (row.get("iata_code") or "").strip().upper(),
        "name": (row.get("name") or "").strip(),
        "type": kind,
        "lat": round(lat, 6),
        "lon": round(lon, 6),
        "elevationFt": elevation,
        "country": (row.get("iso_country") or "").strip().upper(),
        "municipality": (row.get("municipality") or "").strip(),
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--csv", help="local airports.csv")
    parser.add_argument("--out", default=DEFAULT_OUT)
    parser.add_argument("--date", help="dataset date (default: from the file mtime)")
    args = parser.parse_args()

    if args.csv:
        with open(args.csv, "rb") as handle:
            raw = handle.read()
        date = args.date or date_module.today().isoformat()
    else:
        raw = download(SOURCE_URL)
        date = args.date or date_module.today().isoformat()

    rows: list[dict] = []
    reader = csv.DictReader(io.StringIO(raw.decode("utf-8-sig")))
    for row in reader:
        item = compact(row)
        if item is not None:
            rows.append(item)
    rows.sort(key=lambda item: item["ident"])

    payload = {
        "source": "OurAirports",
        "license": "Public domain (Unlicense)",
        "generated": date or "",
        "airports": rows,
    }
    encoded = json.dumps(payload, separators=(",", ":"), ensure_ascii=False).encode("utf-8")

    os.makedirs(os.path.dirname(args.out), exist_ok=True)
    with gzip.open(args.out, "wb", compresslevel=9) as handle:
        handle.write(encoded)

    size = os.path.getsize(args.out)
    print(f"{len(rows)} airports -> {args.out} ({size} bytes, {len(encoded)} raw)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
