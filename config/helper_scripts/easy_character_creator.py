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


def read_names(path: Path):
    with path.open("r", encoding="utf-8") as f:
        return [line.strip() for line in f if line.strip()]


def read_characters(path: Path):
    if not path.exists():
        return []
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def write_characters(path: Path, characters):
    path.write_text(json.dumps(characters, indent=4) + "\n", encoding="utf-8")


def main():
    names_path = find_config_file("names.txt")
    chars_path = find_config_file("characters.json")

    names = read_names(names_path)
    characters = read_characters(chars_path)

    existing = { (c.get("name") or "").strip() for c in characters }

    added = 0
    for n in names:
        if n in existing:
            continue
        characters.append({
            "id": "",
            "name": n,
            "image_path": "",
            "type": "",
            "rarity": "",
            "toughness": 0,
            "power": 0,
        })
        existing.add(n)
        added += 1

    if added:
        write_characters(chars_path, characters)
        print(f"added {added} names to {chars_path}")
    else:
        print("no new names to add")


if __name__ == "__main__":
    main()
