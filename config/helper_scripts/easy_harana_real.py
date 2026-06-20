#!/usr/bin/env python3
import json
import os
from pathlib import Path


def find_config_file(name: str) -> Path:
    wd = Path.cwd()
    for p in [wd] + list(wd.parents):
        candidate = p / "config" / name
        if candidate.exists():
            return candidate
    raise FileNotFoundError(f"could not find config/{name} from working directory")


def assign_rarity(characters_path: Path) -> int:
    data = characters_path.read_text(encoding="utf-8")
    characters = json.loads(data)

    updated = 0
    previous_id = 0
    have_previous = False

    for i, ch in enumerate(characters):
        current_rarity = (ch.get("rarity") or "").strip()
        current_type = (ch.get("type") or "").strip()
        if not current_rarity and current_type:
            continue

        # set rarity to mythic, type to harana 
        if current_type.lower() == "harana":
            characters[i]["rarity"] = "Mythic"
            updated += 1
            continue
    if updated:
        characters_path.write_text(json.dumps(characters, indent=4) + "\n", encoding="utf-8")

    return updated


def main():
    characters_path = find_config_file("characters.json")
    updated = assign_rarity(characters_path)
    if updated == 0:
        print("no character ids needed to be assigned")
    else:
        print(f"assigned {updated} character ids")


if __name__ == "__main__":
    main()
