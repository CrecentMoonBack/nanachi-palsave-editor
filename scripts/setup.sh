#!/usr/bin/env bash
# Gets a fresh clone to a working build.
#
# Roughly 200 MB of what this project needs cannot live in the repository:
# ooz is third-party GPL source we clone rather than vendor, the game artwork
# is Pocketpair's and is never shipped, and the test fixture is a real server
# save containing other players' data. This script fetches or rebuilds each of
# them, and is safe to re-run — every step is skipped when already satisfied.
#
#   bash scripts/setup.sh          everything it can do without the server
#   bash scripts/setup.sh --all    also pull icons and the save fixture over ssh
set -euo pipefail
cd "$(dirname "$0")/.."

WANT_REMOTE=0
[ "${1:-}" = "--all" ] && WANT_REMOTE=1

step() { printf '\n\033[1;33m==> %s\033[0m\n' "$1"; }
skip() { printf '    already present, skipping\n'; }

# --- prerequisites --------------------------------------------------------

step "checking tools"
missing=0
for t in go git g++ node npm; do
    if command -v "$t" >/dev/null 2>&1; then
        printf '    %-6s %s\n' "$t" "$(command -v "$t")"
    else
        printf '    %-6s \033[1;31mMISSING\033[0m\n' "$t"
        missing=1
    fi
done
if ! command -v wails >/dev/null 2>&1; then
    printf '    %-6s \033[1;31mMISSING\033[0m  (go install github.com/wailsapp/wails/v2/cmd/wails@latest)\n' wails
    missing=1
else
    printf '    %-6s %s\n' wails "$(command -v wails)"
fi
if [ "$missing" = 1 ]; then
    echo
    echo "install the missing tools first. g++ comes from MinGW-w64 on Windows."
    exit 1
fi

# --- ooz ------------------------------------------------------------------

step "ooz source (GPL third-party, cloned not vendored)"
if [ -d third_party/oodle/ooz-zao/.git ]; then
    skip
else
    git clone --depth 1 https://github.com/zao/ooz.git third_party/oodle/ooz-zao
fi

if [ -f third_party/oodle/ooz-zao/simde/simde/x86/sse2.h ]; then
    printf '    simde submodule present\n'
else
    ( cd third_party/oodle/ooz-zao && git submodule update --init --depth 1 )
fi

# --- native codec ---------------------------------------------------------

step "Oodle codec DLL"
if [ -f third_party/oodle/nanachi_ooz.dll ]; then
    skip
else
    bash third_party/oodle/build.sh
fi

# --- frontend -------------------------------------------------------------

step "frontend dependencies"
if [ -d frontend/node_modules ]; then
    skip
else
    ( cd frontend && npm install )
fi

# main.go embeds frontend/dist, so Go cannot even compile until the frontend
# has been built once. A fresh clone has no dist directory at all.
step "frontend build (go:embed needs frontend/dist to exist)"
if [ -f frontend/dist/index.html ]; then
    skip
else
    ( cd frontend && npm run build )
fi

# --- optional, needs the server -------------------------------------------

if [ "$WANT_REMOTE" = 1 ]; then
    step "pal artwork (never committed — see docs/THIRD_PARTY.md)"
    bash scripts/fetch-icons.sh

    step "test fixture (a real save; gitignored, holds other players' data)"
    if [ -f testdata/Level.sav ]; then
        skip
    else
        echo "    copying from the server backup..."
        LATEST=$(ssh pal 'ls -d /root/palbackup/*/ | tail -1')
        scp -q "pal:${LATEST}Level.sav" testdata/Level.sav
        echo "    testdata/Level.sav"
        echo "    note: player saves are not copied automatically. To exercise"
        echo "    inventory code you also need one Players/<uid>.sav."
    fi
else
    step "skipped (needs ssh to the server)"
    echo "    artwork and the save fixture. Re-run with --all to include them."
    echo "    The build works without both; icons fall back to Korean names"
    echo "    and only the fixture-backed tests skip."
fi

# --- verify ---------------------------------------------------------------

step "building"
go build ./...
go vet ./...
echo "    go build and vet clean"

step "tests"
go test ./... 2>&1 | grep -v "no test files" || true

step "done"
echo "    bash build.sh          build the desktop app and CLI"
echo "    wails dev              run with hot reload"
echo "    build/bin/palsave.exe  the CLI"
