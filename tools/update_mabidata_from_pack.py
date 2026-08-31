"""Regenerate the runtime data tables from an unpacked Mabinogi data tree.

The game files are an input format only. The application continues to consume
the small, stable schema in mabidata.db.
"""

from __future__ import annotations

import argparse
import base64
import io
import re
import shutil
import sqlite3
import tempfile
import xml.etree.ElementTree as ET
from pathlib import Path


LT_RE = re.compile(r"^_LT\[.+\.(\d+)\]$")


def read_localized(path: Path) -> dict[int, str]:
    values: dict[int, str] = {}
    if not path.exists():
        return values
    raw = path.read_bytes()
    if raw.startswith((b"\xff\xfe", b"\xfe\xff")):
        content = raw.decode("utf-16", errors="replace")
    else:
        content = raw.decode("utf-8-sig", errors="replace")
    for line in content.splitlines():
            key, sep, value = line.rstrip("\r\n").partition("\t")
            if not sep:
                continue
            try:
                values[int(key)] = value
            except ValueError:
                continue
    return values


def localized(value: str, values: dict[str, dict[int, str]]) -> str:
    match = LT_RE.match(value or "")
    if match:
        prefix, number = value[4:-1].rsplit(".", 1)
        prefix = prefix.removeprefix("xml.")
        return values.get(prefix.lower(), {}).get(int(number), "")
    return value or ""


def parse_xml(path: Path) -> ET.Element:
    return ET.parse(path).getroot()


def unique_rows(paths: list[Path], tag: str, key: str) -> dict[str, dict[str, str]]:
    rows: dict[str, dict[str, str]] = {}
    for path in paths:
        if not path.exists():
            continue
        for node in parse_xml(path).iter(tag):
            value = node.attrib.get(key)
            if value:
                # Later files are more specific (for example itemDB_Weapon).
                rows[value] = dict(node.attrib)
    return rows


def skill_icons(pack_data: Path, old: sqlite3.Connection, rows: dict[str, dict[str, str]]) -> dict[int, str]:
    """Crop current icons from sprite sheets, falling back to the old DB."""
    icons: dict[int, str] = {}
    for skill_id, image_data in old.execute("SELECT SkillID, ImageData FROM SkillInfo"):
        if image_data:
            icons[int(skill_id)] = image_data

    try:
        from PIL import Image
    except ImportError:
        return icons

    sheets: dict[str, object] = {}
    for raw_id, attrs in rows.items():
        sid = int(raw_id)
        image_file = attrs.get("ImageFile", "")
        if not image_file:
            continue
        sheet_path = pack_data / image_file.replace("/", "\\")
        if not sheet_path.exists():
            continue
        try:
            if image_file not in sheets:
                sheets[image_file] = Image.open(sheet_path).convert("RGBA")
            sheet = sheets[image_file]
            x = int(attrs.get("PositionX", "0")) * 42
            y = int(attrs.get("PositionY", "0")) * 42
            crop = sheet.crop((x, y, x + 42, y + 42))
            output = io.BytesIO()
            crop.save(output, format="PNG")
            icons[sid] = base64.b64encode(output.getvalue()).decode("ascii")
        except (OSError, ValueError):
            # A few legacy/hidden skills do not have a usable sheet entry.
            # Their previous icon is still preferable to an empty value.
            continue
    return icons


def replace_table(conn: sqlite3.Connection, name: str, schema: str, rows: list[tuple], columns: int) -> None:
    conn.execute(f'DROP TABLE IF EXISTS "{name}"')
    conn.execute(schema)
    placeholders = ",".join("?" for _ in range(columns))
    conn.executemany(f'INSERT INTO "{name}" VALUES ({placeholders})', rows)


def build(source: Path, target: Path) -> dict[str, int]:
    data = source / "data"
    dbdir = data / "db"
    localdir = data / "local" / "xml"
    if not dbdir.exists():
        raise SystemExit(f"missing data/db: {dbdir}")

    target.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="mabidata-update-") as temp:
        temp_db = Path(temp) / target.name
        shutil.copy2(target, temp_db)
        old = sqlite3.connect(temp_db)

        def merged_localized(prefix: str) -> dict[str, dict[int, str]]:
            result: dict[str, dict[int, str]] = {}
            for path in sorted(localdir.glob("*.china.txt"), key=lambda p: p.name.lower()):
                if path.stem.lower().startswith(prefix.lower()):
                    result[path.stem.split(".", 1)[0].lower()] = read_localized(path)
            return result

        item_local = merged_localized("itemdb")
        skill_local = merged_localized("skillinfo")
        condition_local = merged_localized("charactercondition")
        race_local = merged_localized("race")
        dungeon_local = merged_localized("dungeondb")
        minimap_local = merged_localized("minimapinfo")

        item_paths = sorted(dbdir.glob("*ItemDB*.xml"), key=lambda p: p.name.lower())
        items = unique_rows(item_paths, "Mabi_Item", "ID")
        item_rows = []
        for raw_id, attrs in items.items():
            try:
                item_rows.append((
                    int(raw_id),
                    attrs.get("Text_Name0", ""),
                    localized(attrs.get("Text_Name1", ""), item_local),
                    localized(attrs.get("Text_Desc1", ""), item_local),
                    attrs.get("File_InvImage", ""),
                    int(attrs.get("Inv_XSize", "0") or 0),
                    int(attrs.get("Inv_YSize", "0") or 0),
                ))
            except ValueError:
                continue

        skill_nodes = unique_rows([dbdir / "Skill" / "SkillInfo.xml"], "Skill", "SkillID")
        old_skills = old
        icons = skill_icons(data, old_skills, skill_nodes)
        skill_rows = []
        for raw_id, attrs in skill_nodes.items():
            sid = int(raw_id)
            skill_rows.append((
                sid,
                attrs.get("SkillEngName", ""),
                localized(attrs.get("SkillLocalName", ""), skill_local),
                localized(attrs.get("SkillDesc", ""), skill_local),
                icons.get(sid, ""),
            ))

        race_nodes = unique_rows([dbdir / "Race.xml"], "Race", "ID")
        race_rows = [(int(raw_id), a.get("ClassName", ""), a.get("StringID", ""),
                      a.get("EnglishName", ""), localized(a.get("LocalName", ""), race_local))
                     for raw_id, a in race_nodes.items()]

        condition_nodes = unique_rows([dbdir / "CharacterCondition.xml"], "CharacterCondition", "ConditionID")
        condition_rows = [(int(raw_id), a.get("ConditionEngName", ""),
                           localized(a.get("ConditionLocalName", ""), condition_local))
                          for raw_id, a in condition_nodes.items()]

        dungeon_nodes = unique_rows([dbdir / "DungeonDB.xml"], "dungeon", "name")
        dungeon_rows = [(name, localized(a.get("localname", ""), dungeon_local))
                        for name, a in dungeon_nodes.items()]

        minimap_root = parse_xml(dbdir / "MinimapInfo.xml")
        minimap_rows: dict[str, list[tuple]] = {
            "MinimapInfo_FieldMapInfoList": [],
            "MinimapInfo_Dungeon_Lobby": [],
            "MinimapInfo_DynamicRegion": [],
        }
        for group, table in [("FieldMapInfoList", "MinimapInfo_FieldMapInfoList"),
                             ("Dungeon_Lobby", "MinimapInfo_Dungeon_Lobby"),
                             ("DynamicRegion", "MinimapInfo_DynamicRegion")]:
            parent = minimap_root.find(group)
            if parent is None:
                continue
            # The client keeps several feature-specific records per region,
            # while the application's compatibility schema has one row per
            # RegionID. Prefer the unqualified/default map when available.
            selected: dict[int, tuple[int, str, str, str, str]] = {}
            for a in parent.iter("MinimapInfo"):
                try:
                    region_id = int(a.get("RegionID", "0"))
                    row = (
                        region_id,
                        localized(a.get("MapName", ""), minimap_local),
                        localized(a.get("MapLocalName", ""), minimap_local),
                        localized(a.get("MapFile", ""), minimap_local),
                    )
                    feature = a.get("feature", "")
                    previous = selected.get(region_id)
                    if previous is None or (previous[4] and not feature):
                        selected[region_id] = (*row, feature)
                except ValueError:
                    continue
            minimap_rows[table].extend(row[:4] for row in selected.values())

        replace_table(old, "ItemDB", """CREATE TABLE ItemDB (
            ID INTEGER PRIMARY KEY, Text_Name0 TEXT, Text_Name1 TEXT,
            Text_Desc1 TEXT, File_InvImage TEXT, Inv_XSize INTEGER, Inv_YSize INTEGER
        )""", item_rows, 7)
        replace_table(old, "SkillInfo", """CREATE TABLE SkillInfo (
            SkillID INTEGER PRIMARY KEY, SkillEngName TEXT, SkillLocalName TEXT,
            SkillDesc TEXT, ImageData TEXT
        )""", skill_rows, 5)
        replace_table(old, "Race", """CREATE TABLE Race (
            ID INTEGER PRIMARY KEY, ClassName TEXT, StringID TEXT,
            EnglishName TEXT, LocalName TEXT
        )""", race_rows, 5)
        replace_table(old, "CharacterCondition", """CREATE TABLE CharacterCondition (
            ConditionID INTEGER PRIMARY KEY, ConditionEngName TEXT, ConditionLocalName TEXT
        )""", condition_rows, 3)
        replace_table(old, "DungeonDB", """CREATE TABLE DungeonDB (
            name TEXT PRIMARY KEY, localname TEXT
        )""", dungeon_rows, 2)
        for table, rows in minimap_rows.items():
            replace_table(old, table, f'''CREATE TABLE "{table}" (
                RegionID INTEGER PRIMARY KEY, MapName TEXT, MapLocalName TEXT, MapFile TEXT
            )''', rows, 4)

        old.commit()
        old.close()
        shutil.copy2(temp_db, target)
        return {
            "ItemDB": len(item_rows),
            "SkillInfo": len(skill_rows),
            "Race": len(race_rows),
            "CharacterCondition": len(condition_rows),
            "DungeonDB": len(dungeon_rows),
            **{key: len(value) for key, value in minimap_rows.items()},
        }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=Path, help="unpacked game root, containing data/")
    parser.add_argument("target", type=Path, help="runtime mabidata.db to replace")
    args = parser.parse_args()
    print(build(args.source, args.target))


if __name__ == "__main__":
    main()
