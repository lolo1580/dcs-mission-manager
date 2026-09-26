#!/usr/bin/env python3
"""DCS Mission Manager — map tile extractor.

Produces PNG tiles from a georeferenced map image, for a given
theatre, into the directory tree expected by the backend:

    <tilesDir>/<theatre>/<z>/<x>/<y>.png

Two modes are planned:

1. **Image mode** (implemented): slices a large map image (for
   example a capture of the F10 map) into slippy-map tiles, mapping
   the provided geographic extent over the whole image.

2. **DCS mode** (coming): drives DCS World on autopilot to capture the
   in-game F10 map at several zoom levels, Olympus-style, then
   assembles the captures. This requires a scripted DCS session and is
   outside the scope of this script.

Usage (image mode):
    python tools/export-tiles.py --image caucasus.png --theatre Caucasus \
        --bounds 41.0,36.5,45.5,45.0 --min-zoom 2 --max-zoom 6 --out ./tiles

Bounds are given as: minLat,minLng,maxLat,maxLng (decimal degrees).
They must match the extent covered by the image.
"""

from __future__ import annotations

import argparse
import math
import sys
from pathlib import Path

try:
    from PIL import Image
except ImportError:  # pragma: no cover - depends on the user's environment
    print("Pillow is required: pip install Pillow", file=sys.stderr)
    raise SystemExit(2)


TILE_SIZE = 256


def parse_bounds(value: str) -> tuple[float, float, float, float]:
    parts = [float(p) for p in value.split(",")]
    if len(parts) != 4:
        raise argparse.ArgumentTypeError("expected bounds: minLat,minLng,maxLat,maxLng")
    min_lat, min_lng, max_lat, max_lng = parts
    if not (min_lat < max_lat and min_lng < max_lng):
        raise argparse.ArgumentTypeError("invalid bounds (min must be < max)")
    return min_lat, min_lng, max_lat, max_lng


def deg2num(lat: float, lng: float, z: int) -> tuple[float, float]:
    """Converts geographic coordinates to tile coordinates (Web Mercator)."""
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
    parser = argparse.ArgumentParser(description="Extracts DCS map tiles.")
    parser.add_argument("--image", type=Path, required=True, help="source image (PNG/JPG)")
    parser.add_argument("--theatre", required=True, help="theatre identifier (e.g. Caucasus)")
    parser.add_argument("--bounds", type=parse_bounds, required=True,
                        help="minLat,minLng,maxLat,maxLng of the image")
    parser.add_argument("--min-zoom", type=int, default=2)
    parser.add_argument("--max-zoom", type=int, default=6)
    parser.add_argument("--out", type=Path, default=Path("./tiles"))
    args = parser.parse_args()

    if not args.image.exists():
        print(f"image not found: {args.image}", file=sys.stderr)
        return 1

    written = extract(args.image, args.theatre, args.bounds,
                      args.min_zoom, args.max_zoom, args.out)
    print(f"{written} tiles written to {args.out / args.theatre}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
