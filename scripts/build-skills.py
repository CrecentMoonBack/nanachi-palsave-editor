#!/usr/bin/env python3
"""Regenerates internal/paldata/data/skills.json — active (waza) skills.

Maintainer tooling, run once per game update. Source is the same
palworld-save-pal checkout the other tables came from (docs/THIRD_PARTY.md),
over the SSH alias `pal` or a local path:

    python scripts/build-skills.py
    python scripts/build-skills.py /path/to/data/json

Merges active_skills.json (element, type, power) with l10n/ko and l10n/en for
the names. The key is the full EPalWazaID::Name, which is exactly what the save
stores in a pal's EquipWaza array — so no id munging is needed at read time.

Unique_* skills (a species' signature move) are kept: they appear in real
saves and a user may well want to move one onto another pal.
"""
import io
import json
import os
import re
import subprocess
import sys
import tempfile

RICH_TEXT = re.compile(r"<[^>]*>")
REMOTE = os.environ.get("REMOTE", "pal")
REMOTE_DIR = os.environ.get("REMOTE_DIR", "/root/save-pal-ref/data/json")
FILES = ("active_skills.json", "l10n/ko/active_skills.json", "l10n/en/active_skills.json")

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
DEST = os.path.join(ROOT, "internal", "paldata", "data", "skills.json")


def load(path):
    with io.open(path, encoding="utf-8") as f:
        return json.load(f)


def fetch(src):
    """Return {relative filename: parsed json}, from a local dir or over ssh."""
    if os.path.isdir(src):
        return {f: load(os.path.join(src, f)) for f in FILES}
    out = {}
    tmp = tempfile.mkdtemp()
    for f in FILES:
        local = os.path.join(tmp, os.path.basename(f))
        subprocess.run(["scp", "-q", f"{src}:{REMOTE_DIR}/{f}", local], check=True)
        out[f] = load(local)
    return out


def clean(s):
    return RICH_TEXT.sub("", s or "").strip()


def main():
    src = sys.argv[1] if len(sys.argv) > 1 else REMOTE
    data = fetch(src)
    base = data["active_skills.json"]
    ko = data["l10n/ko/active_skills.json"]
    en = data["l10n/en/active_skills.json"]

    out = {}
    missing_ko = 0
    for key, v in base.items():
        # key is "EPalWazaID::AcidRain"; store the bare name as the json key,
        # since that is friendlier, and keep the full id available.
        name = key.split("::", 1)[1] if "::" in key else key
        krow = ko.get(key, {})
        erow = en.get(key, {})
        name_ko = clean(krow.get("localized_name"))
        if not name_ko:
            missing_ko += 1
        out[name] = {
            "name_ko": name_ko,
            "name_en": clean(erow.get("localized_name")) or name,
            "element": v.get("element", ""),
            "type": v.get("type", ""),      # Shot / Melee / etc.
            "power": v.get("power", 0),
            "unique": name.startswith("Unique_"),
        }

    with io.open(DEST, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, separators=(",", ":"), sort_keys=True)
    print(f"wrote {len(out)} skills to {DEST}")
    print(f"  missing Korean name: {missing_ko}")


if __name__ == "__main__":
    main()
