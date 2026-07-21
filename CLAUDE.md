# 나나치의 팰월드 세이브 에디터 (NanachiPalSaveEditor)

Palworld dedicated-server save editor. Go + Wails v2 + React/TS.

Sibling to `NanachiDeprotector2` — same stack, same DLL-binding approach,
same Nanachi branding. Reuse its conventions rather than inventing new ones.

## Layout

```
internal/oodle/     Oodle compress/decompress. Binds a native DLL via
                    windows.NewLazyDLL + syscall — NO cgo, same as
                    NanachiDeprotector2's internal/mpq ↔ StormLib.
internal/gvas/      Generic UE GVAS property tree: decode + encode.
internal/palsave/   Palworld-specific RawData codecs + typed accessors.
internal/paldata/   Embedded item/pal ID tables + Korean l10n.
cmd/palsave/        CLI. Built and validated BEFORE any GUI work.
```

## Save format facts (verified against a live server save, 2026-07-21)

- Magic `PlM1` = Oodle-compressed. `save_type` = **49**.
- cheahjs `palworld-save-tools` 0.24.0 **cannot** read PlM1.
  Working Python reference: deafdudecomputers/PalworldSaveTools (`palsav` + `palooz`).
- Reference implementations:
  - **oMaN-Rod/palworld-save-pal** — Rust (`psp-core`) + Svelte + Tauri. Most complete;
    use as the correctness/coverage reference.
  - **palsav** (Python) — dynamic dict shape, closer to how a decoder is structured.
- Live save scale: ~2.7 MB compressed → **~46 MB** decompressed GVAS,
  1670 characters, ~11k item containers.
- Palbox pals live in `Level.sav` → `CharacterSaveParameterMap`,
  **not** in `Players/*.sav`. Player saves only hold container *references*.
- Player inventory containers come from `Players/<UID>.sav` →
  `SaveData.InventoryInfo.{Common,DropSlot,Essential,Weapon,Armor,Food}ContainerId`,
  which index into `Level.sav` → `ItemContainerSaveData`.

## Traps (each of these cost real debugging time)

**`Level` is a ByteProperty with a nested value.** `Exp` is a flat Int64Property.
Writing the wrong shape produces `TypeError: 'int' object is not subscriptable`
at *encode* time, long after the mutation looked fine:

```
Level: {"id":null, "value":{"type":"None","value":80}, "type":"ByteProperty"}
Exp:   {"id":null, "value":45859908,                   "type":"Int64Property"}
```

This is exactly why `internal/gvas` uses **typed properties, not `map[string]any`**,
and why `internal/palsave` must expose `SetLevel(n)` rather than letting callers
index the tree by hand. Make the wrong shape unrepresentable.

**Absent property ≠ zero.** A pal with no `Level` property is level 1; the property
must be *created* (54 of 99 pals in the observed save were like this), not just assigned.

**Item container slots are sparse.** `SlotNum` is capacity; only occupied slots are
materialized in `Slots`. Adding an item means appending a slot with an unused
`slot_index`, not writing to index N.

**Re-compression is not byte-identical.** ooz's Oodle encoder differs from the game's:
a re-encoded save came out ~22% smaller with zero data loss. So:

- GVAS layer round-trip **must** be byte-identical → assert it.
- Compression layer can only be tested as `Decompress(Compress(x)) == x`.
- Never use output file size as an integrity signal. Compare entity counts
  (characters / item containers / character containers / guilds).

**Python 3.12 segfaults on teardown** when freeing the ~46 MB GVAS object graph —
deterministic, same instruction pointer, not OOM. The Python reference scripts need
`os._exit(0)` plus explicit flush, or output is lost with the crash. Go has no
equivalent problem; this is a genuine reason the port is worth doing.

## Rules

- **Backup before every write.** Non-negotiable, in CLI and GUI both.
- Real `.sav` fixtures are gitignored — they contain other players' Steam IDs.
- `oo2core*.dll` is Epic-owned: never redistribute. Locate it in the user's game
  install, or bundle ooz only if its license allows.
- Verify integrity by entity counts, never by file size.
