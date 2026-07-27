#Requires -Version 5.1
<#
    Populates assets/icons/ with pal and item artwork, on this machine only.
    This is the Windows-friendly twin of fetch-icons.sh — same job, no Git Bash
    or WSL needed.

    Why this is a script and not a folder in the repository:
      The artwork is Pocketpair's. This project ships no game assets, and that
      applies to release archives too, which is why a downloaded release has no
      images until this is run. The editor works fully without them: it just
      falls back to Korean text names.

    Where the images come from:
      palworld-save-pal (https://github.com/oMaN-Rod/palworld-save-pal), under
      ui/src/lib/assets/img/. Only the flat .webp files are taken; the img/app/
      subfolder is that project's own branding and is left behind.

    How it fetches:
      - With git installed: a sparse checkout of just that one folder (~34 MB).
      - Without git: downloads the repo zip (~380 MB, one time) and pulls only
        the .webp out of it. Slower, but needs nothing installed — Windows 10
        and 11 already have everything used here.

    Usage:
      Right-click fetch-icons.bat -> nothing to install, just run it.
      or:  powershell -ExecutionPolicy Bypass -File fetch-icons.ps1
      Add -Force to fetch again even when icons are already present.

    Idempotent: re-running does nothing once the icons are in place.
#>
[CmdletBinding()]
param([switch]$Force)

$ErrorActionPreference = 'Stop'

# Destination mirrors the shell script: in the repo the icons go under the
# project root (one level up from scripts/); in a release the script sits beside
# the exe and the icons must land right next to it, where internal/icons looks.
$scriptDir = $PSScriptRoot
if (Test-Path (Join-Path $scriptDir '..\go.mod')) {
    $root = (Resolve-Path (Join-Path $scriptDir '..')).Path
} else {
    $root = $scriptDir
}
$dest = Join-Path $root 'assets\icons'

$upstreamGit = 'https://github.com/oMaN-Rod/palworld-save-pal.git'
$upstreamZip = 'https://github.com/oMaN-Rod/palworld-save-pal/archive/refs/heads/main.zip'
$imgPath = 'ui/src/lib/assets/img'

function Count-Webp($dir) {
    if (Test-Path $dir) {
        @(Get-ChildItem -LiteralPath $dir -Filter *.webp -File -ErrorAction SilentlyContinue).Count
    } else {
        0
    }
}

$before = Count-Webp $dest
Write-Host "설치 위치: $dest"
Write-Host "이미 있는 아이콘: ${before}개"

if ($before -gt 0 -and -not $Force) {
    Write-Host ""
    Write-Host "이미 설치돼 있어요. 다시 받으려면 -Force 를 붙여 실행하세요."
    return
}

New-Item -ItemType Directory -Force -Path $dest | Out-Null
$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("pal-icons-" + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force -Path $tmp | Out-Null
try {
    if (Get-Command git -ErrorAction SilentlyContinue) {
        Write-Host "git 발견 — 필요한 폴더만 받습니다 (약 34MB)..."
        $checkout = Join-Path $tmp 'src'
        & git clone --quiet --depth 1 --filter=blob:none --sparse $upstreamGit $checkout
        if ($LASTEXITCODE -ne 0) { throw "git clone 실패 (인터넷 연결을 확인하세요)" }
        & git -C $checkout sparse-checkout set --no-cone $imgPath
        if ($LASTEXITCODE -ne 0) { throw "sparse-checkout 실패" }
        $src = Join-Path $checkout ($imgPath -replace '/', '\')
        if (-not (Test-Path $src)) { throw "업스트림 구조가 바뀌었어요: $imgPath 를 찾을 수 없습니다." }
        # The glob takes only the top level, so img/app/ is left behind without
        # needing a filter.
        Copy-Item -Path (Join-Path $src '*.webp') -Destination $dest -Force
    } else {
        Write-Host "git이 없어서 전체 압축본을 받아 아이콘만 꺼냅니다 (약 380MB, 한 번만 받으면 됩니다)..."
        $zip = Join-Path $tmp 'repo.zip'
        if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
            # curl.exe streams straight to disk with a progress bar and does not
            # hold the whole file in memory. Present on Windows 10 1803+ / 11.
            & curl.exe -L --fail -o $zip $upstreamZip
            if ($LASTEXITCODE -ne 0) { throw "다운로드 실패 (인터넷 연결을 확인하세요)" }
        } else {
            Write-Host "  (curl 이 없어 내장 다운로더로 받습니다 — 시간이 좀 더 걸릴 수 있어요)"
            Invoke-WebRequest -Uri $upstreamZip -OutFile $zip
        }

        Write-Host "아이콘 추출 중..."
        # Windows PowerShell 5.1 needs this assembly loaded; PowerShell 7 already
        # has the type, so only load it when it is missing.
        if (-not ('System.IO.Compression.ZipFile' -as [type])) {
            Add-Type -AssemblyName System.IO.Compression.FileSystem
        }
        $archive = [System.IO.Compression.ZipFile]::OpenRead($zip)
        try {
            # Entries look like <repo>-main/ui/src/lib/assets/img/<name>.webp.
            # The [^/]+ keeps it to the top level, dropping the img/app/ branding.
            $rx = [regex]'(^|/)ui/src/lib/assets/img/[^/]+\.webp$'
            foreach ($entry in $archive.Entries) {
                if ($entry.FullName -match $rx) {
                    $out = Join-Path $dest $entry.Name
                    [System.IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $out, $true)
                }
            }
        } finally {
            $archive.Dispose()
        }
    }

    $after = Count-Webp $dest
    if ($after -eq 0) { throw "아이콘을 하나도 받지 못했어요." }
    Write-Host ""
    Write-Host "완료! $dest 에 아이콘 ${after}개 ($($after - $before)개 새로 추가)."
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
