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


def assign_ids(characters_path: Path) -> int:
    data = characters_path.read_text(encoding="utf-8")
    characters = json.loads(data)

    updated = 0
    previous_id = 0
    have_previous = False

    for i, ch in enumerate(characters):
        current_id = (ch.get("id") or "").strip()
        if not current_id:
            if not have_previous:
                previous_id = 0
                have_previous = True
            previous_id += 1
            characters[i]["id"] = f"{previous_id:08x}"
            updated += 1
            continue

        try:
            parsed = int(current_id, 16)
        except ValueError as e:
            raise ValueError(f"parse character id {current_id!r}: {e}")
        previous_id = parsed
        have_previous = True

    if updated:
        characters_path.write_text(json.dumps(characters, indent=4) + "\n", encoding="utf-8")

    return updated


def main():
    characters_path = find_config_file("characters.json")
    updated = assign_ids(characters_path)
    if updated == 0:
        print("no character ids needed to be assigned")
    else:
        print(f"assigned {updated} character ids")


if __name__ == "__main__":
    main()
