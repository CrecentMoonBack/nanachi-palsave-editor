# 나나치의 팰월드 세이브 에디터

팰월드 데디케이티드 서버 세이브 에디터. Go + Wails 데스크탑 앱.

`Level.sav` 를 열어 플레이어를 고르고, 그 플레이어의 팰과 인벤토리를 보고
수정한다. 저장할 때마다 자동으로 백업을 남긴다.

> **상태**: 코어는 실제 운영 서버 세이브로 검증됨. GUI는 빌드·테스트는
> 통과했지만 아직 실사용 다듬기가 남아 있음. CLI 쪽이 더 검증된 경로다.

---

## 무엇을 하나

- 팰 일괄 편집 — 종족별로 골라 레벨을 한 번에 변경
- 팰 강화 정보 — 레벨, 응축 랭크, 패시브, 개체값(IV)
- 인벤토리 — 아이템 검색(한글), 지급, 수량 지정
- 한글 이름 내장 — 아이템 2,372개 / 팰 809개
- 아이콘 표시 (선택) — 없으면 이름으로 폴백

## 왜 만들었나

기존 도구는 대부분 Python이다. 실제로 써보니 두 가지가 걸렸다.

**배포가 번거롭다.** venv를 만들고 C 확장을 빌드해야 세이브 하나를 열 수 있다.

**대용량 세이브에서 죽는다.** 46MB짜리 GVAS 트리를 파싱한 뒤 Python 3.12가
객체 그래프를 해제하는 단계에서 재현성 있게 세그폴트가 났다. 파싱은 끝났는데
종료할 때 죽으니 출력이 버퍼에 남은 채 사라졌다.

Go로 옮기면 단일 실행 파일이 되고, 그 해제 문제 자체가 없다.

## 빠른 시작

### 실행만 할 때

[Releases](../../releases) 에서 받아 압축을 풀고 `NanachiPalSaveEditor.exe` 실행.
`nanachi_ooz.dll` 이 **같은 폴더에 있어야 한다** — 없으면 세이브를 열 때 실패한다.

### 소스에서 빌드

```sh
git clone https://github.com/CrecentMoonBack/nanachi-palsave-editor.git
cd nanachi-palsave-editor
bash scripts/setup.sh     # 의존성 확인, ooz 클론, DLL 빌드, npm install
bash build.sh             # 데스크탑 앱 + CLI
```

필요한 것: Go 1.25+, [Wails v2](https://wails.io), Node 20+, MinGW-w64 g++.

`setup.sh --all` 을 쓰면 아이콘과 테스트용 세이브도 서버에서 받아온다(SSH 필요).
둘 다 없어도 빌드와 실행에는 문제 없다.

## CLI

GUI보다 먼저 만들었고 더 많이 검증됐다.

```sh
palsave info   <Level.sav>
palsave players <Level.sav>
palsave pals   <Level.sav> -owner <uid> [-species <id>]
palsave inv    <Level.sav> -player <Players/UID.sav>

palsave set-level <Level.sav> -owner <uid> -species <id> -level <n> [-dry-run]
palsave give      <Level.sav> -player <Players/UID.sav> -item <id> -count <n> [-set]
```

쓰기 명령은 항상 타임스탬프 백업을 먼저 만든다. `-dry-run` 으로 미리 확인할 수 있다.

## 구조

```
internal/oodle/     .sav 컨테이너. Oodle(Kraken) 압축·해제
internal/gvas/      언리얼 GVAS 프로퍼티 트리 디코더/인코더
internal/palsave/   팰월드 전용 RawData 코덱 + 타입 안전한 접근자
internal/paldata/   내장 참조 데이터 (한글 이름, 아이콘 파일명)
internal/icons/     로컬 아이콘 폴더 서빙 (선택)
cmd/palsave/        CLI
app.go              Wails 바인딩
frontend/           React + TypeScript
```

### 이 프로젝트를 지탱하는 규칙 하나

**디코드한 뒤 다시 인코드하면 원본과 바이트 단위로 같아야 한다.**

세이브 편집기는 건드리지 않은 데이터를 조용히 망가뜨릴 수 있다. 그래서 실제
46MB 세이브를 대상으로 왕복 일치를 테스트로 못 박아뒀다. 캐릭터 blob 1,670개,
아이템 슬롯 21,079개도 각각 검증한다.

압축 계층은 예외다. ooz의 Kraken 인코더는 RAD의 것과 달라서 같은 데이터가 22%
작게 나온다. **그래서 파일 크기는 무결성 지표가 아니다** — 엔티티 개수로 판단한다.

## 라이선스

**GPL-3.0.** [ooz](https://github.com/zao/ooz) 를 링크하기 때문에 선택의 여지가 없다.

게임 아트워크는 저장소에 들어가지 않는다. Pocketpair 소유이고, 다른 도구의
저장소에서 가져오든 팬사이트에서 긁든 *누구에게서 가져오는가*가 바뀔 뿐
*사용권이 있는가*는 바뀌지 않는다. 자세한 내용과 참조 데이터의 출처는
[`docs/THIRD_PARTY.md`](docs/THIRD_PARTY.md) 에 정리해뒀다.

## 참고한 구현

이 프로젝트는 다음 작업들 위에 서 있다.

- [oMaN-Rod/palworld-save-pal](https://github.com/oMaN-Rod/palworld-save-pal) — Rust + Svelte 에디터. 가장 완성도 높고, 정확성 기준으로 삼았다
- [deafdudecomputers/PalworldSaveTools](https://github.com/deafdudecomputers/PalworldSaveTools) — Python. Oodle 호출 규약을 여기서 가져왔다
- [zao/ooz](https://github.com/zao/ooz) — Kraken 인코더. 상류 [powzix/ooz](https://github.com/powzix/ooz) 는 디코더만 있다

## 형제 프로젝트

[NanachiDeprotector](https://github.com/CrecentMoonBack) — 워크래프트 3 맵 디프로텍터.
같은 스택(Go + Wails), 같은 DLL 바인딩 방식.
