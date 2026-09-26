"""Inspect DCS reference map scans and dump corner crops.

Useful when preparing a map for tile extraction: it reports each image's size,
metadata (georeferencing is usually absent) and saves readable corner crops so
the graticule labels can be read for manual calibration.

Usage:
    python tools/inspect-maps.py --dir maps_dcs --out %TEMP%/dcsmm-corners
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

from PIL import Image

Image.MAX_IMAGE_PIXELS = None

IMAGE_SUFFIXES = {".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp", ".bmp"}


def describe(path: Path, root: Path) -> dict:
    info: dict = {"path": str(path.relative_to(root)), "mb": round(path.stat().st_size / 1e6, 1)}
    try:
        with Image.open(path) as im:
            info["size"] = list(im.size)
            info["mode"] = im.mode
            info["format"] = im.format
            exif = im.getexif()
            info["exif_tags"] = len(exif) if exif else 0
            info["icc_profile"] = bool(im.info.get("icc_profile"))
            dpi = im.info.get("dpi")
            info["dpi"] = [float(x) for x in dpi] if dpi else None
    except Exception as exc:  # noqa: BLE001 - report and continue
        info["error"] = f"{type(exc).__name__}: {exc}"
    return info


def corner_crops(path: Path, tag: str, out_dir: Path, fraction: float) -> list[str]:
    written: list[str] = []
    try:
        with Image.open(path) as im:
            im = im.convert("RGB")
            w, h = im.size
            cw, ch = int(w * fraction), int(h * fraction)
            regions = {
                "tl": (0, 0, cw, ch),
                "tr": (w - cw, 0, w, ch),
                "bl": (0, h - ch, cw, h),
                "br": (w - cw, h - ch, w, h),
            }
            for name, box in regions.items():
                crop = im.crop(box)
                crop.thumbnail((1200, 1200), Image.LANCZOS)
                dest = out_dir / f"{tag}_{name}.png"
                crop.save(dest)
                written.append(str(dest))
    except Exception as exc:  # noqa: BLE001
        written.append(f"ERROR {type(exc).__name__}: {exc}")
    return written


def slug(text: str) -> str:
    return "".join(c if c.isalnum() else "_" for c in text).strip("_").lower()


def main() -> int:
    parser = argparse.ArgumentParser(description="Inspects DCS map scans.")
    parser.add_argument("--dir", type=Path, default=Path("maps_dcs"))
    parser.add_argument("--out", type=Path, default=Path("corners"))
    parser.add_argument("--fraction", type=float, default=0.16,
                        help="fraction of the image captured at each corner")
    parser.add_argument("--crops", action="store_true", help="write the corner crops")
    parser.add_argument("--json", action="store_true", help="JSON output")
    args = parser.parse_args()

    root = args.dir
    if not root.is_dir():
        print(f"directory not found: {root}")
        return 1

    images = sorted(p for p in root.rglob("*") if p.suffix.lower() in IMAGE_SUFFIXES)
    if not images:
        print(f"no images under {root}")
        return 1

    if args.crops:
        args.out.mkdir(parents=True, exist_ok=True)

    results = []
    for path in images:
        d = describe(path, root)
        if args.crops and "error" not in d:
            d["crops"] = corner_crops(path, slug(path.stem), args.out, args.fraction)
        results.append(d)
        if not args.json:
            print(f"{d['path']}: {d.get('size')} {d.get('mb')} MB exif={d.get('exif_tags')}")

    if args.json:
        print(json.dumps(results, ensure_ascii=False, indent=2))

    total = sum(r["mb"] for r in results)
    print(f"\n{len(results)} images, {total:.0f} MB total")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
