# 세이브 편집 가이드

서버에서 세이브를 꺼내 → 에디터로 고치고 → 다시 넣는 전체 과정.

> **먼저 알아둘 것**
> 세이브를 되돌릴 방법은 백업뿐입니다. 3단계를 건너뛰지 마세요.
> 그리고 **서버를 멈추면 접속 중인 사람이 전부 튕깁니다.** 1단계를 먼저 하세요.

---

## 0. 준비물

- 에디터: [Releases](https://github.com/CrecentMoonBack/nanachi-palsave-editor/releases) 에서 최신 zip
- 서버 SSH 접속 (`ssh root@<서버주소>`)
- 윈도우 PC

압축을 풀면 `NanachiPalSaveEditor.exe` 와 `nanachi_ooz.dll` 이 같은 폴더에 있어야 합니다.
**둘을 따로 옮기면 세이브를 못 엽니다.**

팰 그림까지 보고 싶으면 압축 푼 폴더에서 **`fetch-icons.bat` 를 더블클릭**하세요. 딱 한 번만 하면 됩니다.

- 설치할 것도, 명령어 입력도 없습니다. 검은 창이 뜨고 다 받으면 "완료!" 가 나옵니다.
- 파란색 "Windows의 PC 보호" 경고가 뜨면 → **추가 정보** → **실행**.
- git 이 깔려 있으면 약 34MB, 없으면 자동으로 전체본(약 380MB)을 받아 그림만 꺼냅니다. 시간이 좀 더 걸려도 그대로 두면 됩니다.
- 맥/리눅스거나 명령어가 편하면 대신 `bash fetch-icons.sh` 를 써도 됩니다.

안 해도 기능은 전부 동작합니다(그림 대신 이름이 한글로 표시됨).

---

## 1. 접속자 확인 — 제일 먼저

```bash
ssh root@<서버주소>
ps -eo stat,comm | grep PalServer-Linux
```

| 결과 | 뜻 | 할 일 |
|---|---|---|
| `T` | 아무도 없음 (자동 일시정지 상태) | 바로 진행 |
| `R`, `Rl`, `S` 등 | **누군가 접속 중** | 나갈 때까지 기다리거나 미리 공지 |

접속자가 있는데 서버를 내리면 그 사람은 그냥 튕깁니다.
마지막 자동저장 이후 플레이한 내용도 날아갑니다.

---

## 2. 서버 정지

```bash
docker stop palworld
```

**왜 꼭 멈춰야 하나:** 서버가 켜져 있으면 게임이 세이브를 메모리에 들고 있다가
주기적으로 파일에 덮어씁니다. 켜둔 채 편집해서 올리면 **몇 분 뒤 게임이
자기 메모리 내용으로 덮어써서 편집이 통째로 사라집니다.**

---

## 3. 백업 — 건너뛰지 마세요

```bash
cd /opt/palworld/data/Pal/Saved/SaveGames/0/275C56A87CE54A0DA8F37E5EC74EBD6F
tar czf ~/save-backup-$(date +%Y%m%d-%H%M).tar.gz Level.sav LevelMeta.sav Players
ls -lh ~/save-backup-*.tar.gz
```

뭔가 잘못되면 이 파일이 유일한 복구 수단입니다.

> 서버는 매일 04:00 자동 백업도 남기지만(`/opt/palworld/data/backups/`, 7일 보관),
> 그건 오늘 편집 직전 상태가 아닙니다. 직접 뜨세요.

---

## 4. 내 PC로 가져오기

**내 PC에서** 실행합니다(서버가 아닙니다). PowerShell·Git Bash 어느 쪽이든 됩니다.

경로가 길어서 한 번만 변수로 잡아둡니다.

PowerShell:

```powershell
cd $env:USERPROFILE\Downloads
mkdir palsave
$W = "root@<서버주소>:/opt/palworld/data/Pal/Saved/SaveGames/0/275C56A87CE54A0DA8F37E5EC74EBD6F"
scp "$W/Level.sav"     palsave\
scp "$W/LevelMeta.sav" palsave\
scp -r "$W/Players"    palsave\
```

Git Bash:

```bash
cd ~/Downloads && mkdir -p palsave
W=root@<서버주소>:/opt/palworld/data/Pal/Saved/SaveGames/0/275C56A87CE54A0DA8F37E5EC74EBD6F
scp "$W/Level.sav" "$W/LevelMeta.sav" palsave/
scp -r "$W/Players" palsave/
```

받고 나면 이렇게 되어 있어야 합니다:

```
palsave/
├── Level.sav        ← 월드 전체 (팰, 거점, 상자)
├── LevelMeta.sav    ← 월드 정보
└── Players/         ← 플레이어별 파일 (인벤토리·레벨이 여기 있음)
    ├── 2D0A96AD000000000000000000000000.sav
    └── ...
```

**`Players` 폴더가 반드시 있어야 합니다.** 인벤토리와 플레이어 스탯은
`Level.sav` 가 아니라 이쪽에 들어 있어서, 빠지면 에디터에서 인벤토리 탭이
비어 보입니다.

---

## 5. 편집

1. `NanachiPalSaveEditor.exe` 실행
2. **세이브 열기** → 방금 받은 `palsave/Level.sav` 선택
   (같은 폴더의 `Players` 는 알아서 같이 읽습니다)
3. 왼쪽에서 편집할 플레이어 선택
4. 고친다
5. 오른쪽 위 **저장**

저장하면 원본 옆에 `Level.sav.<날짜>.bak` 이 자동으로 생깁니다.

### 할 수 있는 것

| 탭 | 내용 |
|---|---|
| **팰** | 레벨·응축·개체값·팰 영혼·패시브·노동 적성. 없는 팰 새로 추가. 팰박스/파티/거점별로 나눠 보기 |
| **인벤토리** | 소지품과 거점 보관함. 칸을 클릭해 수량 변경, 없는 아이템 추가 |
| **플레이어** | 레벨·경험치·스테이터스 포인트·힘의 석상 등 유물 |

---

## 6. 서버로 되돌리기

내 PC에서 (`$W` 는 4단계에서 잡아둔 그 변수):

PowerShell:

```powershell
scp palsave\Level.sav     "$W/"
scp palsave\LevelMeta.sav "$W/"
scp -r palsave\Players    "$W/"
```

Git Bash:

```bash
scp palsave/Level.sav palsave/LevelMeta.sav "$W/"
scp -r palsave/Players "$W/"
```

**그다음 소유권을 반드시 고칩니다.** 서버에서:

```bash
chown -R ubuntu:ubuntu /opt/palworld/data/Pal/Saved/SaveGames/0/275C56A87CE54A0DA8F37E5EC74EBD6F
```

> `scp` 를 root로 하면 파일 주인이 `root` 가 됩니다. 컨테이너 안의 게임은
> `ubuntu` 로 돌기 때문에, 이걸 안 고치면 **게임이 세이브에 쓰지 못해서
> 이후 진행 상황이 저장되지 않습니다.** 바로 티가 안 나서 제일 위험합니다.

---

## 7. 서버 시작 + 확인

```bash
docker start palworld
docker logs -f palworld
```

로그가 안정되면(1~2분) 게임에서 접속해 확인합니다.

**반드시 게임에 들어가서 눈으로 확인하세요.**
파일을 다시 읽어보는 건 검증이 아닙니다 — 내가 쓴 걸 그대로 읽는 것뿐이라,
게임이 그 데이터를 받아들일지는 알 수 없습니다. 게임은 유효하지 않은 항목을
**조용히 버리고** 자동저장으로 덮어씁니다.

---

## 문제가 생기면

**되돌리기**

```bash
docker stop palworld
cd /opt/palworld/data/Pal/Saved/SaveGames/0/275C56A87CE54A0DA8F37E5EC74EBD6F
tar xzf ~/save-backup-<날짜>.tar.gz
chown -R ubuntu:ubuntu .
docker start palworld
```

**증상별**

| 증상 | 원인 |
|---|---|
| 에디터가 세이브를 못 엶 | `nanachi_ooz.dll` 이 exe 옆에 없음 |
| 인벤토리 탭이 비어 있음 | `Players` 폴더를 같이 안 가져옴 |
| 편집한 게 몇 분 뒤 사라짐 | 서버를 안 멈추고 편집함 (2단계) |
| 접속 후 진행이 저장 안 됨 | 파일 주인이 `root` 로 남음 (6단계) |
| 팰이 사라짐 | 게임이 유효하지 않다고 판단해 버림 — 백업에서 복구 |

---

## 부록: 싱글 플레이 세이브

혼자 하는 월드는 서버가 아니라 PC에 있습니다.

```
%LOCALAPPDATA%\Pal\Saved\SaveGames\<스팀ID>\<월드ID>\
```

탐색기 주소창에 `%LOCALAPPDATA%\Pal\Saved\SaveGames` 를 붙여넣으면 열립니다.
숫자 폴더가 여러 개면 **수정한 날짜가 가장 최근인 것**이 최근에 플레이한 월드입니다.

**게임을 완전히 종료한 뒤** 편집하세요. 켜둔 채로 고치면 게임이 나갈 때
덮어씁니다. 폴더를 통째로 복사해두는 것이 백업입니다.
