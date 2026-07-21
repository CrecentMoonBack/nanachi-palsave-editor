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

## Reference data (`internal/paldata/data/`)

The item and pal tables embedded by `internal/paldata` are derived from the
JSON under `data/json/` in <https://github.com/oMaN-Rod/palworld-save-pal>
(rev `c1668830ad984caf46256c3619f8564daab9de8d`, 2026-07-15): `items.json`,
`pals.json`, `elements.json` and the `l10n/ko` + `l10n/en` counterparts.

They are not vendored verbatim. Fields the editor does not use were dropped and
the localisation files merged in, so what ships is three files totalling ~1.2 MB:

| file | size | entries |
| --- | --- | --- |
| `data/items.json` | 926 KB | 2372 |
| `data/pals.json` | 253 KB | 809 |
| `data/elements.json` | 849 B | 9 |

### Provenance and licensing — the same unresolved question as ooz

This is **extracted game data**: ids, stats, icon filenames and localised
strings lifted out of Palworld's own data tables. palworld-save-pal is MIT, but
that covers the code it wrote, not the game data it redistributes. Pocketpair
owns the underlying content and has granted nobody a licence to it. So the
honest position is the same one this file already takes on ooz: we are relying
on an upstream project's implicit claim, we cannot point at a licence grant,
and if that ever gets clarified differently this file is the first thing to
revisit.

What makes it defensible in practice is that the strings are facts about the
game — a pal's id, its Korean name, the filename of its icon — and the
alternative is worse: a runtime scraper against a fan site, which `CLAUDE.md`
rules out for good reason.

**No artwork is embedded, ever.** `internal/paldata` maps ids to icon
*filenames* only; the `.webp` files themselves live in `assets/icons/` beside
the executable and the GUI falls back to text names when they are absent.

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
