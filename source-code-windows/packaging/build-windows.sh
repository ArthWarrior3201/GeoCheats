#!/usr/bin/env bash
set -euo pipefail
command -v x86_64-w64-mingw32-windres >/dev/null
mkdir -p out
x86_64-w64-mingw32-windres --preprocessor=cpp -O coff packaging/geocheats.rc cmd/geocheats/icon_windows_amd64.syso
x86_64-w64-mingw32-windres --preprocessor=cpp -O coff packaging/dash.rc cmd/dash/icon_windows_amd64.syso
cp cmd/dash/icon_windows_amd64.syso cmd/dash-desktop/icon_windows_amd64.syso
export GOOS=windows GOARCH=amd64 CGO_ENABLED=0
go build -trimpath -ldflags='-s -w -H=windowsgui' -o out/GeoCheats-Windows.exe ./cmd/geocheats
go build -trimpath -ldflags='-s -w -H=windowsgui' -o out/Dash-Windows.exe ./cmd/dash-desktop
go build -trimpath -ldflags='-s -w' -o out/Dash-CLI.exe ./cmd/dash
cp mod_building_guide.txt out/
