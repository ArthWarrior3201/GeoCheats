#!/usr/bin/env bash
set -euo pipefail
windres_bin=windres
if command -v x86_64-w64-mingw32-windres >/dev/null; then windres_bin=x86_64-w64-mingw32-windres; fi
command -v "$windres_bin" >/dev/null
mkdir -p out
"$windres_bin" -O coff packaging/geocheats.rc cmd/geocheats/icon_windows_amd64.syso
"$windres_bin" -O coff packaging/dash.rc cmd/dash/icon_windows_amd64.syso
cp cmd/dash/icon_windows_amd64.syso cmd/dash-desktop/icon_windows_amd64.syso
export GOOS=windows GOARCH=amd64 CGO_ENABLED=1
go build -trimpath -ldflags='-s -w -H=windowsgui -extldflags=-static' -o out/GeoCheats.exe ./cmd/geocheats
go build -trimpath -ldflags='-s -w -H=windowsgui -extldflags=-static' -o out/Dash.exe ./cmd/dash-desktop
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o out/Dash-CLI.exe ./cmd/dash
cp mod_building_guide.txt out/
cp THIRD_PARTY_NOTICES.txt out/
