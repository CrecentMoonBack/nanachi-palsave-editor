#!/usr/bin/env python3
"""Regenerates internal/paldata/data/passives.json.

Maintainer tooling, run once per game update — not part of the build. The
source is the same palworld-save-pal checkout the item and pal tables came
from (see docs/THIRD_PARTY.md), reachable as the SSH alias `pal` or given as a
local path:

    python scripts/build-passives.py                    # over ssh
    python scripts/build-passives.py /path/to/data/json # local checkout

Three upstream files are merged: passive_skills.json for the flags and rank,
l10n/ko and l10n/en for the names.

Why the gear filter is what it is
---------------------------------
Upstream marks who can receive a passive with add_pal / add_rare_pal and four
gear flags (armor, accessory, shot weapon, melee weapon). Filtering on
add_pal alone looks right and is wrong: it keeps 86 of the 105 passives a real
save actually uses and drops 19 that carry no add_* flag at all — Legend,
Nushi, the MutationPal_* line, ElementBoost_*_2_PAL, and WorldTree_CraftSpeed
(악마의 손), which pals genuinely have.

So the rule is inverted: keep everything except what is *only* obtainable on
gear. That drops 147 entries (the BossDefeatReward_* player buffs, AirDash_*,
and the rest of the armor pool) — and zero of them appear in a real save,
which is the check that says the rule is right.
"""
import io
import json
import os
import re
import subprocess
import sys
import tempfile

# Descriptions carry Unreal rich-text runs — "작업 속도 <NumBlue_13>+</>90.0" —
# which are styling for the game's own text renderer and read as garbage
# anywhere else. Stripped here rather than in the UI: it is a property of the
# extracted data, not of how one frontend draws it.
RICH_TEXT = re.compile(r"<[^>]*>")

REMOTE = os.environ.get("REMOTE", "pal")
REMOTE_DIR = os.environ.get(
    "REMOTE_DIR", "/root/save-pal-ref/data/json"
)
GEAR_FLAGS = ("add_armor", "add_accessory", "add_shot_weapon", "add_melee_weapon")
FILES = ("passive_skills.json", "l10n/ko/passive_skills.json", "l10n/en/passive_skills.json")

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
DEST = os.path.join(ROOT, "internal", "paldata", "data", "passives.json")


def load(path):
    # Explicit utf-8: Python on a Korean Windows defaults to cp949 and dies on
    # the very first localised name.
    with io.open(path, encoding="utf-8") as f:
        return json.load(f)


def fetch(dest_dir):
    """Copies the three source files out of the remote checkout."""
    print("source: %s:%s" % (REMOTE, REMOTE_DIR))
    for rel in FILES:
        out = os.path.join(dest_dir, rel.replace("/", "_"))
        subprocess.check_call(
            ["scp", "-q", "-o", "BatchMode=yes", "%s:%s/%s" % (REMOTE, REMOTE_DIR, rel), out]
        )
    return dest_dir


def source_paths(d, fetched):
    if fetched:
        return [os.path.join(d, r.replace("/", "_")) for r in FILES]
    return [os.path.join(d, r) for r in FILES]


def main():
    if len(sys.argv) > 1:
        src, fetched = sys.argv[1], False
        tmp = None
    else:
        tmp = tempfile.mkdtemp()
        src, fetched = fetch(tmp), True

    base_p, ko_p, en_p = source_paths(src, fetched)
    base, ko, en = load(base_p), load(ko_p), load(en_p)

    out = {}
    dropped = 0
    for pid, v in base.items():
        pal = v.get("add_pal") or v.get("add_rare_pal")
        gear = any(v.get(g) for g in GEAR_FLAGS)
        if gear and not pal:
            dropped += 1
            continue
        entry = {"rank": v.get("rank", 0)}
        name_ko = (ko.get(pid) or {}).get("localized_name")
        name_en = (en.get(pid) or {}).get("localized_name")
        desc_ko = (ko.get(pid) or {}).get("description")
        if name_ko:
            entry["name_ko"] = name_ko
        if name_en:
            entry["name_en"] = name_en
        if desc_ko:
            desc_ko = " ".join(RICH_TEXT.sub("", desc_ko).split())
            if desc_ko:
                entry["desc_ko"] = desc_ko
        out[pid] = entry

    with io.open(DEST, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
        f.write("\n")

    missing_ko = [k for k, v in out.items() if "name_ko" not in v]
    print("kept %d, dropped %d gear-only" % (len(out), dropped))
    print("without a Korean name: %d" % len(missing_ko))
    print("wrote %s (%d bytes)" % (DEST, os.path.getsize(DEST)))
    if tmp:
        import shutil

        shutil.rmtree(tmp, ignore_errors=True)


if __name__ == "__main__":
    main()
