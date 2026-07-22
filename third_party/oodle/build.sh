#!/usr/bin/env bash
# Builds nanachi_ooz.dll from the zao/ooz sources plus shim.cpp.
#
# Prerequisites:
#   git submodule update --init --depth 1   (inside ooz-zao, for simde)
#   MinGW-w64 g++ on PATH
#
# Notes on the flags:
#   -fpermissive  ooz declares a static _rotl that clashes with MinGW's extern
#                 declaration. Upstream targets MSVC and Linux gcc, not MinGW.
#   -w            ooz is noisy (notably __forceinline redefinition); the noise
#                 is not actionable for vendored third-party code.
#   -Isimde       stdafx.h includes <simde/x86/sse2.h>; simde's headers live one
#                 level down inside the submodule.
set -euo pipefail

cd "$(dirname "$0")"
SRC=ooz-zao
OUT=nanachi_ooz.dll
# Objects go in a relative directory beside this script, not mktemp -d. A
# non-ASCII TMPDIR (C:/Users/<한글>/AppData/Local/Temp on a Korean Windows)
# reaches the assembler mangled and it fails with "can't create ...kraken.o".
# Every path handed to g++ here is relative to this directory, so none of them
# carry the username at all.
OBJ=obj
rm -rf "$OBJ" && mkdir -p "$OBJ"
trap 'rm -rf "$OBJ"' EXIT

if [ ! -f "$SRC/simde/simde/x86/sse2.h" ]; then
    echo "simde submodule missing. Run:" >&2
    echo "  (cd $SRC && git submodule update --init --depth 1)" >&2
    exit 1
fi

FILES=(
    kraken.cpp            # decompressor
    lzna.cpp              # kraken.cpp calls LZNA_DecodeQuantum / LZNA_InitLookup
    bitknit.cpp           # kraken.cpp calls Bitknit_Decode / BitknitState_Init
    compress.cpp          # CompressBlock entry point
    compr_kraken.cpp
    compr_entropy.cpp
    compr_match_finder.cpp
    compr_multiarray.cpp
    compr_tans.cpp
    compr_mermaid.cpp
    compr_leviathan.cpp
    stdafx.cpp
)

echo "compiling ooz..."
for f in "${FILES[@]}"; do
    printf '  %-26s' "$f"
    g++ -std=c++17 -O2 -w -fpermissive -I"$SRC" -I"$SRC/simde" \
        -c "$SRC/$f" -o "$OBJ/${f%.cpp}.o"
    echo ok
done

printf '  %-26s' shim.cpp
g++ -std=c++17 -O2 -w -fpermissive -I"$SRC" -I"$SRC/simde" \
    -c shim.cpp -o "$OBJ/shim.o"
echo ok

# --allow-multiple-definition: -fpermissive promotes stdafx.h's static _rotl to
# external linkage, so every translation unit emits one. The definitions are
# identical, so letting the linker keep the first is safe.
#
# -static, not just -static-libgcc/-static-libstdc++. Those two leave
# libwinpthread-1.dll as a runtime dependency, which is fine on a machine with
# MinGW installed and a trap everywhere else: Windows searches PATH for it, and
# if it finds a 32-bit copy left behind by some unrelated tool it loads that and
# fails with "%1 is not a valid Win32 application" — an architecture error
# pointing at our DLL, which is the wrong place to look. A user hit exactly
# this. Linking the runtime in means the DLL asks the system for nothing but
# kernel32 and the UCRT.
echo "linking $OUT..."
g++ -shared -o "$OUT" "$OBJ"/*.o \
    -static -static-libgcc -static-libstdc++ \
    -Wl,--allow-multiple-definition -Wl,--strip-all

ls -la "$OUT"
echo "done"

# The DLL must not need anything the user might not have. A dependency the
# build machine happens to own is invisible here and fatal on someone else's
# PC: v0.1.3 shipped needing libwinpthread-1.dll, and a user whose PATH held a
# 32-bit copy of it got "%1 is not a valid Win32 application" — an
# architecture error naming *our* DLL, which is the wrong place to look.
#
# So the imports are checked rather than assumed. kernel32 and the C runtime
# ship with Windows; anything else has to be linked in or deliberately added
# to this list.
ALLOWED='^(kernel32|msvcrt|advapi32|user32|api-ms-win-crt-[a-z0-9-]+)\.dll$'
if command -v objdump >/dev/null 2>&1; then
    bad=$(objdump -p "$OUT" | sed -n 's/^\tDLL Name: //p' \
        | tr 'A-Z' 'a-z' | grep -Ev "$ALLOWED" || true)
    if [ -n "$bad" ]; then
        echo >&2
        echo "ERROR: $OUT depends on DLLs a user may not have:" >&2
        echo "$bad" | sed 's/^/  /' >&2
        echo >&2
        echo "Link them in (see -static above) rather than shipping them." >&2
        exit 1
    fi
    echo "dependencies ok: $(objdump -p "$OUT" | sed -n 's/^\tDLL Name: //p' | tr '\n' ' ')"
else
    echo "objdump not found; skipping the dependency check." >&2
fi
