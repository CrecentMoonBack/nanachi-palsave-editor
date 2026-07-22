import { useCallback, useEffect, useState } from "react";
import "./style.css";
import nanachiFace from "./assets/nanachi_face.png";
import nanachiIdle from "./assets/nanachi_idle.png";
import nanachiWorking from "./assets/nanachi_working.png";
import nanachiSuccess from "./assets/nanachi_success.png";
import nanachiError from "./assets/nanachi_error.png";
import {
  BaseCamps,
  BasePals,
  BaseStorages,
  GiveContainerItem,
  SetContainerItemCount,
  SetSlotCount,
  SetPlayerSlotCount,
  GiveItem,
  Inventory,
  OpenSave,
  PalSpecies,
  Pals,
  PickSaveFile,
  Players,
  SaveToDisk,
  SearchItems,
  SearchPassives,
  SearchPals,
  AddPal,
  PalboxSpace,
  PlayerDetail,
  SetPlayerLevel,
  SetPlayerUnusedPoints,
  SetPlayerStat,
  Relics,
  SetRelic,
  Presets,
  SavePreset,
  DeletePreset,
  SetItemCount,
  SetPalLevel,
  SetPalPassives,
  SetPalRank,
  SetPalRankBonus,
  SetPalTalent,
  SetPalWorkSuitability,
  Status,
} from "../wailsjs/go/main/App";
import { main } from "../wailsjs/go/models";

type Tab = "pals" | "items" | "player";

/**
 * How a storage structure is named in the picker. The save has no Korean name
 * for buildings, so the game's own id is shown rather than a guess at what
 * "CoolerPalFoodBox" is called in Korean.
 */
function boxLabel(b: main.StorageInfo | null): string {
  if (!b) return "";
  return `거점 ${b.camp} · ${b.kind} (${b.items.length})`;
}

/**
 * The save property names behind each editable stat, paired with the label to
 * show. They are the game's own names and the Go side allow-lists them, so a
 * typo here fails loudly on apply rather than writing a junk property.
 */
const TALENTS = [
  { prop: "Talent_HP", label: "체력" },
  { prop: "Talent_Melee", label: "근접" },
  { prop: "Talent_Shot", label: "원거리" },
  { prop: "Talent_Defense", label: "방어" },
] as const;

const SOULS = [
  { prop: "Rank_HP", label: "체력", field: "soulHp" },
  { prop: "Rank_Attack", label: "공격", field: "soulAttack" },
  { prop: "Rank_Defence", label: "방어", field: "soulDefence" },
  { prop: "Rank_CraftSpeed", label: "작업속도", field: "soulCraftSpeed" },
] as const;

/**
 * A dialog that closes on a click outside it, on Escape, or on its close
 * button.
 *
 * "Click outside" has to mean the press *and* the release happened outside.
 * A plain onClick on the backdrop does not: dragging inside a number field and
 * letting go past the edge of the dialog sends mousedown to the input and
 * mouseup to the backdrop, and the browser fires the click on their nearest
 * common ancestor — the backdrop. The dialog then closes while the user is
 * only selecting a value, losing everything they had filled in.
 */
function Modal({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  // Where the press started. Only a press that began on the backdrop counts.
  const [downOnBackdrop, setDownOnBackdrop] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div
      className="modal-backdrop"
      onMouseDown={(e) => setDownOnBackdrop(e.target === e.currentTarget)}
      onMouseUp={(e) => {
        if (downOnBackdrop && e.target === e.currentTarget) onClose();
        setDownOnBackdrop(false);
      }}
    >
      <div className="modal">
        <div className="modal-head">
          <span className="modal-title">{title}</span>
          <button className="ghost" onClick={onClose}>
            닫기
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

/** Icons are optional: the folder may be absent, so every image falls back. */
function Icon({ file, alt }: { file: string; alt: string }) {
  const [failed, setFailed] = useState(false);
  if (!file || failed) {
    return <div className="icon placeholder">{alt.slice(0, 2)}</div>;
  }
  return (
    <img
      className="icon"
      src={`/icons/${file}`}
      alt={alt}
      onError={() => setFailed(true)}
    />
  );
}

export default function App() {
  const [status, setStatus] = useState<main.Status | null>(null);
  const [save, setSave] = useState<main.SaveInfo | null>(null);
  const [players, setPlayers] = useState<main.PlayerInfo[]>([]);
  const [uid, setUid] = useState("");
  const [tab, setTab] = useState<Tab>("pals");
  const [species, setSpecies] = useState<main.SpeciesSummary[]>([]);
  const [items, setItems] = useState<main.ItemInfo[]>([]);
  const [busy, setBusy] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [toast, setToast] = useState<{ msg: string; bad?: boolean } | null>(
    null,
  );

  const say = useCallback((msg: string, bad?: boolean) => {
    setToast({ msg, bad });
    setTimeout(() => setToast(null), 5000);
  }, []);

  useEffect(() => {
    Status().then(setStatus);
  }, []);

  async function pickAndOpen() {
    try {
      const path = await PickSaveFile();
      if (!path) return;
      setBusy(true);
      const info = await OpenSave(path);
      setSave(info);
      setDirty(false);
      const ps = await Players();
      setPlayers(ps);
      setUid("");
      setSpecies([]);
      setItems([]);
      Status().then(setStatus);
      say(
        `불러옴 · 플레이어 ${info.playerCount}명, 팰 ${info.palCount}마리` +
          (info.playerSaves < info.playerCount
            ? ` · 플레이어 세이브 ${info.playerSaves}개만 발견됨`
            : ""),
      );
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  async function selectPlayer(id: string) {
    setUid(id);
    setTab("pals");
    setBusy(true);
    try {
      setSpecies(await PalSpecies(id));
      try {
        setItems(await Inventory(id));
      } catch {
        // A player with no save file has no reachable inventory; the pal list
        // still works, so this is not fatal.
        setItems([]);
      }
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  async function refresh() {
    if (!uid) return;
    setSpecies(await PalSpecies(uid));
    try {
      setItems(await Inventory(uid));
    } catch {
      setItems([]);
    }
  }

  async function write() {
    setBusy(true);
    try {
      const r = await SaveToDisk();
      setDirty(false);
      say(
        `저장 완료 · ${r.sizeBytes.toLocaleString()} 바이트 · 백업 ${r.backupPath}` +
          (r.playerSaves > 0
            ? ` · 플레이어 세이브 ${r.playerSaves}개도 기록됨`
            : ""),
      );
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  const selected = players.find((p) => p.uid === uid);
  const [camps, setCamps] = useState<main.CampInfo[]>([]);

  useEffect(() => {
    if (!save || !uid) {
      setCamps([]);
      return;
    }
    BaseCamps(uid)
      .then(setCamps)
      .catch(() => setCamps([]));
  }, [save, uid]);

  return (
    <div className="app">
      <div className="topbar">
        <img
          src={nanachiFace}
          alt=""
          className="brand-avatar"
          aria-hidden="true"
        />
        <h1>나나치의 팰월드 세이브 에디터</h1>
        {save && <span className="path">{save.path}</span>}
        <span className="spacer" />
        {dirty && <span className="dirty">● 저장하지 않은 변경</span>}
        <button onClick={pickAndOpen} disabled={busy}>
          세이브 열기
        </button>
        <button className="primary" onClick={write} disabled={busy || !dirty}>
          저장
        </button>
      </div>

      {!save ? (
        <Welcome status={status} onOpen={pickAndOpen} busy={busy} />
      ) : (
        <div className="body">
          <div className="sidebar">
            <div className="section-title">플레이어</div>
            {players.map((p) => (
              <button
                key={p.uid}
                className={`player ${p.uid === uid ? "active" : ""}`}
                onClick={() => selectPlayer(p.uid)}
              >
                <div className="name">{p.name}</div>
                <div className="meta">
                  팰 {p.palCount} · Lv {p.level}
                  {!p.hasSave && <span className="warn"> · 세이브 없음</span>}
                </div>
              </button>
            ))}
          </div>

          <div className="main">
            {!selected ? (
              <div className="empty">
                <img
                  src={nanachiIdle}
                  alt=""
                  className="empty-mascot"
                  aria-hidden="true"
                />
                왼쪽에서 편집할 플레이어를 고르세요.
              </div>
            ) : (
              <>
                <div className="tabs">
                  <button
                    className={`tab ${tab === "pals" ? "active" : ""}`}
                    onClick={() => setTab("pals")}
                  >
                    팰 {species.reduce((n, s) => n + s.count, 0)}
                  </button>
                  <button
                    className={`tab ${tab === "items" ? "active" : ""}`}
                    onClick={() => setTab("items")}
                  >
                    인벤토리 {items.length}
                  </button>
                  <button
                    className={`tab ${tab === "player" ? "active" : ""}`}
                    onClick={() => setTab("player")}
                  >
                    플레이어
                  </button>
                </div>

                {tab === "pals" ? (
                  <PalsTab
                    uid={uid}
                    camps={camps}
                    status={status}
                    busy={busy}
                    setBusy={setBusy}
                    say={say}
                    onChanged={async () => {
                      setDirty(true);
                      await refresh();
                    }}
                  />
                ) : tab === "player" ? (
                  <PlayerTab
                    uid={uid}
                    status={status}
                    busy={busy}
                    setBusy={setBusy}
                    say={say}
                    onChanged={async () => {
                      setDirty(true);
                      await refresh();
                    }}
                  />
                ) : (
                  <ItemsTab
                    uid={uid}
                    items={items}
                    hasSave={selected.hasSave}
                    busy={busy}
                    setBusy={setBusy}
                    say={say}
                    onChanged={async () => {
                      setDirty(true);
                      await refresh();
                    }}
                  />
                )}
              </>
            )}
          </div>
        </div>
      )}

      {busy && (
        <div className="working" role="status" aria-live="polite">
          <img src={nanachiWorking} alt="" aria-hidden="true" />
          <span>작업 중…</span>
        </div>
      )}

      {toast && (
        <div className={`toast ${toast.bad ? "error" : ""}`}>
          <img
            src={toast.bad ? nanachiError : nanachiSuccess}
            alt=""
            className="toast-mascot"
            aria-hidden="true"
          />
          <span>{toast.msg}</span>
        </div>
      )}
    </div>
  );
}

function Welcome({
  status,
  onOpen,
  busy,
}: {
  status: main.Status | null;
  onOpen: () => void;
  busy: boolean;
}) {
  return (
    <div className="welcome">
      <img
        src={nanachiIdle}
        alt=""
        className="welcome-mascot"
        aria-hidden="true"
      />
      <h2>세이브 파일을 여세요</h2>
      <p>
        서버의 <code>Level.sav</code> 를 고르면 같은 폴더의 <code>Players</code>{" "}
        안에 있는 플레이어 세이브도 함께 읽습니다. 인벤토리 정보가 거기 있어서
        둘 다 필요합니다.
      </p>
      <button className="primary" onClick={onOpen} disabled={busy}>
        Level.sav 선택
      </button>
      {status && (
        <div className="statusline">
          <span>
            <span className={`dot ${status.codecOk ? "ok" : "bad"}`} />
            {status.codecOk
              ? "Oodle 코덱 정상"
              : "코덱 없음 — 세이브를 열 수 없습니다"}
          </span>
          <span>
            <span className={`dot ${status.iconsOk ? "ok" : "warn"}`} />
            {status.iconsOk
              ? `아이콘 ${status.iconCount.toLocaleString()}개`
              : "아이콘 없음 (이름만 표시)"}
          </span>
        </div>
      )}
      {status && !status.codecOk && status.codecError && (
        <p style={{ color: "var(--danger)" }}>{status.codecError}</p>
      )}
    </div>
  );
}

type PalView = "box" | "party" | "base";

const VIEWS: { id: PalView; label: string }[] = [
  { id: "box", label: "팰박스" },
  { id: "party", label: "파티" },
  { id: "base", label: "거점" },
];

/** Groups a list of pals into the species cards the grid draws. */
function summarise(pals: main.PalInfo[]): main.SpeciesSummary[] {
  const by = new Map<string, main.SpeciesSummary>();
  for (const p of pals) {
    const s = by.get(p.speciesId);
    if (!s) {
      by.set(p.speciesId, {
        speciesId: p.speciesId,
        name: p.name,
        icon: p.icon,
        count: 1,
        minLevel: p.level,
        maxLevel: p.level,
      } as main.SpeciesSummary);
      continue;
    }
    s.count++;
    s.minLevel = Math.min(s.minLevel, p.level);
    s.maxLevel = Math.max(s.maxLevel, p.level);
  }
  return [...by.values()].sort((a, b) =>
    a.count !== b.count ? b.count - a.count : a.name.localeCompare(b.name),
  );
}

function PalsTab({
  uid,
  camps,
  status,
  busy,
  setBusy,
  say,
  onChanged,
}: {
  uid: string;
  camps: main.CampInfo[];
  status: main.Status | null;
  busy: boolean;
  setBusy: (b: boolean) => void;
  say: (m: string, bad?: boolean) => void;
  onChanged: () => Promise<void>;
}) {
  const [view, setView] = useState<PalView>("box");
  const [pick, setPick] = useState("");
  const [level, setLevel] = useState(50);
  const [roster, setRoster] = useState<main.PalInfo[]>([]);
  const [basePals, setBasePals] = useState<main.PalInfo[]>([]);
  const [editing, setEditing] = useState("");
  const [camp, setCamp] = useState(0); // 0 = every camp
  const [presets, setPresets] = useState<main.PresetInfo[]>([]);
  const [presetPick, setPresetPick] = useState("");
  const [managing, setManaging] = useState(false);
  const [adding, setAdding] = useState(false);
  // Instance ids ticked for bulk work. A Set so toggling one row does not
  // rebuild an array of a few hundred entries.
  const [selected, setSelected] = useState<Set<string>>(new Set());

  const loadPresets = useCallback(() => {
    Presets()
      .then(setPresets)
      .catch((e) => say(String(e), true));
  }, [say]);

  useEffect(loadPresets, [loadPresets]);

  // Both rosters are fetched once and filtered here. They are a few hundred
  // entries already in memory on the Go side, so re-fetching per view would
  // cost a round trip to save nothing.
  const load = useCallback(async () => {
    if (!uid) {
      setRoster([]);
      setBasePals([]);
      return;
    }
    try {
      const [mine, base] = await Promise.all([Pals(uid), BasePals(uid)]);
      setRoster(mine);
      setBasePals(base);
    } catch (e: any) {
      say(String(e), true);
    }
  }, [uid, say]);

  useEffect(() => {
    setPick("");
    setEditing("");
    load();
  }, [load]);

  // Base camp pals belong to the guild rather than to the selected player, so
  // that view ignores the roster entirely.
  const active =
    view === "base"
      ? camp
        ? basePals.filter((p) => p.camp === camp)
        : basePals
      : roster.filter((p) => p.location === view);

  const cards = summarise(active);
  const target = cards.find((s) => s.speciesId === pick);
  const members = pick ? active.filter((p) => p.speciesId === pick) : [];
  const current =
    [...roster, ...basePals].find((p) => p.instanceId === editing) ?? null;

  const counts: Record<PalView, number> = {
    box: roster.filter((p) => p.location === "box").length,
    party: roster.filter((p) => p.location === "party").length,
    base: basePals.length,
  };

  const afterEdit = useCallback(async () => {
    await onChanged();
    await load();
    // The editor can create a preset, and the toolbar's list is fetched
    // separately, so it would otherwise not see it until a remount.
    loadPresets();
  }, [onChanged, load, loadPresets]);

  // Applies to exactly the pals on screen. The old bulk call selected by owner
  // and species, which would have reached party pals while the palbox was
  // showing — a filtered view has to act on what it is showing.
  async function applyBulk() {
    if (!pick || members.length === 0) return;
    setBusy(true);
    try {
      for (const p of members) {
        await SetPalLevel(p.instanceId, level);
      }
      say(`${target?.name ?? pick} ${members.length}마리를 Lv${level} 로 변경`);
      await afterEdit();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  // Acts on the pals on screen, exactly like applyBulk and for the same
  // reason: selecting by species alone would reach party pals while the
  // palbox is showing.
  async function applyPresetBulk() {
    const preset = presets.find((p) => p.name === presetPick);
    if (!preset || members.length === 0) return;
    if (preset.stale) {
      say(
        `프리셋 "${preset.name}" 에 알 수 없는 패시브가 있어 적용할 수 없습니다`,
        true,
      );
      return;
    }
    setBusy(true);
    try {
      const ids = preset.passives.map((x) => x.id);
      for (const p of members) {
        await SetPalPassives(p.instanceId, ids);
      }
      say(
        `${target?.name ?? pick} ${members.length}마리에 "${preset.name}" 적용`,
      );
      await afterEdit();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  function switchView(v: PalView) {
    setView(v);
    setPick("");
    setEditing("");
    setSelected(new Set());
  }

  function toggle(id: string) {
    const next = new Set(selected);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setSelected(next);
  }

  // The ticked pals, in the order shown, filtered through what is on screen:
  // an id left over from another species would edit something invisible.
  const picked = members.filter((p) => selected.has(p.instanceId));

  // What the detail editor works on: the ticked pals when there are any,
  // otherwise the single row that was opened.
  const editTargets = picked.length > 0 ? picked : current ? [current] : [];

  return (
    <>
      <div className="subtabs">
        {VIEWS.map((v) => (
          <button
            key={v.id}
            className={`subtab ${view === v.id ? "active" : ""}`}
            onClick={() => switchView(v.id)}
          >
            {v.label} <span className="n">{counts[v.id]}</span>
          </button>
        ))}
      </div>

      <div className="toolbar">
        {view === "base" && (
          <>
            <label>거점</label>
            <select
              value={camp}
              onChange={(e) => {
                setCamp(Number(e.target.value));
                setPick("");
                setEditing("");
              }}
            >
              <option value={0}>전체 ({basePals.length}마리)</option>
              {camps.map((c) => (
                <option key={c.index} value={c.index}>
                  거점 {c.index} ({c.palCount}마리)
                </option>
              ))}
            </select>
          </>
        )}
        <label>종족</label>
        <select
          value={pick}
          onChange={(e) => {
            setPick(e.target.value);
            setEditing("");
            setSelected(new Set());
          }}
        >
          <option value="">선택…</option>
          {cards.map((s) => (
            <option key={s.speciesId} value={s.speciesId}>
              {s.name} ({s.count})
            </option>
          ))}
        </select>
        <label>레벨</label>
        <input
          type="number"
          min={1}
          max={status?.maxLevel ?? 100}
          value={level}
          onChange={(e) => setLevel(Number(e.target.value))}
        />
        <button onClick={applyBulk} disabled={busy || !pick}>
          레벨 일괄 적용
          {members.length ? ` (${members.length}마리)` : ""}
        </button>

        <span className="tb-sep" />

        <label>패시브 프리셋</label>
        <select
          value={presetPick}
          onChange={(e) => setPresetPick(e.target.value)}
          title={
            presets.length === 0
              ? "팰 하나를 열어 패시브를 고른 뒤 저장하면 여기에 나옵니다"
              : "저장해둔 패시브 조합"
          }
        >
          <option value="">
            {presets.length === 0 ? "저장된 조합 없음" : "선택…"}
          </option>
          {presets.map((p) => (
            <option key={p.name} value={p.name}>
              {p.name}
              {p.stale ? " (사용 불가)" : ""}
            </option>
          ))}
        </select>
        <button
          onClick={applyPresetBulk}
          disabled={busy || !pick || !presetPick}
          title="선택한 종족 전체의 패시브를 이 조합으로 바꿉니다"
        >
          패시브 일괄 적용
          {presetPick && members.length ? ` (${members.length}마리)` : ""}
        </button>
        <button className="ghost" onClick={() => setManaging(true)}>
          프리셋 관리
        </button>

        <span className="tb-sep" />

        <button
          onClick={() => setAdding(true)}
          title="가지고 있지 않은 팰을 팰박스에 새로 만들어 넣습니다"
        >
          팰 추가
        </button>
      </div>

      {adding && (
        <AddPalDialog
          uid={uid}
          status={status}
          onClose={() => setAdding(false)}
          say={say}
          onAdded={onChanged}
        />
      )}

      {managing && (
        <PresetManager
          max={status?.maxPassives ?? 4}
          presets={presets}
          onClose={() => setManaging(false)}
          onChanged={loadPresets}
          say={say}
        />
      )}

      {cards.length === 0 ? (
        <div className="empty">
          {view === "party"
            ? "파티에 팰이 없습니다."
            : view === "base"
              ? "이 거점에는 팰이 없습니다."
              : "팰박스가 비어 있습니다."}
        </div>
      ) : (
        <div className="split">
          <div className="grid">
            {cards.map((s) => (
              <button
                key={s.speciesId}
                className={`card ${s.speciesId === pick ? "active" : ""}`}
                onClick={() => {
                  setPick(s.speciesId);
                  setEditing("");
                  setSelected(new Set());
                }}
              >
                <Icon file={s.icon} alt={s.name} />
                <div className="info">
                  <div className="title">{s.name}</div>
                  <div className="sub">
                    {s.count}마리 · Lv{" "}
                    {s.minLevel === s.maxLevel
                      ? s.minLevel
                      : `${s.minLevel}–${s.maxLevel}`}
                  </div>
                </div>
              </button>
            ))}
          </div>

          {pick && (
            <aside className="detail">
              <div className="detail-head">
                <span className="detail-title">
                  {target?.name ?? pick} · {members.length}마리
                </span>
                <button className="ghost" onClick={() => setPick("")}>
                  닫기
                </button>
              </div>

              <div className="list-head">
                <label className="check-all">
                  <input
                    type="checkbox"
                    checked={
                      picked.length === members.length && members.length > 0
                    }
                    ref={(el) => {
                      if (el)
                        el.indeterminate =
                          picked.length > 0 && picked.length < members.length;
                    }}
                    onChange={(e) =>
                      setSelected(
                        e.target.checked
                          ? new Set(members.map((p) => p.instanceId))
                          : new Set(),
                      )
                    }
                  />
                  <span>
                    {picked.length > 0
                      ? picked.length + "마리 선택"
                      : "전체 선택"}
                  </span>
                </label>
                {picked.length > 0 && (
                  <button
                    className="ghost tiny"
                    onClick={() => setSelected(new Set())}
                  >
                    해제
                  </button>
                )}
              </div>

              <div className="pal-list">
                {members.map((p) => (
                  <div
                    key={p.instanceId}
                    className={`pal-row ${
                      p.instanceId === editing ? "active" : ""
                    } ${selected.has(p.instanceId) ? "picked" : ""}`}
                  >
                    <input
                      type="checkbox"
                      checked={selected.has(p.instanceId)}
                      onChange={() => toggle(p.instanceId)}
                      title="선택"
                    />
                    <button
                      className="pal-open"
                      onClick={() =>
                        setEditing(p.instanceId === editing ? "" : p.instanceId)
                      }
                    >
                      {view === "base" && p.camp > 0 && (
                        <span className="camp">거점{p.camp}</span>
                      )}
                      <span className="lv">Lv{p.level}</span>
                      <span className="rank">{"★".repeat(p.rank - 1)}</span>
                      <span className="passives">
                        {p.passives.length
                          ? p.passives.map((x) => x.name).join(" · ")
                          : "—"}
                      </span>
                    </button>
                  </div>
                ))}
              </div>

              {editTargets.length > 0 && (
                <PalEditor
                  key={editTargets.map((t) => t.instanceId).join(",")}
                  targets={editTargets}
                  status={status}
                  presets={presets}
                  onManage={() => setManaging(true)}
                  busy={busy}
                  setBusy={setBusy}
                  say={say}
                  onChanged={afterEdit}
                />
              )}
            </aside>
          )}
        </div>
      )}
    </>
  );
}

/**
 * Edits one pal. Values are held locally and written on 적용 rather than on
 * every keystroke, so a half-typed number never reaches the save.
 */
/**
 * The detail editor, for one pal or for a whole selection.
 *
 * One component rather than a separate bulk form: the field list would
 * otherwise drift between the two, and the fields are the interesting part.
 *
 * The difference that matters is what gets written. Editing one pal writes
 * every field, because the form was seeded from that pal and matches it.
 * Editing several seeds from the first of them, which says nothing about the
 * rest, so each group must be ticked before it is written. Without that, a
 * selection would silently inherit the first pal's talents and souls — the
 * same shape of mistake as a max button that lowered a stat.
 */
function PalEditor({
  targets,
  status,
  presets,
  onManage,
  busy,
  setBusy,
  say,
  onChanged,
}: {
  targets: main.PalInfo[];
  status: main.Status | null;
  presets: main.PresetInfo[];
  onManage: () => void;
  busy: boolean;
  setBusy: (b: boolean) => void;
  say: (m: string, bad?: boolean) => void;
  onChanged: () => Promise<void>;
}) {
  const pal = targets[0];
  const many = targets.length > 1;

  const maxLevel = status?.maxLevel ?? 80;
  const maxRank = status?.maxRank ?? 5;
  const maxTalent = status?.maxTalent ?? 100;
  const maxSoul = status?.maxRankBonus ?? 20;
  const maxPassives = status?.maxPassives ?? 4;
  const maxWork = status?.maxWork ?? 10;

  const [level, setLevel] = useState(pal.level);
  const [rank, setRank] = useState(pal.rank);
  const [talents, setTalents] = useState<Record<string, number>>({
    Talent_HP: pal.talentHp,
    Talent_Melee: pal.talentMelee,
    Talent_Shot: pal.talentShot,
    Talent_Defense: pal.talentDefense,
  });
  const [souls, setSouls] = useState<Record<string, number>>({
    Rank_HP: pal.soulHp,
    Rank_Attack: pal.soulAttack,
    Rank_Defence: pal.soulDefence,
    Rank_CraftSpeed: pal.soulCraftSpeed,
  });
  const [passives, setPassives] = useState<main.PassiveInfo[]>(pal.passives);
  // Keyed by bare job id, holding only what a book added — the species base is
  // display context and is never written.
  const [work, setWork] = useState<Record<string, number>>(() =>
    Object.fromEntries((pal.work ?? []).map((w) => [w.id, w.bonus])),
  );

  // Which groups apply. Everything is on for a single pal; nothing is on for a
  // selection until the user says so.
  const [on, setOn] = useState<Record<string, boolean>>({
    level: !many,
    rank: !many,
    talents: !many,
    souls: !many,
    work: !many,
    passives: !many,
  });
  const enable = (k: string) => setOn((o) => ({ ...o, [k]: true }));
  const nothingOn = many && !Object.values(on).some(Boolean);

  async function apply() {
    setBusy(true);
    try {
      for (const t of targets) {
        // Passives first: it is the only call that can reject the whole form,
        // so failing here leaves nothing half-written.
        if (on.passives) {
          await SetPalPassives(
            t.instanceId,
            passives.map((p) => p.id),
          );
        }
        if (on.level) await SetPalLevel(t.instanceId, level);
        if (on.rank) await SetPalRank(t.instanceId, rank);
        if (on.talents) {
          for (const x of TALENTS) {
            await SetPalTalent(t.instanceId, x.prop, talents[x.prop]);
          }
        }
        if (on.souls) {
          for (const x of SOULS) {
            await SetPalRankBonus(t.instanceId, x.prop, souls[x.prop]);
          }
        }
        if (on.work) {
          // Written against this pal's own rows, not the seed's: a mixed
          // selection has different jobs per species.
          for (const w of t.work ?? []) {
            const next = work[w.id] ?? 0;
            if (next !== w.bonus) {
              await SetPalWorkSuitability(t.instanceId, w.id, next);
            }
          }
        }
      }
      say(many ? `${targets.length}마리 수정됨` : `${pal.name} 수정됨`);
      await onChanged();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  function maxAll() {
    setLevel(maxLevel);
    setRank(maxRank);
    setTalents(Object.fromEntries(TALENTS.map((t) => [t.prop, maxTalent])));
    setSouls(Object.fromEntries(SOULS.map((s) => [s.prop, maxSoul])));
    setOn((o) => ({
      ...o,
      level: true,
      rank: true,
      talents: true,
      souls: true,
    }));
  }

  // A per-group tick, shown only when editing several pals.
  function Gate({ k }: { k: string }) {
    if (!many) return null;
    return (
      <label className="gate" title="이 항목을 선택한 팰 전체에 적용">
        <input
          type="checkbox"
          checked={!!on[k]}
          onChange={(e) => setOn({ ...on, [k]: e.target.checked })}
        />
      </label>
    );
  }

  const species = Array.from(new Set(targets.map((t) => t.name)));

  return (
    <div className="editor">
      <div className="editor-head">
        <Icon file={pal.icon} alt={pal.name} />
        <div className="info">
          <div className="title">
            {many ? `${targets.length}마리 선택됨` : pal.nickname || pal.name}
          </div>
          <div className="sub">
            {many
              ? species.slice(0, 3).join(", ") +
                (species.length > 3 ? ` 외 ${species.length - 3}종` : "")
              : pal.name}
            {!many && pal.isBoss && <span className="badge">알파</span>}
          </div>
        </div>
        <button className="ghost" onClick={maxAll} disabled={busy}>
          전부 최대
        </button>
      </div>

      {many && (
        <div className="bulk-note">
          체크한 항목만 선택한 {targets.length}마리에 적용됩니다. 값은 첫 번째
          팰에서 가져왔습니다.
        </div>
      )}

      <div className={`field-group ${many && !on.level ? "off" : ""}`}>
        <div className="group-title">
          <Gate k="level" />
          레벨 <span className="range">1–{maxLevel}</span>
        </div>
        <div className="hint">경험치도 그 레벨에 맞춰 함께 바뀝니다.</div>
        <div className="field-row">
          <input
            type="number"
            min={1}
            max={maxLevel}
            value={level}
            onChange={(e) => {
              setLevel(Number(e.target.value));
              enable("level");
            }}
          />
          <input
            type="range"
            min={1}
            max={maxLevel}
            value={level}
            onChange={(e) => {
              setLevel(Number(e.target.value));
              enable("level");
            }}
          />
        </div>
      </div>

      <div className={`field-group ${many && !on.rank ? "off" : ""}`}>
        <div className="group-title">
          <Gate k="rank" />
          응축 <span className="range">1–{maxRank}</span>
        </div>
        <div className="hint">
          같은 팰을 여러 마리 합쳐 올리는 등급입니다. 1이 합치지 않은 상태,
          {maxRank}가 풀농축(별 {maxRank - 1}개)이고 공격과 방어가 올라갑니다.
        </div>
        <div className="field-row">
          <input
            type="number"
            min={1}
            max={maxRank}
            value={rank}
            onChange={(e) => {
              setRank(Number(e.target.value));
              enable("rank");
            }}
          />
          <span className="stars">
            {"★".repeat(Math.max(0, rank - 1)) +
              "☆".repeat(Math.max(0, maxRank - rank))}
          </span>
        </div>
      </div>

      <div className={`field-group ${many && !on.talents ? "off" : ""}`}>
        <div className="group-title">
          <Gate k="talents" />
          개체값 <span className="range">0–{maxTalent}</span>
        </div>
        <div className="hint">
          태어날 때 정해지는 타고난 자질이라 게임에서는 나중에 못 바꿉니다.
          {maxTalent}이 최대입니다.
        </div>
        <div className="stat-grid">
          {TALENTS.map((t) => (
            <label key={t.prop} className="stat">
              <span>{t.label}</span>
              <input
                type="number"
                min={0}
                max={maxTalent}
                value={talents[t.prop]}
                onChange={(e) => {
                  setTalents({ ...talents, [t.prop]: Number(e.target.value) });
                  enable("talents");
                }}
              />
            </label>
          ))}
        </div>
      </div>

      <div className={`field-group ${many && !on.souls ? "off" : ""}`}>
        <div className="group-title">
          <Gate k="souls" />팰 영혼 <span className="range">0–{maxSoul}</span>
        </div>
        <div className="hint">
          팰 영혼 아이템을 먹여 올리는 강화로, 항목마다 따로 쌓입니다. 개체값과
          달리 게임 안에서도 올릴 수 있는 값입니다.
        </div>
        <div className="stat-grid">
          {SOULS.map((s) => (
            <label key={s.prop} className="stat">
              <span>{s.label}</span>
              <input
                type="number"
                min={0}
                max={maxSoul}
                value={souls[s.prop]}
                onChange={(e) => {
                  setSouls({ ...souls, [s.prop]: Number(e.target.value) });
                  enable("souls");
                }}
              />
            </label>
          ))}
        </div>
      </div>

      <div className={`field-group ${many && !on.work ? "off" : ""}`}>
        <div className="group-title">
          <Gate k="work" />
          노동 적성 <span className="range">0–{maxWork}</span>
        </div>
        <div className="hint">
          적성 향상서를 먹여 올리는 값입니다. 세이브에는{" "}
          <b>책으로 더한 분량만</b>
          기록되고, 종족이 원래 가진 적성은 따로입니다. 둘을 합치면 게임에서 몇
          등급으로 보이는지는 아직 확인하지 못해서, 여기서는 각각 그대로
          보여줍니다.
          {many && " 여러 종족을 함께 고르면 기본 적성은 첫 팰 기준입니다."}
        </div>
        <div className="work-grid">
          {(pal.work ?? []).map((w) => {
            const cur = work[w.id] ?? 0;
            const dim = cur === 0 && w.base === 0;
            return (
              <label key={w.id} className={`work-row ${dim ? "dim" : ""}`}>
                <span className="work-name">{w.name}</span>
                <span className="work-base" title="종족이 원래 가진 적성">
                  {w.base > 0 ? `기본 ${w.base}` : "—"}
                </span>
                <input
                  type="number"
                  min={0}
                  max={maxWork}
                  value={cur}
                  onChange={(e) => {
                    setWork({ ...work, [w.id]: Number(e.target.value) });
                    enable("work");
                  }}
                />
              </label>
            );
          })}
        </div>
      </div>

      <div className={`field-group-wrap ${many && !on.passives ? "off" : ""}`}>
        <PassivePicker
          chosen={passives}
          max={maxPassives}
          presets={presets}
          gate={many ? <Gate k="passives" /> : null}
          onChange={(next) => {
            setPassives(next);
            enable("passives");
          }}
          onManage={onManage}
          say={say}
        />
      </div>

      <button
        className="primary apply"
        onClick={apply}
        disabled={busy || nothingOn}
        title={nothingOn ? "적용할 항목을 체크하세요" : ""}
      >
        {many ? `선택한 ${targets.length}마리에 적용` : "이 팰에 적용"}
      </button>
    </div>
  );
}

/**
 * The passive chips plus their search box, with no notion of presets.
 *
 * Split out because the preset manager builds a set the same way the pal
 * editor does, and a picker that knew about presets could not be used inside
 * the thing that manages them.
 */
function PassiveChooser({
  chosen,
  max,
  onChange,
  say,
  placeholder,
}: {
  chosen: main.PassiveInfo[];
  max: number;
  onChange: (next: main.PassiveInfo[]) => void;
  say: (m: string, bad?: boolean) => void;
  placeholder?: string;
}) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<main.PassiveInfo[]>([]);

  useEffect(() => {
    if (!query.trim()) {
      setResults([]);
      return;
    }
    let live = true;
    SearchPassives(query)
      .then((r) => live && setResults(r.slice(0, 40)))
      .catch((e) => live && say(String(e), true));
    return () => {
      live = false;
    };
  }, [query, say]);

  function add(p: main.PassiveInfo) {
    if (chosen.some((c) => c.id === p.id)) return;
    if (chosen.length >= max) {
      say(`패시브는 최대 ${max}개까지입니다`, true);
      return;
    }
    onChange([...chosen, p]);
    setQuery("");
  }

  return (
    <>
      <div className="chips">
        {chosen.length === 0 && <span className="muted">없음</span>}
        {chosen.map((p) => (
          <button
            key={p.id}
            className={`chip rank${p.rank < 0 ? "neg" : p.rank}`}
            title={p.desc || p.id}
            onClick={() => onChange(chosen.filter((c) => c.id !== p.id))}
          >
            {p.name} <span className="x">×</span>
          </button>
        ))}
      </div>

      <input
        type="text"
        placeholder={placeholder ?? "패시브 검색 (한글/영문)"}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />

      {results.length > 0 && (
        <div className="results">
          {results.map((p) => (
            <button
              key={p.id}
              className="result"
              title={p.desc || p.id}
              onClick={() => add(p)}
            >
              <span className={`chip rank${p.rank < 0 ? "neg" : p.rank}`}>
                {p.name}
              </span>
              <span className="desc">{p.desc}</span>
            </button>
          ))}
        </div>
      )}
    </>
  );
}

/** The pal editor's passive section: a chooser plus one-click preset recall. */
function PassivePicker({
  chosen,
  max,
  presets,
  gate,
  onChange,
  onManage,
  say,
}: {
  chosen: main.PassiveInfo[];
  max: number;
  presets: main.PresetInfo[];
  // gate is the per-group apply tick, present only when editing several pals.
  gate?: React.ReactNode;
  onChange: (next: main.PassiveInfo[]) => void;
  onManage: () => void;
  say: (m: string, bad?: boolean) => void;
}) {
  return (
    <div className="field-group">
      <div className="group-title">
        {gate}
        패시브{" "}
        <span className="range">
          {chosen.length}/{max}
        </span>
      </div>
      <div className="hint">
        칩을 누르면 빠지고, 아래에서 검색해 고르면 추가됩니다. 이름 위에 잠깐
        올려두면 효과 설명이 뜹니다. 빨간 칩은 나쁜 패시브입니다.
      </div>

      <div className="preset-bar">
        <span className="preset-label">프리셋</span>
        {presets.length === 0 && <span className="muted">없음</span>}
        {presets.map((p) => (
          <button
            key={p.name}
            className={`preset-use solo ${p.stale ? "stale" : ""}`}
            title={
              (p.stale ? "알 수 없는 패시브가 들어 있습니다 — " : "") +
              p.passives.map((x) => x.name).join(", ")
            }
            disabled={p.stale}
            onClick={() => onChange(p.passives)}
          >
            {p.name}
          </button>
        ))}
        <button className="ghost preset-add" onClick={onManage}>
          프리셋 관리
        </button>
      </div>

      <PassiveChooser chosen={chosen} max={max} onChange={onChange} say={say} />
    </div>
  );
}

/**
 * Build and keep passive sets without touching a pal.
 *
 * Presets are not pal data — they are the user's own shortcuts — so making one
 * should not require opening a pal first, which is what the first version made
 * you do.
 */
function PresetManager({
  max,
  presets,
  onClose,
  onChanged,
  say,
}: {
  max: number;
  presets: main.PresetInfo[];
  onClose: () => void;
  onChanged: () => void;
  say: (m: string, bad?: boolean) => void;
}) {
  const [name, setName] = useState("");
  const [chosen, setChosen] = useState<main.PassiveInfo[]>([]);
  const [editing, setEditing] = useState("");

  function startNew() {
    setEditing("");
    setName("");
    setChosen([]);
  }

  function startEdit(p: main.PresetInfo) {
    setEditing(p.name);
    setName(p.name);
    setChosen(p.passives);
  }

  async function store() {
    try {
      await SavePreset(
        name,
        chosen.map((c) => c.id),
      );
      // Renaming means the old entry would otherwise linger under its old name.
      if (editing && editing !== name.trim()) {
        await DeletePreset(editing);
      }
      say(`프리셋 저장됨 · ${name}`);
      startNew();
      onChanged();
    } catch (e: any) {
      say(String(e), true);
    }
  }

  async function drop(target: string) {
    try {
      await DeletePreset(target);
      say(`프리셋 삭제됨 · ${target}`);
      if (editing === target) startNew();
      onChanged();
    } catch (e: any) {
      say(String(e), true);
    }
  }

  return (
    <Modal title="패시브 프리셋" onClose={onClose}>
      <div className="modal-body">
        <div className="preset-list">
          <div className="section-title">저장된 조합</div>
          {presets.length === 0 && (
            <div className="muted small">아직 없습니다.</div>
          )}
          {presets.map((p) => (
            <div
              key={p.name}
              className={`preset-item ${editing === p.name ? "active" : ""} ${
                p.stale ? "stale" : ""
              }`}
            >
              <button className="preset-open" onClick={() => startEdit(p)}>
                <div className="preset-item-name">
                  {p.name}
                  {p.stale && <span className="badge warn">사용 불가</span>}
                </div>
                <div className="preset-item-sub">
                  {p.passives.map((x) => x.name).join(", ")}
                </div>
              </button>
              <button
                className="preset-del"
                title="삭제"
                onClick={() => drop(p.name)}
              >
                ×
              </button>
            </div>
          ))}
          <button className="ghost wide" onClick={startNew}>
            + 새 프리셋
          </button>
        </div>

        <div className="preset-form">
          <div className="section-title">
            {editing ? `"${editing}" 수정` : "새 프리셋"}
          </div>

          <label className="form-row">
            <span>이름</span>
            <input
              type="text"
              placeholder="예: 작업용"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </label>

          <div className="form-row col">
            <span>
              패시브{" "}
              <span className="range">
                {chosen.length}/{max}
              </span>
            </span>
            <PassiveChooser
              chosen={chosen}
              max={max}
              onChange={setChosen}
              say={say}
              placeholder="패시브 검색해서 추가"
            />
          </div>

          <button
            className="primary"
            onClick={store}
            disabled={!name.trim() || chosen.length === 0}
            title={
              !name.trim()
                ? "이름을 입력하세요"
                : chosen.length === 0
                  ? "패시브를 하나 이상 고르세요"
                  : ""
            }
          >
            {editing ? "저장" : "만들기"}
          </button>
        </div>
      </div>
    </Modal>
  );
}

/**
 * The player's own numbers: level and the two status point tracks.
 *
 * No max button here, unlike the pal editor. The per-status cap is not known —
 * observed values run to 20 on the ordinary track and 40 on the Ex one — and a
 * max button built on a guessed cap is exactly what lowered a pal's souls from
 * 20 to 10. Type a number instead.
 */
function PlayerTab({
  uid,
  status,
  busy,
  setBusy,
  say,
  onChanged,
}: {
  uid: string;
  status: main.Status | null;
  busy: boolean;
  setBusy: (b: boolean) => void;
  say: (m: string, bad?: boolean) => void;
  onChanged: () => Promise<void>;
}) {
  const maxLevel = status?.maxLevel ?? 80;

  const [detail, setDetail] = useState<main.PlayerDetail | null>(null);
  const [level, setLevel] = useState(1);
  const [unused, setUnused] = useState(0);
  const [stats, setStats] = useState<Record<string, number>>({});
  const [ex, setEx] = useState<Record<string, number>>({});
  const [relics, setRelics] = useState<main.RelicInfo[]>([]);
  const [relicVals, setRelicVals] = useState<Record<string, number>>({});
  // Relics live in the player's own .sav, which may be absent even when
  // the player exists in the world save.
  const [relicErr, setRelicErr] = useState("");

  const load = useCallback(async () => {
    if (!uid) return;
    try {
      const d = await PlayerDetail(uid);
      setDetail(d);
      setLevel(d.level);
      setUnused(d.unused);
      setStats(
        Object.fromEntries((d.stats ?? []).map((s) => [s.key, s.value])),
      );
      setEx(Object.fromEntries((d.ex ?? []).map((s) => [s.key, s.value])));
      try {
        const r = await Relics(uid);
        setRelics(r);
        setRelicVals(Object.fromEntries(r.map((x) => [x.key, x.count])));
        setRelicErr("");
      } catch (e: any) {
        setRelics([]);
        setRelicErr(String(e));
      }
    } catch (e: any) {
      say(String(e), true);
    }
  }, [uid, say]);

  useEffect(() => {
    load();
  }, [load]);

  async function apply() {
    if (!detail) return;
    setBusy(true);
    try {
      await SetPlayerLevel(uid, level);
      await SetPlayerUnusedPoints(uid, unused);
      for (const s of detail.stats ?? []) {
        const next = stats[s.key] ?? 0;
        if (next !== s.value) await SetPlayerStat(uid, "", s.key, next);
      }
      for (const s of detail.ex ?? []) {
        const next = ex[s.key] ?? 0;
        if (next !== s.value) await SetPlayerStat(uid, "ex", s.key, next);
      }
      for (const r of relics) {
        const next = relicVals[r.key] ?? 0;
        if (next !== r.count) await SetRelic(uid, r.key, next);
      }
      say(`${detail.name} 수정됨 · Lv${level}`);
      await onChanged();
      await load();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  if (!detail) return <div className="empty">불러오는 중…</div>;

  const spent = Object.values(stats).reduce((a, b) => a + b, 0);

  return (
    <div className="editor wide">
      <div className="editor-head">
        <div className="info">
          <div className="title">{detail.name}</div>
          <div className="sub">
            투자한 포인트 {spent} · 남은 포인트 {unused}
          </div>
        </div>
      </div>

      <div className="field-group">
        <div className="group-title">
          레벨 <span className="range">1–{maxLevel}</span>
        </div>
        <div className="hint">경험치도 그 레벨에 맞춰 함께 바뀝니다.</div>
        <div className="field-row">
          <input
            type="number"
            min={1}
            max={maxLevel}
            value={level}
            onChange={(e) => setLevel(Number(e.target.value))}
          />
          <input
            type="range"
            min={1}
            max={maxLevel}
            value={level}
            onChange={(e) => setLevel(Number(e.target.value))}
          />
        </div>
      </div>

      <div className="field-group">
        <div className="group-title">남은 스테이터스 포인트</div>
        <div className="hint">
          아직 투자하지 않은 포인트입니다. 게임에서 원하는 항목에 나눠 넣을 수
          있습니다.
        </div>
        <div className="field-row">
          <input
            type="number"
            min={0}
            value={unused}
            onChange={(e) => setUnused(Number(e.target.value))}
          />
        </div>
      </div>

      <div className="field-group">
        <div className="group-title">스테이터스</div>
        <div className="hint">
          레벨업으로 얻은 포인트를 넣은 값입니다.{" "}
          <b>상한은 아직 확인하지 못했습니다</b> — 이 세이브에서 관측된 최대는
          20입니다. 게임이 받아들이지 않는 값을 넣으면 되돌려지거나 무시될 수
          있습니다.
        </div>
        <div className="work-grid">
          {(detail.stats ?? []).map((s) => (
            <label key={s.key} className="work-row">
              <span
                className="work-name"
                title={s.known ? s.key : "알 수 없는 항목"}
              >
                {s.name}
                {!s.known && <span className="badge warn">?</span>}
              </span>
              <input
                type="number"
                min={0}
                value={stats[s.key] ?? 0}
                onChange={(e) =>
                  setStats({ ...stats, [s.key]: Number(e.target.value) })
                }
              />
            </label>
          ))}
        </div>
      </div>

      {(detail.ex ?? []).length > 0 && (
        <div className="field-group">
          <div className="group-title">추가 스테이터스</div>
          <div className="hint">
            일반 스테이터스와 별개로 쌓이는 값입니다. 관측된 최대는 40으로 위쪽
            목록보다 훨씬 높아, 같은 척도가 아닙니다.{" "}
            <b>상한은 확인하지 못했습니다.</b>
          </div>
          <div className="work-grid">
            {(detail.ex ?? []).map((s) => (
              <label key={s.key} className="work-row">
                <span
                  className="work-name"
                  title={s.known ? s.key : "알 수 없는 항목"}
                >
                  {s.name}
                  {!s.known && <span className="badge warn">?</span>}
                </span>
                <input
                  type="number"
                  min={0}
                  value={ex[s.key] ?? 0}
                  onChange={(e) =>
                    setEx({ ...ex, [s.key]: Number(e.target.value) })
                  }
                />
              </label>
            ))}
          </div>
        </div>
      )}

      <div className="field-group">
        <div className="group-title">유물</div>
        <div className="hint">
          리프몽 상 같은 유물을 바친 개수입니다. <b>포획률</b>이 힘의 석상으로
          올리는 값입니다. 이 값들은 월드 세이브가 아니라 플레이어 세이브에 들어
          있어, 저장하면 그 파일도 함께 기록됩니다(백업도 따로 남습니다).
          <b> 상한은 확인하지 못했습니다</b> — 이 세이브에서 관측된 최대는
          38입니다.
        </div>
        {relicErr ? (
          <div className="muted small">{relicErr}</div>
        ) : (
          <div className="work-grid">
            {relics.map((r) => (
              <label
                key={r.key}
                className={`work-row ${(relicVals[r.key] ?? 0) === 0 ? "dim" : ""}`}
              >
                <span
                  className="work-name"
                  title={r.known ? r.key : "알 수 없는 항목"}
                >
                  {r.name}
                  {!r.known && <span className="badge warn">?</span>}
                </span>
                <input
                  type="number"
                  min={0}
                  value={relicVals[r.key] ?? 0}
                  onChange={(e) =>
                    setRelicVals({
                      ...relicVals,
                      [r.key]: Number(e.target.value),
                    })
                  }
                />
              </label>
            ))}
          </div>
        )}
      </div>

      <button className="primary apply" onClick={apply} disabled={busy}>
        플레이어에 적용
      </button>
    </div>
  );
}

function ItemsTab({
  uid,
  items,
  hasSave,
  busy,
  setBusy,
  say,
  onChanged,
}: {
  uid: string;
  items: main.ItemInfo[];
  hasSave: boolean;
  busy: boolean;
  setBusy: (b: boolean) => void;
  say: (m: string, bad?: boolean) => void;
  onChanged: () => Promise<void>;
}) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<main.ItemChoice[]>([]);
  const [chosen, setChosen] = useState<main.ItemChoice | null>(null);
  const [count, setCount] = useState(1);
  const [exact, setExact] = useState(false);

  // Where the items being shown and edited live: the player's own pockets, or
  // one of the boxes their guild built.
  const [view, setView] = useState<"player" | "camp">("player");
  const [boxes, setBoxes] = useState<main.StorageInfo[]>([]);
  const [box, setBox] = useState("");

  const loadBoxes = useCallback(async () => {
    if (!uid) return;
    try {
      const got = await BaseStorages(uid);
      setBoxes(got);
      // Keep the current box selected across a refresh; fall back to the first.
      setBox((b) =>
        got.some((s) => s.containerId === b) ? b : (got[0]?.containerId ?? ""),
      );
    } catch {
      setBoxes([]);
    }
  }, [uid]);

  useEffect(() => {
    if (view === "camp") void loadBoxes();
  }, [view, loadBoxes]);

  const chosenBox = boxes.find((b) => b.containerId === box) ?? null;
  const shown = view === "camp" ? (chosenBox?.items ?? []) : items;

  // The stack the user clicked. Held by slot rather than by object so it
  // survives a refresh: after applying, the list is rebuilt and the old
  // object is stale.
  const [pickedSlot, setPickedSlot] = useState<number | null>(null);
  const picked = shown.find((s) => s.slot === pickedSlot) ?? null;
  const setPicked = (it: main.ItemInfo | null) =>
    setPickedSlot(it ? it.slot : null);

  // Switching container or view invalidates the selection: slot 3 in another
  // box is a different stack entirely.
  useEffect(() => {
    setPickedSlot(null);
  }, [view, box]);

  async function applyStack(slot: number, count: number) {
    setBusy(true);
    try {
      if (view === "camp") {
        await SetSlotCount(box, slot, count);
      } else {
        await SetPlayerSlotCount(uid, slot, count);
      }
      say(
        count === 0
          ? `슬롯 ${slot} 비움`
          : `슬롯 ${slot} → ${count.toLocaleString()}개`,
      );
      await onChanged();
      if (view === "camp") await loadBoxes();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  useEffect(() => {
    if (query.trim().length < 1) {
      setResults([]);
      return;
    }
    let live = true;
    SearchItems(query).then((r) => live && setResults(r));
    return () => {
      live = false;
    };
  }, [query]);

  async function apply() {
    if (!chosen) return;
    if (view === "camp" && !box) return;
    setBusy(true);
    try {
      const where = view === "camp" ? boxLabel(chosenBox) : "인벤토리";
      if (exact) {
        if (view === "camp") await SetContainerItemCount(box, chosen.id, count);
        else await SetItemCount(uid, chosen.id, count);
        say(
          `${where}: ${chosen.name} 을(를) ${count.toLocaleString()}개로 설정`,
        );
      } else {
        const slot =
          view === "camp"
            ? await GiveContainerItem(box, chosen.id, count)
            : await GiveItem(uid, chosen.id, count);
        say(
          `${where}: ${chosen.name} +${count.toLocaleString()} → 슬롯 ${slot}`,
        );
      }
      await onChanged();
      if (view === "camp") await loadBoxes();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  if (!hasSave) {
    return (
      <div className="empty">
        이 플레이어의 세이브 파일을 찾지 못해 인벤토리를 읽을 수 없습니다.
        <br />
        Level.sav 옆 Players 폴더를 확인하세요.
      </div>
    );
  }

  return (
    <>
      <div className="subtabs">
        <button
          className={`subtab ${view === "player" ? "active" : ""}`}
          onClick={() => setView("player")}
        >
          소지품 <span className="n">{items.length}</span>
        </button>
        <button
          className={`subtab ${view === "camp" ? "active" : ""}`}
          onClick={() => setView("camp")}
        >
          거점 보관함 <span className="n">{boxes.length}</span>
        </button>
      </div>

      <div className="toolbar">
        {view === "camp" && (
          <>
            <label>보관함</label>
            <select value={box} onChange={(e) => setBox(e.target.value)}>
              {boxes.length === 0 && <option value="">(없음)</option>}
              {boxes.map((b) => (
                <option key={b.containerId} value={b.containerId}>
                  {boxLabel(b)}
                </option>
              ))}
            </select>
          </>
        )}
        <label>추가</label>
        <input
          type="text"
          placeholder="없는 아이템도 검색 (한글/영문)"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <select
          value={chosen?.id ?? ""}
          onChange={(e) =>
            setChosen(results.find((r) => r.id === e.target.value) ?? null)
          }
        >
          <option value="">
            {results.length ? "선택…" : "검색어를 입력하세요"}
          </option>
          {results.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </select>
        <input
          type="number"
          min={0}
          value={count}
          onChange={(e) => setCount(Number(e.target.value))}
        />
        <label>
          <input
            type="checkbox"
            checked={exact}
            onChange={(e) => setExact(e.target.checked)}
          />{" "}
          정확히 이 수량으로
        </label>
        <button onClick={apply} disabled={busy || !chosen}>
          {exact ? "수량 지정" : "추가"}
        </button>
      </div>

      {/* This row adds items the player has never owned — the search covers
          every item in the game, not just what is in the list below. That was
          not obvious when it looked like a filter over the grid. */}
      <div className="hint">
        위 검색은 <b>가지고 있지 않은 아이템도</b> 찾습니다 — 고르고 <b>추가</b>
        를 누르면 빈 칸에 새로 만들어 넣습니다. 아래 목록에서 칸을 <b>클릭</b>
        하면 그 칸만 수량을 고칠 수 있습니다.
      </div>

      {view === "camp" && boxes.length === 0 ? (
        <div className="empty">
          이 길드의 거점에 아이템을 담는 시설이 없습니다.
        </div>
      ) : shown.length === 0 ? (
        <div className="empty">
          {view === "camp"
            ? "이 보관함이 비어 있습니다."
            : "인벤토리가 비어 있습니다."}
        </div>
      ) : (
        <div className="split">
          <div className="grid">
            {shown.map((it) => (
              <button
                key={`${it.slot}-${it.itemId}`}
                className={`card ${picked?.slot === it.slot ? "picked" : ""}`}
                onClick={() => setPicked(it)}
              >
                <Icon file={it.icon} alt={it.name} />
                <div className="info">
                  <div className="title">{it.name}</div>
                  <div className="sub">슬롯 {it.slot}</div>
                </div>
                <div className="count">{it.count.toLocaleString()}</div>
              </button>
            ))}
          </div>

          {picked && (
            <StackEditor
              key={`${view}-${box}-${picked.slot}`}
              stack={picked}
              busy={busy}
              onClose={() => setPicked(null)}
              onApply={applyStack}
            />
          )}
        </div>
      )}
    </>
  );
}

/**
 * Edits one stack, addressed by the slot it sits in.
 *
 * Editing by item id cannot say "this one" when a box holds the same item
 * twice — a chest with 9,999 crude oil in one slot and 500 in another is
 * ordinary, and the id-addressed call would set both. So the click carries the
 * slot number and the Go side writes only that slot.
 */
function StackEditor({
  stack,
  busy,
  onClose,
  onApply,
}: {
  stack: main.ItemInfo;
  busy: boolean;
  onClose: () => void;
  onApply: (slot: number, count: number) => Promise<void>;
}) {
  const [count, setCount] = useState(stack.count);
  const dirty = count !== stack.count;

  return (
    <div className="detail">
      <div className="detail-head">
        <Icon file={stack.icon} alt={stack.name} />
        <div className="detail-title">{stack.name}</div>
        <button className="ghost" onClick={onClose}>
          닫기
        </button>
      </div>

      <div className="hint">
        슬롯 {stack.slot} · 현재 {stack.count.toLocaleString()}개
      </div>

      <label className="stack-field">
        <span>수량</span>
        <input
          type="number"
          min={0}
          value={count}
          autoFocus
          onChange={(e) => setCount(Number(e.target.value))}
        />
      </label>

      {/* No "max" button. The per-item stack ceiling is not in the reference
          data, and a button built on a guessed cap is what overwrote a pal's
          souls once already. These are just common round numbers. */}
      <div className="chips">
        {[1, 100, 999, 9999].map((n) => (
          <button key={n} className="chip" onClick={() => setCount(n)}>
            {n.toLocaleString()}
          </button>
        ))}
      </div>

      <div className="stack-actions">
        <button
          className="primary"
          disabled={busy || !dirty || count < 0}
          onClick={() => onApply(stack.slot, count)}
        >
          적용
        </button>
        <button
          className="ghost"
          disabled={busy || stack.count === 0}
          onClick={() => onApply(stack.slot, 0)}
          title="이 칸을 비웁니다"
        >
          비우기
        </button>
      </div>
    </div>
  );
}

/**
 * Creates a pal the player does not own and puts it in their palbox.
 *
 * Species, level, condensation and IVs are offered; passives are left to the
 * per-pal editor afterwards, so this form stays short. The Go side clones an
 * existing record and replaces what it understands, which is what produces a
 * save the game accepts.
 */
function AddPalDialog({
  uid,
  status,
  onClose,
  say,
  onAdded,
}: {
  uid: string;
  status: main.Status | null;
  onClose: () => void;
  say: (m: string, bad?: boolean) => void;
  onAdded: () => Promise<void>;
}) {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<main.PalChoice[]>([]);
  const [chosen, setChosen] = useState<main.PalChoice | null>(null);
  const [level, setLevel] = useState(1);
  const [rank, setRank] = useState(1);
  const [count, setCount] = useState(1);
  const [talents, setTalents] = useState<Record<string, number>>({});
  const [space, setSpace] = useState<number | null>(null);
  const [working, setWorking] = useState(false);

  const maxLevel = status?.maxLevel ?? 80;
  const maxRank = status?.maxRank ?? 5;
  const maxTalent = status?.maxTalent ?? 100;

  useEffect(() => {
    PalboxSpace(uid)
      .then(setSpace)
      .catch(() => setSpace(null));
  }, [uid]);

  useEffect(() => {
    let live = true;
    SearchPals(query).then((r) => live && setResults(r));
    return () => {
      live = false;
    };
  }, [query]);

  const room = space ?? 0;
  const tooMany = space !== null && count > room;

  async function create() {
    if (!chosen) return;
    setWorking(true);
    try {
      for (let i = 0; i < count; i++) {
        await AddPal(uid, chosen.id, level, rank, talents, []);
      }
      say(`${chosen.name} ${count}마리를 팰박스에 넣었습니다`);
      await onAdded();
      onClose();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setWorking(false);
    }
  }

  return (
    <Modal title="팰 추가" onClose={onClose}>
      <div className="modal-body add-pal">
        <div className="hint">
          가지고 있지 않은 팰을 <b>팰박스</b>에 새로 만들어 넣습니다.
          {space !== null && ` 남은 칸 ${room.toLocaleString()}개.`}
        </div>

        <input
          type="text"
          placeholder="팰 검색 (한글/영문)"
          value={query}
          autoFocus
          onChange={(e) => setQuery(e.target.value)}
        />

        <div className="pal-picker">
          {results.length === 0 && (
            <div className="empty">검색 결과가 없습니다.</div>
          )}
          {results.map((p) => (
            <button
              key={p.id}
              className={`card ${chosen?.id === p.id ? "picked" : ""}`}
              onClick={() => setChosen(p)}
            >
              <Icon file={p.icon} alt={p.name} />
              <div className="info">
                <div className="title">{p.name}</div>
                {/* Several entries share a Korean name — the tower bosses
                      come as a trainer-and-pal pair per difficulty — so the
                      game's own id is what tells them apart. */}
                <div className="sub">{p.id}</div>
              </div>
            </button>
          ))}
        </div>

        <div className="stat-grid">
          <label>
            <span>
              레벨 <span className="range">1–{maxLevel}</span>
            </span>
            <input
              type="number"
              min={1}
              max={maxLevel}
              value={level}
              onChange={(e) => setLevel(Number(e.target.value))}
            />
          </label>
          <label>
            <span>
              응축 <span className="range">1–{maxRank}</span>
            </span>
            <input
              type="number"
              min={1}
              max={maxRank}
              value={rank}
              onChange={(e) => setRank(Number(e.target.value))}
            />
          </label>
          <label>
            <span>마리 수</span>
            <input
              type="number"
              min={1}
              value={count}
              onChange={(e) => setCount(Number(e.target.value))}
            />
          </label>
        </div>

        <div className="section-title">
          개체값 <span className="range">0–{maxTalent}</span>
        </div>
        <div className="stat-grid">
          {TALENTS.map((t) => (
            <label key={t.prop}>
              <span>{t.label}</span>
              <input
                type="number"
                min={0}
                max={maxTalent}
                value={talents[t.prop] ?? 0}
                onChange={(e) =>
                  setTalents({ ...talents, [t.prop]: Number(e.target.value) })
                }
              />
            </label>
          ))}
        </div>

        {tooMany && (
          <div className="hint bad">
            팰박스에 {room.toLocaleString()}칸밖에 없습니다.
          </div>
        )}

        <div className="stack-actions">
          <button
            className="primary"
            disabled={working || !chosen || count < 1 || tooMany}
            onClick={create}
          >
            {chosen ? `${chosen.name} ${count}마리 추가` : "팰을 고르세요"}
          </button>
        </div>

        <div className="hint">
          패시브는 추가한 뒤 개별 편집 화면에서 붙이면 됩니다.
        </div>
      </div>
    </Modal>
  );
}
