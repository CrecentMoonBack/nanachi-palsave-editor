#!/usr/bin/env bash
# Populates assets/icons/ with pal and item artwork, on this machine only.
#
# Why this is a script and not a folder in the repository:
#   The artwork is Pocketpair's. The repository is public and ships no game
#   assets — see the "Distribution stance" section of CLAUDE.md. Copying the
#   images out of another project's repo would only change who we take them
#   from, not whether we may. So the images stay off the repo and the user
#   fetches them locally, exactly as docs/THIRD_PARTY.md has you clone ooz
#   rather than vendoring it. assets/icons/ is gitignored.
#
#   The editor works fully without any of this. internal/icons reports the
#   directory as absent and the GUI falls back to Korean text names.
#
# Source:
#   A palworld-save-pal working copy on the remote server reachable as the SSH
#   alias `pal`, under ui/src/lib/assets/img/. That project extracted these
#   from the game; we are copying from a checkout we already have, not
#   scraping a fan site — CLAUDE.md rules that out, and rightly.
#
# What is copied:
#   Only the flat .webp icons at the top level. The img/app/ subfolder is
#   palworld-save-pal's own branding (its logo, its contributors' avatars) and
#   has nothing to do with Palworld, so it is skipped.
#
# Usage:
#   bash scripts/fetch-icons.sh            # sync into assets/icons/
#   REMOTE=other-host bash scripts/fetch-icons.sh
#
# Idempotent: re-running syncs only what changed, and prints what it did.
set -euo pipefail

REMOTE=${REMOTE:-pal}
REMOTE_DIR=${REMOTE_DIR:-/root/save-pal-ref/ui/src/lib/assets/img}

cd "$(dirname "$0")/.."
DEST=assets/icons

before=0
if [ -d "$DEST" ]; then
    before=$(find "$DEST" -maxdepth 1 -type f -name '*.webp' | wc -l | tr -d ' ')
fi

echo "source:      $REMOTE:$REMOTE_DIR"
echo "destination: $DEST"
echo "already present: $before icons"

if ! ssh -o BatchMode=yes -o ConnectTimeout=15 "$REMOTE" true 2>/dev/null; then
    echo >&2
    echo "cannot reach '$REMOTE' over ssh." >&2
    echo "Set up the host alias, or point this at another checkout:" >&2
    echo "  REMOTE=user@host REMOTE_DIR=/path/to/img bash scripts/fetch-icons.sh" >&2
    exit 1
fi

mkdir -p "$DEST"

if command -v rsync >/dev/null 2>&1; then
    # --ignore-existing is deliberately NOT used: the upstream checkout can be
    # updated, and a changed icon should win. rsync's size/mtime check already
    # makes a no-op re-run cheap.
    echo "syncing with rsync..."
    rsync -a --info=stats2 --human-readable \
        --include='*.webp' --exclude='*' \
        -e 'ssh -o BatchMode=yes' \
        "$REMOTE:$REMOTE_DIR/" "$DEST/"
else
    # No rsync — the usual case under Git Bash on Windows. scp has no include
    # filter and no incremental mode, so a copy here is all-or-nothing and
    # takes ~10 minutes for the 34 MB.
    #
    # That makes a cheap up-front check worth one extra ssh round trip: ask the
    # remote what it has and skip the transfer entirely when we already hold
    # every file. It is a name-only comparison, not a checksum, so pass
    # FORCE=1 to re-copy after the upstream checkout is updated.
    echo "rsync not found; using scp."
    missing=0
    if [ "${FORCE:-0}" = "1" ]; then
        echo "FORCE=1: re-copying everything."
        missing=1
    else
        echo "checking what is missing..."
        while read -r name; do
            [ -n "$name" ] || continue
            if [ ! -f "$DEST/$name" ]; then
                missing=$((missing + 1))
            fi
        done < <(ssh -o BatchMode=yes "$REMOTE" "cd '$REMOTE_DIR' && ls -1 *.webp")
        echo "$missing missing locally."
    fi

    if [ "$missing" -gt 0 ]; then
        # Staged through a temp dir so an interrupted transfer cannot leave
        # half-written files in assets/icons for the app to serve.
        echo "copying ~34 MB, this takes a few minutes..."
        TMP=$(mktemp -d)
        trap 'rm -rf "$TMP"' EXIT
        scp -q -o BatchMode=yes "$REMOTE:$REMOTE_DIR/*.webp" "$TMP/"
        cp -f "$TMP"/*.webp "$DEST/"
    fi
fi

after=$(find "$DEST" -maxdepth 1 -type f -name '*.webp' | wc -l | tr -d ' ')
size=$(du -sh "$DEST" | cut -f1)

echo
echo "done. $DEST now holds $after icons ($size), $((after - before)) new."
if [ "$after" -eq "$before" ]; then
    echo "nothing changed — already up to date."
fi
