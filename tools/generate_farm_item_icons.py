"""Generate farm storage icons with Mabinogi's item-color rendering rule."""

from __future__ import annotations

import argparse
from pathlib import Path

from PIL import Image


SEASONAL_ICON_NAMES = [
    *(f"item_seasonalfarming_{index:02d}_{quality}" for index in range(4, 11) for quality in ("s", "h", "p")),
    *(f"item_seasonalfarming_{index:02d}" for index in range(15, 35)),
]


def apply_item_color(image: Image.Image, color: tuple[int, int, int]) -> Image.Image:
    """Apply output = clamp(2 * (source - 128) + color), preserving alpha."""
    image = image.convert("RGBA")
    red, green, blue, alpha = image.split()
    channels = []
    for channel, base in zip((red, green, blue), color):
        channels.append(channel.point(lambda value, base=base: max(0, min(255, 2 * (value - 128) + base))))
    return Image.merge("RGBA", (*channels, alpha))


def crop_icon(image: Image.Image) -> Image.Image:
    if image.width < 48 or image.height < 48:
        raise ValueError(f"Icon source is smaller than 48x48: {image.size}")
    return image.crop((0, 0, 48, 48))


def save_colored_icon(source: Path, destination: Path, color: tuple[int, int, int]) -> None:
    with Image.open(source) as image:
        crop_icon(apply_item_color(image, color)).save(destination, optimize=True)


def alpha_crop(image: Image.Image) -> Image.Image:
    bounds = image.getchannel("A").getbbox()
    if bounds is None:
        raise ValueError("Paint layer has no visible pixels")
    return image.crop(bounds)


def generate_paint_icon(source: Path, destination: Path) -> None:
    with Image.open(source) as image:
        image = image.convert("RGBA")
        if image.size != (256, 64):
            raise ValueError(f"Unexpected paint source size: {image.size}")

        orange_layer = alpha_crop(apply_item_color(image.crop((0, 0, 128, 64)), (255, 156, 0)))
        bucket_layer = alpha_crop(apply_item_color(image.crop((128, 0, 256, 64)), (128, 128, 128)))

    canvas = Image.new("RGBA", (48, 48))
    bucket_x = (canvas.width - bucket_layer.width) // 2
    orange_x = (canvas.width - orange_layer.width) // 2
    canvas.alpha_composite(bucket_layer, (bucket_x, 16))
    canvas.alpha_composite(orange_layer, (orange_x, 1))
    canvas.save(destination, optimize=True)


def generate(data_root: Path, output_root: Path) -> None:
    inventory_root = data_root / "gfx" / "image2" / "inven" / "item"
    legacy_image_root = data_root / "gfx" / "image"
    output_root.mkdir(parents=True, exist_ok=True)

    for name in SEASONAL_ICON_NAMES:
        save_colored_icon(inventory_root / f"{name}.dds", output_root / f"{name}.png", (128, 128, 128))

    save_colored_icon(legacy_image_root / "item_brick.dds", output_root / "item_brick.png", (151, 70, 54))
    save_colored_icon(legacy_image_root / "item_metalplate.dds", output_root / "item_metalplate.png", (189, 189, 189))
    generate_paint_icon(legacy_image_root / "item_paint.dds", output_root / "item_paint.png")
    save_colored_icon(legacy_image_root / "item_ice_piece.dds", output_root / "item_ice_piece.png", (146, 229, 252))


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("data_root", type=Path, help="Path to the unpacked Mabinogi data directory")
    parser.add_argument(
        "--output",
        type=Path,
        default=Path(__file__).resolve().parents[1] / "frontend" / "src" / "assets" / "farm-items",
        help="PNG output directory (defaults to the frontend farm icon directory)",
    )
    return parser.parse_args()


if __name__ == "__main__":
    arguments = parse_args()
    generate(arguments.data_root.resolve(), arguments.output.resolve())
