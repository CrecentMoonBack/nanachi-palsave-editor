#!/usr/bin/env bash
# Builds the desktop app, the CLI, and drops the native codec beside them.
#
# The DLL copy is the easy step to forget: without it the app builds fine and
# then fails on the first save it opens.
set -euo pipefail
cd "$(dirname "$0")"

DLL=third_party/oodle/nanachi_ooz.dll

if [ ! -f "$DLL" ]; then
    echo "native codec missing; building it first..."
    bash third_party/oodle/build.sh
fi

echo "building desktop app..."
wails build

echo "building cli..."
go build -o build/bin/palsave.exe ./cmd/palsave

cp "$DLL" build/bin/
echo
ls -la build/bin/
echo
echo "smoke test:"
(cd build/bin && ./palsave.exe codec)
