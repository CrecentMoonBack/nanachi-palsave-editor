# Third-party code and licensing

This project is **GPL-3.0** because it links ooz. See `LICENSE`.

## ooz (Kraken/Mermaid/Selkie/Leviathan codec)

- Vendored at `third_party/oodle/ooz-zao/` as a git clone of
  <https://github.com/zao/ooz> (rev `ff5aeb9e45e362e8d6bb1199aa82406285dd2a18`, 2025-10-24).
- `zao/ooz` is a fork of <https://github.com/powzix/ooz> that adds the **encoder**
  (`compress.cpp`, `compr_*.cpp`). Upstream powzix/ooz is decompress-only, so it
  is not sufficient on its own — we need to write saves, not just read them.
- Pulls in `simde` (<https://github.com/simd-everywhere/simde>) as a submodule.

### Licence provenance — read before distributing

Neither `zao/ooz` nor `powzix/ooz` ships a `LICENSE` file. The GPL-3.0 claim
rests on two secondary sources:

1. powzix/ooz's README states it is licensed under GPL, version 3 or later.
2. `palooz` / `palsav-flex` (deafdudecomputers/PalworldSaveTools), which vendors
   the same encoder-bearing sources, declares `License-Expression: GPL-3.0-or-later`
   in its package metadata.

We therefore treat ooz as GPL-3.0-or-later and license this project to match.
If ooz's licensing is ever clarified differently, revisit this file first.

**Never vendor or redistribute `oo2core*.dll`.** That is RAD/Epic's proprietary
Oodle. It is also not an option in practice: UE5 statically links Oodle into the
game executable, so a Palworld install contains no such DLL to borrow.

## Building the native codec

```sh
cd third_party/oodle/ooz-zao && git submodule update --init --depth 1
cd .. && bash build.sh          # produces nanachi_ooz.dll
```

Two MinGW-specific workarounds live in `build.sh`, both documented inline:
`-fpermissive` for ooz's `static _rotl` colliding with MinGW's `extern` one, and
`-Wl,--allow-multiple-definition` because that same flag then gives every
translation unit its own copy.

## Reference implementations (not vendored, consulted only)

- **oMaN-Rod/palworld-save-pal** — Rust + Svelte + Tauri. The most complete
  editor; used as the correctness and coverage reference.
- **palsav / palooz** (deafdudecomputers/PalworldSaveTools) — Python. Source of
  the exact Oodle call sequences reproduced in `third_party/oodle/shim.cpp`.
- **cheahjs/palworld-save-tools** — cannot read PlM1 saves; listed so nobody
  wastes time trying it.
