import { useCallback, useEffect, useState } from "react";
import "./style.css";
import {
  GiveItem,
  Inventory,
  OpenSave,
  PalSpecies,
  Pals,
  PickSaveFile,
  Players,
  SaveToDisk,
  SearchItems,
  SetItemCount,
  SetPalLevelBulk,
  Status,
} from "../wailsjs/go/main/App";
import { main } from "../wailsjs/go/models";

type Tab = "pals" | "items";

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
  const [toast, setToast] = useState<{ msg: string; bad?: boolean } | null>(null);

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
            : "")
      );
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  async function selectPlayer(id: string) {
    setUid(id);
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
      say(`저장 완료 · ${r.sizeBytes.toLocaleString()} 바이트 · 백업 ${r.backupPath}`);
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  const selected = players.find((p) => p.uid === uid);

  return (
    <div className="app">
      <div className="topbar">
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
              <div className="empty">왼쪽에서 편집할 플레이어를 고르세요.</div>
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
                </div>

                {tab === "pals" ? (
                  <PalsTab
                    uid={uid}
                    species={species}
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

      {toast && (
        <div className={`toast ${toast.bad ? "error" : ""}`}>{toast.msg}</div>
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
            {status.codecOk ? "Oodle 코덱 정상" : "코덱 없음 — 세이브를 열 수 없습니다"}
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

function PalsTab({
  uid,
  species,
  busy,
  setBusy,
  say,
  onChanged,
}: {
  uid: string;
  species: main.SpeciesSummary[];
  busy: boolean;
  setBusy: (b: boolean) => void;
  say: (m: string, bad?: boolean) => void;
  onChanged: () => Promise<void>;
}) {
  const [pick, setPick] = useState("");
  const [level, setLevel] = useState(50);

  const target = species.find((s) => s.speciesId === pick);

  async function apply() {
    if (!pick) return;
    setBusy(true);
    try {
      const n = await SetPalLevelBulk(uid, pick, level);
      say(`${target?.name ?? pick} ${n}마리를 Lv${level} 로 변경`);
      await onChanged();
    } catch (e: any) {
      say(String(e), true);
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <div className="toolbar">
        <label>종족</label>
        <select value={pick} onChange={(e) => setPick(e.target.value)}>
          <option value="">선택…</option>
          {species.map((s) => (
            <option key={s.speciesId} value={s.speciesId}>
              {s.name} ({s.count})
            </option>
          ))}
        </select>
        <label>레벨</label>
        <input
          type="number"
          min={1}
          max={100}
          value={level}
          onChange={(e) => setLevel(Number(e.target.value))}
        />
        <button onClick={apply} disabled={busy || !pick}>
          일괄 적용
          {target ? ` (${target.count}마리)` : ""}
        </button>
      </div>

      {species.length === 0 ? (
        <div className="empty">이 플레이어는 팰이 없습니다.</div>
      ) : (
        <div className="grid">
          {species.map((s) => (
            <button
              key={s.speciesId}
              className={`card ${s.speciesId === pick ? "active" : ""}`}
              onClick={() => setPick(s.speciesId)}
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
      )}
    </>
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
    setBusy(true);
    try {
      if (exact) {
        await SetItemCount(uid, chosen.id, count);
        say(`${chosen.name} 을(를) ${count.toLocaleString()}개로 설정`);
      } else {
        const slot = await GiveItem(uid, chosen.id, count);
        say(`${chosen.name} +${count.toLocaleString()} → 슬롯 ${slot}`);
      }
      await onChanged();
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
      <div className="toolbar">
        <input
          type="text"
          placeholder="아이템 검색 (한글/영문)"
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
          적용
        </button>
      </div>

      {items.length === 0 ? (
        <div className="empty">인벤토리가 비어 있습니다.</div>
      ) : (
        <div className="grid">
          {items.map((it) => (
            <div key={`${it.slot}-${it.itemId}`} className="card">
              <Icon file={it.icon} alt={it.name} />
              <div className="info">
                <div className="title">{it.name}</div>
                <div className="sub">슬롯 {it.slot}</div>
              </div>
              <div className="count">{it.count.toLocaleString()}</div>
            </div>
          ))}
        </div>
      )}
    </>
  );
}
