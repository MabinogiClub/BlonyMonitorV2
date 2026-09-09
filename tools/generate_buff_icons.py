"""Crop CharacterCondition icons from unpacked Mabinogi sprite sheets."""

from __future__ import annotations

import argparse
import sys
from pathlib import Path

from PIL import Image

sys.path.insert(0, str(Path(__file__).resolve().parent))
from update_mabidata_from_pack import resolve_pack_file, unique_rows

CONDITION_ICON_SIZE = 16


def pack_data_dir(source: Path) -> Path:
    if (source / "data" / "db" / "CharacterCondition.xml").exists():
        return source / "data"
    if (source / "db" / "CharacterCondition.xml").exists():
        return source
    raise SystemExit(f"missing CharacterCondition.xml under {source}")


def generate(source: Path, output: Path, overwrite: bool = False) -> list[int]:
    data = pack_data_dir(source)
    rows = unique_rows([data / "db" / "CharacterCondition.xml"], "CharacterCondition", "ConditionID")
    output.mkdir(parents=True, exist_ok=True)

    sheets: dict[str, Image.Image] = {}
    written: list[int] = []
    for raw_id, attrs in rows.items():
        condition_id = int(raw_id)
        destination = output / f"{condition_id}.png"
        if destination.exists() and not overwrite:
            continue

        image_file = attrs.get("ImageFile", "")
        if not image_file:
            continue
        sheet_path = resolve_pack_file(data, image_file)
        if not sheet_path.exists():
            continue
        try:
            if image_file not in sheets:
                sheets[image_file] = Image.open(sheet_path).convert("RGBA")
            sheet = sheets[image_file]
            x = int(attrs.get("PositionX", "0")) * CONDITION_ICON_SIZE
            y = int(attrs.get("PositionY", "0")) * CONDITION_ICON_SIZE
            crop = sheet.crop((x, y, x + CONDITION_ICON_SIZE, y + CONDITION_ICON_SIZE))
            if crop.size != (CONDITION_ICON_SIZE, CONDITION_ICON_SIZE):
                continue
            crop.save(destination, format="PNG", optimize=True)
            written.append(condition_id)
        except (OSError, ValueError):
            continue
    return written


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("data_root", type=Path, help="Unpacked pack root or its data/ directory")
    parser.add_argument(
        "--output",
        type=Path,
        default=Path(__file__).resolve().parents[1] / "frontend" / "public" / "buff-icons",
        help="PNG output directory (defaults to frontend/public/buff-icons)",
    )
    parser.add_argument("--overwrite", action="store_true", help="Replace existing icon files")
    return parser.parse_args()


if __name__ == "__main__":
    arguments = parse_args()
    created = generate(arguments.data_root.resolve(), arguments.output.resolve(), arguments.overwrite)
    print(f"wrote {len(created)} icons")
    if created:
        print(", ".join(str(condition_id) for condition_id in sorted(created)))
