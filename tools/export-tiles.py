#!/usr/bin/env python3
"""DCS Mission Manager — extracteur de tuiles de carte.

Produit des tuiles PNG à partir d'une image de carte géoréférencée, pour un
théâtre donné, dans l'arborescence attendue par le backend :

    <tilesDir>/<theatre>/<z>/<x>/<y>.png

Deux modes sont prévus :

1. **Mode image** (implémenté) : découpe une grande image de carte (par
   exemple une capture de la carte F10) en tuiles façon slippy-map, en
   mappant l'emprise géographique fournie sur toute l'image.

2. **Mode DCS** (à venir) : pilote DCS World en autopilote pour capturer la
   carte F10 in-game à plusieurs niveaux de zoom, façon Olympus, puis
   assemble les captures. Cela requiert une session DCS scriptée et sort du
   périmètre de ce script.

Usage (mode image) :
    python tools/export-tiles.py --image caucasus.png --theatre Caucasus \
        --bounds 41.0,36.5,45.5,45.0 --min-zoom 2 --max-zoom 6 --out ./tiles

Les bornes sont données comme : minLat,minLng,maxLat,maxLng (degrés décimaux).
Elles doivent correspondre à l'emprise couverte par l'image.
"""

from __future__ import annotations

import argparse
import math
import sys
from pathlib import Path

try:
    from PIL import Image
except ImportError:  # pragma: no cover - depends on the user's environment
    print("Pillow est requis : pip install Pillow", file=sys.stderr)
    raise SystemExit(2)


TILE_SIZE = 256


def parse_bounds(value: str) -> tuple[float, float, float, float]:
    parts = [float(p) for p in value.split(",")]
    if len(parts) != 4:
        raise argparse.ArgumentTypeError("bounds attendu : minLat,minLng,maxLat,maxLng")
    min_lat, min_lng, max_lat, max_lng = parts
    if not (min_lat < max_lat and min_lng < max_lng):
        raise argparse.ArgumentTypeError("bounds invalides (min doit être < max)")
    return min_lat, min_lng, max_lat, max_lng


def deg2num(lat: float, lng: float, z: int) -> tuple[float, float]:
    """Convertit des coordonnées géographiques en coordonnées de tuile (Web Mercator)."""
    lat_rad = math.radians(lat)
    n = 2.0**z
    x = (lng + 180.0) / 360.0 * n
    y = (1.0 - math.asinh(math.tan(lat_rad)) / math.pi) / 2.0 * n
    return x, y


def extract(
    image_path: Path,
    theatre: str,
    bounds: tuple[float, float, float, float],
    min_zoom: int,
    max_zoom: int,
    out_dir: Path,
) -> int:
    image = Image.open(image_path).convert("RGBA")
    img_w, img_h = image.size
    min_lat, min_lng, max_lat, max_lng = bounds

    written = 0
    for z in range(min_zoom, max_zoom + 1):
        # Tile range covering the image's geographic extent at this zoom.
        x0f, y0f = deg2num(max_lat, min_lng, z)
        x1f, y1f = deg2num(min_lat, max_lng, z)

        x_start, x_end = int(math.floor(x0f)), int(math.ceil(x1f))
        y_start, y_end = int(math.floor(y0f)), int(math.ceil(y1f))

        for tx in range(x_start, x_end):
            for ty in range(y_start, y_end):
                # Geographic extent of this tile, then its pixel window in the
                # source image (linear mapping).
                tile_lng0 = tx / 2.0**z * 360.0 - 180.0
                tile_lng1 = (tx + 1) / 2.0**z * 360.0 - 180.0
                tile_lat0 = math.degrees(math.atan(math.sinh(math.pi * (1 - 2 * ty / 2.0**z))))
                tile_lat1 = math.degrees(math.atan(math.sinh(math.pi * (1 - 2 * (ty + 1) / 2.0**z))))

                def to_px(lat: float, lng: float) -> tuple[float, float]:
                    px = (lng - min_lng) / (max_lng - min_lng) * img_w
                    py = (max_lat - lat) / (max_lat - min_lat) * img_h
                    return px, py

                px0, py0 = to_px(tile_lat0, tile_lng0)
                px1, py1 = to_px(tile_lat1, tile_lng1)

                left, top = int(round(px0)), int(round(py0))
                right, bottom = int(round(px1)), int(round(py1))
                if right <= left or bottom <= top:
                    continue

                # Skip tiles fully outside the source image.
                if right <= 0 or bottom <= 0 or left >= img_w or top >= img_h:
                    continue

                crop = image.crop((left, top, right, bottom))
                if crop.size != (TILE_SIZE, TILE_SIZE):
                    crop = crop.resize((TILE_SIZE, TILE_SIZE), Image.LANCZOS)

                tile_dir = out_dir / theatre / str(z) / str(tx)
                tile_dir.mkdir(parents=True, exist_ok=True)
                crop.save(tile_dir / f"{ty}.png", "PNG", optimize=True)
                written += 1

    return written


def main() -> int:
    parser = argparse.ArgumentParser(description="Extrait des tuiles de carte DCS.")
    parser.add_argument("--image", type=Path, required=True, help="image source (PNG/JPG)")
    parser.add_argument("--theatre", required=True, help="identifiant du théâtre (ex. Caucasus)")
    parser.add_argument("--bounds", type=parse_bounds, required=True,
                        help="minLat,minLng,maxLat,maxLng de l'image")
    parser.add_argument("--min-zoom", type=int, default=2)
    parser.add_argument("--max-zoom", type=int, default=6)
    parser.add_argument("--out", type=Path, default=Path("./tiles"))
    args = parser.parse_args()

    if not args.image.exists():
        print(f"image introuvable : {args.image}", file=sys.stderr)
        return 1

    written = extract(args.image, args.theatre, args.bounds,
                      args.min_zoom, args.max_zoom, args.out)
    print(f"{written} tuiles écrites dans {args.out / args.theatre}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
