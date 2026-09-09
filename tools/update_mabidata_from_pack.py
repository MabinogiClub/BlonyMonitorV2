"""Regenerate the runtime data tables from an unpacked Mabinogi data tree.

The game files are an input format only. The application continues to consume
the small, stable schema in mabidata.db.
"""

from __future__ import annotations

import argparse
import base64
from collections import defaultdict
import io
import re
import shutil
import sqlite3
import tempfile
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Callable


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


def item_features(path: Path, locale: str) -> dict[str, set[str]]:
    """Return the feature names assigned to each item ID for one region."""
    features: dict[str, set[str]] = defaultdict(set)
    if not path.exists():
        return features
    region = parse_xml(path).find(locale.lower())
    if region is None:
        return features
    for node in region.iter("itemInfo"):
        item_id = node.attrib.get("ItemID")
        feature = node.attrib.get("feature")
        if item_id and feature:
            features[item_id].add(feature)
    return features


def item_locale_score(value: str, target: str) -> int | None:
    """Rank an item Locale for target, or return None when it is excluded."""
    raw = (value or "").strip().lower()
    target = target.lower()
    if not raw:
        return 0

    tokens = [token.strip() for token in re.split(r"[;,|]", raw) if token.strip()]
    positives = {token for token in tokens if not token.startswith("!")}
    negatives = {token[1:] for token in tokens if token.startswith("!")}
    if target in negatives:
        return None
    if positives:
        return 2 if target in positives else None
    return 1


def pick_item_row(
    candidates: list[dict[str, str]],
    enabled_features: set[str] | None,
    target_locale: str,
    has_localized_name: Callable[[dict[str, str]], bool] | None = None,
) -> dict[str, str]:
    """Pick the item definition used by a regional client.

    Feature-specific definitions named by FeaturesCheckList win first. Without
    a matching feature, preserve the client's later-definition-wins behavior.
    Locale ranks candidates so a China-specific row cannot be overwritten by a
    later Korea/USA definition.
    """
    applicable = [
        (score, index, row)
        for index, row in enumerate(candidates)
        if (score := item_locale_score(row.get("Locale", ""), target_locale)) is not None
    ]
    if not applicable:
        applicable = [(0, index, row) for index, row in enumerate(candidates)]

    def best(entries: list[tuple[int, int, dict[str, str]]]) -> dict[str, str]:
        locale_score = max(entry[0] for entry in entries)
        same_locale = [entry for entry in entries if entry[0] == locale_score]
        if has_localized_name is not None:
            localized_entries = [entry for entry in same_locale if has_localized_name(entry[2])]
            if localized_entries:
                same_locale = localized_entries
        return max(same_locale, key=lambda entry: entry[1])[2]

    if enabled_features:
        matching = [entry for entry in applicable if entry[2].get("feature") in enabled_features]
        if matching:
            return best(matching)

    return best(applicable)


def unique_item_rows(
    paths: list[Path],
    features: dict[str, set[str]],
    target_locale: str,
    has_localized_name: Callable[[dict[str, str]], bool] | None = None,
) -> dict[str, dict[str, str]]:
    candidates: dict[str, list[dict[str, str]]] = defaultdict(list)
    for path in paths:
        if not path.exists():
            continue
        for node in parse_xml(path).iter("Mabi_Item"):
            item_id = node.attrib.get("ID")
            if item_id:
                candidates[item_id].append(dict(node.attrib))
    return {
        item_id: pick_item_row(rows, features.get(item_id), target_locale, has_localized_name)
        for item_id, rows in candidates.items()
    }


def resolve_pack_file(pack_data: Path, relative: str) -> Path:
    """Resolve a packed asset path against the unpacked data/ directory.

    XML attributes are usually rooted at data/, for example
    data/gfx/image/gui_icon_skill_006.dds. pack_data already points at that
    data/ directory, so the leading data/ segment must be stripped.
    """
    rel = relative.replace("\\", "/").lstrip("/")
    if rel.lower().startswith("data/"):
        rel = rel[5:]
    return pack_data.joinpath(*rel.split("/")) if rel else pack_data


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
        sheet_path = resolve_pack_file(pack_data, image_file)
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
        option_local = merged_localized("optionset")

        target_locale = "china"
        item_paths = sorted(dbdir.glob("*ItemDB*.xml"), key=lambda p: p.name.lower())
        features = item_features(dbdir / "FeaturesCheckList.xml", target_locale)
        items = unique_item_rows(
            item_paths,
            features,
            target_locale,
            lambda row: bool(localized(row.get("Text_Name1", ""), item_local).strip()),
        )
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

        option_rows = []
        option_path = dbdir / "OptionSet.xml"
        if option_path.exists():
            for node in parse_xml(option_path).iter("OptionSet"):
                try:
                    option_id = int(node.attrib.get("ID", "0"))
                except ValueError:
                    continue
                if option_id <= 0:
                    continue
                # LocalName2 is the suffix display form when present; it is
                # normally identical to LocalName but matches the client UI.
                option_name = localized(node.attrib.get("LocalName2", ""), option_local)
                if not option_name:
                    option_name = localized(node.attrib.get("LocalName", ""), option_local)
                if option_name:
                    option_rows.append((option_id, option_name))

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
        replace_table(old, "OptionSet", """CREATE TABLE OptionSet (
            ID INTEGER NOT NULL, LocalName TEXT NOT NULL
        )""", option_rows, 2)
        old.execute("CREATE INDEX idx_optionset_id ON OptionSet(ID)")
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
            "OptionSet": len(option_rows),
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
