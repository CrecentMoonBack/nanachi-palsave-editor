#!/usr/bin/env bash
# Populates assets/icons/ with pal and item artwork, on this machine only.
#
# Why this is a script and not a folder in the repository:
#   The artwork is Pocketpair's. This project ships no game assets — see the
#   "Distribution stance" section of CLAUDE.md and docs/THIRD_PARTY.md. That
#   applies to release archives too, which is why a downloaded release has no
#   images until this is run.
#
#   The editor works fully without any of this. internal/icons reports the
#   directory as absent and the GUI falls back to Korean text names.
#
# Where the images come from:
#   palworld-save-pal (https://github.com/oMaN-Rod/palworld-save-pal), under
#   ui/src/lib/assets/img/. That project extracted them from the game. We fetch
#   from their public repository the same way docs/THIRD_PARTY.md has you clone
#   ooz rather than vendoring it — not by scraping a fan site, which CLAUDE.md
#   rules out and rightly.
#
#   Only the flat .webp files are taken. The img/app/ subfolder is that
#   project's own branding and has nothing to do with Palworld.
#
# Usage:
#   bash fetch-icons.sh                    # fetch from the public repo
#   REMOTE=host bash fetch-icons.sh        # or from an existing checkout over ssh
#   REMOTE=host REMOTE_DIR=/path/to/img bash fetch-icons.sh
#   FORCE=1 bash fetch-icons.sh            # re-copy even if the count matches
#
# Idempotent: re-running fetches only when something is missing.
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)

# Two layouts have to work, and they put the icons in different places.
#
#   repo    scripts/fetch-icons.sh  -> <repo>/assets/icons
#   release fetch-icons.sh          -> <folder with the exe>/assets/icons
#
# internal/icons looks beside the executable first, so in a release the icons
# must land next to the exe. Deriving the destination as "one level up" is
# right for the repo and wrong for a release — it would put them in the parent
# of the release folder, where nothing looks for them, and the app would still
# show text names with no hint why.
if [ -f "$SCRIPT_DIR/../go.mod" ]; then
    ROOT=$(cd "$SCRIPT_DIR/.." && pwd)
else
    ROOT=$SCRIPT_DIR
fi
cd "$ROOT"
DEST=assets/icons

UPSTREAM=${UPSTREAM:-https://github.com/oMaN-Rod/palworld-save-pal.git}
UPSTREAM_DIR=ui/src/lib/assets/img
REMOTE=${REMOTE:-}
REMOTE_DIR=${REMOTE_DIR:-/root/save-pal-ref/ui/src/lib/assets/img}

before=0
if [ -d "$DEST" ]; then
    before=$(find "$DEST" -maxdepth 1 -type f -name '*.webp' | wc -l | tr -d ' ')
fi

echo "destination: $ROOT/$DEST"
echo "already present: $before icons"

if [ "$before" -gt 0 ] && [ "${FORCE:-0}" != "1" ]; then
    echo
    echo "already populated. Pass FORCE=1 to fetch again."
    exit 0
fi

mkdir -p "$DEST"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if [ -n "$REMOTE" ]; then
    # An existing checkout over ssh. Faster when you already have one, which is
    # the maintainer's case; nobody else has this.
    echo "source: $REMOTE:$REMOTE_DIR"
    if ! ssh -o BatchMode=yes -o ConnectTimeout=15 "$REMOTE" true 2>/dev/null; then
        echo "cannot reach '$REMOTE' over ssh." >&2
        exit 1
    fi
    # tar over ssh rather than scp: the bytes are the same but scp
    # acknowledges each of the ~2,400 files separately, and the round trips
    # cost far more than the data. No -z; .webp is already compressed.
    echo "copying as a single stream..."
    ssh -o BatchMode=yes "$REMOTE" "cd '$REMOTE_DIR' && tar cf - *.webp" \
        | tar xf - -C "$TMP"
else
    # The public repository. A plain clone would pull ~380 MB for the 34 MB we
    # want, so this takes only the one directory: no blobs up front, sparse
    # checkout, single commit.
    echo "source: $UPSTREAM ($UPSTREAM_DIR)"
    command -v git >/dev/null 2>&1 || { echo "git is required." >&2; exit 1; }
    echo "fetching (about 34 MB)..."
    git clone --quiet --depth 1 --filter=blob:none --sparse "$UPSTREAM" "$TMP/src"
    git -C "$TMP/src" sparse-checkout set --no-cone "$UPSTREAM_DIR"
    if [ ! -d "$TMP/src/$UPSTREAM_DIR" ]; then
        echo "upstream layout changed: $UPSTREAM_DIR not found." >&2
        exit 1
    fi
    # Top level only — img/app/ is that project's own branding.
    find "$TMP/src/$UPSTREAM_DIR" -maxdepth 1 -type f -name '*.webp' \
        -exec cp -f {} "$TMP/" \;
fi

count=$(find "$TMP" -maxdepth 1 -type f -name '*.webp' | wc -l | tr -d ' ')
if [ "$count" -eq 0 ]; then
    echo "no icons were fetched." >&2
    exit 1
fi
# Staged through a temp dir so an interrupted fetch cannot leave half-written
# files in assets/icons for the app to serve.
cp -f "$TMP"/*.webp "$DEST/"

after=$(find "$DEST" -maxdepth 1 -type f -name '*.webp' | wc -l | tr -d ' ')
size=$(du -sh "$DEST" | cut -f1)

echo
echo "done. $DEST holds $after icons ($size), $((after - before)) new."
