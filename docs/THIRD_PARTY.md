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

## Artwork (`assets/icons/`) — fetched, never shipped

The pal and item images are Palworld's own UI textures, extracted from the
game. Pocketpair owns them and has licensed them to nobody. Unlike the id
tables above there is no "these are facts about the game" argument to make: an
icon is the artwork itself, so the repository ships none of it, ever.

What ships is the machinery:

- `internal/paldata` maps an id to an icon *filename* — see `PalIcon`,
  `PalMenuIcon`, `ItemIcon`.
- `internal/icons` locates `assets/icons/` (beside the executable first, then
  the working tree, the same search order `internal/oodle` uses for its DLL)
  and serves it to the frontend over `/icons/`.
- `scripts/fetch-icons.sh` puts the files there, on the user's machine.
- `.gitignore` has `assets/icons/`, so a populated folder cannot be committed
  by accident.

Every one of those degrades cleanly: with no artwork, `icons.Available()` is
false, requests 404, and the GUI shows Korean names. **The editor is fully
usable with no images at all** — this is a display nicety, not a dependency.

The script fetches from palworld-save-pal
(<https://github.com/oMaN-Rod/palworld-save-pal>, `ui/src/lib/assets/img/`),
which is the same upstream the id tables came from. It clones that public
repository — a sparse, blob-filtered, depth-1 clone of the one directory, so it
transfers the 34 MB rather than the repo's ~380 MB — the same way this project
clones ooz instead of vendoring it. Not a scraper against a fan site;
`CLAUDE.md` rules that out for two good reasons: it puts our load on someone who
never agreed to it, and it breaks the moment they change their markup.

`REMOTE=host` switches it to an existing checkout over ssh instead, which is
faster when you already have one. **That used to be the only mode**, pointing at
an SSH alias that exists on one machine, which made the copy shipped in a
release useless to everyone who downloaded it. Found on the first release, by
a user asking the obvious question: how does anyone else get the images.

That upstream repository carries no licence of its own. It is the same source
the id tables came from and the same one this file already documented, so
pointing the script at it changes nothing about who owns the artwork — but it
is worth knowing before recommending the tool to anyone.

About the set: ~2462 files, ~34 MB, all lowercase `.webp`, flat. Two outliers
are not icons at all — `t_worldmap.webp` (2.5 MB) and `t_treemap.webp` (3.4 MB)
are full-screen textures that happen to live in the same folder, and together
are ~17% of the download. The script skips upstream's `img/app/` subfolder,
which is that project's own branding rather than game content.

## 나나치 마스코트 (`frontend/src/assets/nanachi_*.png`) — 저장소에 있음

앱 아이콘과 UI 마스코트로 쓰는 나나치 그림 5장이다. 나나치는 『메이드 인
어비스』(츠쿠시 아키히토 / 타케쇼보) 캐릭터다. 같은 작성자의
`NanachiDeprotector` 에서 가져왔고, 그쪽에서도 저장소에 함께 들어 있다.

**바로 위 팰월드 아트 정책과 다른 판단이라, 그 차이를 여기 적어둔다.**
팰월드 텍스처는 이 도구가 다루는 게임의 에셋이고 2,400장이 넘어서 "받아서 쓰고
저장소엔 안 넣는다"가 명확한 선이었다. 나나치 그림은 앱 자체의 브랜딩이고
5장이다. 그래도 **남의 저작물인 건 똑같다** — "게임 에셋이 아니니 괜찮다"가
아니라, 작성자가 자기 다른 프로젝트에서 이미 같은 선택을 했으니 여기서도
맞춘 것이다.

문제가 되면 팰월드 아트와 같은 처리를 하면 된다: `frontend/src/assets/` 를
`.gitignore` 에 넣고 스크립트로 받아오게 바꾸는 것. 다만 마스코트는 아이콘과
달리 **없으면 빌드가 깨진다** — `import` 로 번들에 들어가서, 아이콘처럼 없으면
텍스트로 폴백하는 구조가 아니다. 그렇게 바꾸려면 로딩 방식도 같이 손봐야 한다.

`scripts/build-appicon.py` 가 `nanachi_face.png` 하나로 `build/appicon.png`
(1024px)와 `build/windows/icon.ico`(16~256px 7종)를 만든다. 초상화를 바꾸면
스크립트를 다시 돌린다.

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
