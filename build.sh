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

# Artwork goes beside the executable, which is the layout internal/icons looks
# in first and the one docs/THIRD_PARTY.md describes. Without this the build
# only finds icons through the working-tree fallback, so it renders correctly
# when launched from build/bin and shows placeholders for everything when
# launched from anywhere else — which reads as "half the images are broken".
#
# Copied rather than linked so the folder can be zipped and shipped as-is;
# -u keeps a rebuild from re-copying 34 MB it already has.
if [ -d assets/icons ]; then
    echo "copying artwork beside the executable..."
    mkdir -p build/bin/assets/icons
    cp -ru assets/icons/. build/bin/assets/icons/
    echo "  $(find build/bin/assets/icons -name '*.webp' | wc -l | tr -d ' ') icons"
else
    echo "no assets/icons to copy; the app will fall back to text names."
    echo "  run scripts/fetch-icons.sh to populate it."
fi

echo
ls -la build/bin/
echo
echo "smoke test:"
(cd build/bin && ./palsave.exe codec)
