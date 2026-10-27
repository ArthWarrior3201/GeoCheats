#!/usr/bin/env bash
set -euo pipefail
[[ "$(uname -s)" == Darwin ]] || { echo 'Mac packaging requires a macOS host.' >&2; exit 1; }
work=$(mktemp -d)
keychain=""
cleanup() { if [[ -n "$keychain" ]]; then security delete-keychain "$keychain" >/dev/null 2>&1 || true; fi; rm -rf "$work"; }
trap cleanup EXIT
stage="$work/GeoCheats"
mkdir -p "$stage" out

# A supplied Developer ID identity is imported only into a temporary CI keychain.
identity='-'
if [[ -n "${MACOS_CERTIFICATE_P12_BASE64:-}" ]]; then
  export MACOS_CERTIFICATE_PATH="$work/developer.p12"
  python3 - <<'PY'
import base64,os
from pathlib import Path
Path(os.environ['MACOS_CERTIFICATE_PATH']).write_bytes(base64.b64decode(os.environ['MACOS_CERTIFICATE_P12_BASE64'],validate=True))
PY
  keychain="$work/build.keychain-db"
  key_password=$(uuidgen)
  security create-keychain -p "$key_password" "$keychain"
  security set-keychain-settings -lut 21600 "$keychain"
  security unlock-keychain -p "$key_password" "$keychain"
  security import "$MACOS_CERTIFICATE_PATH" -k "$keychain" -P "$MACOS_CERTIFICATE_PASSWORD" -T /usr/bin/codesign >/dev/null
  security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$key_password" "$keychain" >/dev/null
  identity=$(security find-identity -v -p codesigning "$keychain" | awk '/Developer ID Application/ {print $2; exit}')
  [[ -n "$identity" ]] || { echo 'No Developer ID Application identity found.' >&2; exit 1; }
fi

for arch in amd64 arm64; do
  mkdir -p "$work/$arch"
  GOOS=darwin GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$work/$arch/GeoCheats" ./cmd/geocheats
  GOOS=darwin GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$work/$arch/Dash" ./cmd/dash-desktop
  GOOS=darwin GOARCH="$arch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$work/$arch/dash-cli" ./cmd/dash
done

for name in GeoCheats Dash; do
  app="$stage/$name.app"
  mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
  lipo -create "$work/amd64/$name" "$work/arm64/$name" -output "$app/Contents/MacOS/$name"
  chmod 755 "$app/Contents/MacOS/$name"
  cp "packaging/icons/$name.icns" "$app/Contents/Resources/"
  cp mod_building_guide.txt "$app/Contents/Resources/"
  if [[ "$name" == GeoCheats ]]; then cp packaging/Info.plist "$app/Contents/Info.plist"; else
    cp packaging/Dash-Info.plist "$app/Contents/Info.plist"
    mkdir -p "$app/Contents/Helpers"
    lipo -create "$work/amd64/dash-cli" "$work/arm64/dash-cli" -output "$app/Contents/Helpers/dash"
    chmod 755 "$app/Contents/Helpers/dash"
  fi
  # Sign nested executables first, then seal the complete app after all resources exist.
  sign=(--force --sign "$identity")
  if [[ "$identity" == '-' ]]; then sign+=(--timestamp=none); else sign+=(--keychain "$keychain" --options runtime --timestamp); fi
  if [[ "$name" == Dash ]]; then codesign "${sign[@]}" "$app/Contents/Helpers/dash"; fi
  if [[ "$name" == GeoCheats ]]; then codesign "${sign[@]}" --entitlements packaging/Debugging.entitlements "$app"; else codesign "${sign[@]}" "$app"; fi
done
cp MAC-SUPPORT.txt mod_building_guide.txt "$stage/"
ln -s /Applications "$stage/Applications"

if [[ "$identity" == '-' ]]; then
  printf '%s\n' 'Locally signed (ad-hoc); NOT Apple Developer ID signed or notarized. Signature integrity and native launching are tested. Gatekeeper trust is a separate check.' > "$stage/SIGNING-STATUS.txt"
else
  [[ -n "${APPLE_ID:-}" && -n "${APPLE_APP_SPECIFIC_PASSWORD:-}" && -n "${APPLE_TEAM_ID:-}" ]] || { echo 'Developer ID distribution requires notarization credentials as well.' >&2; exit 1; }
  ditto -c -k --sequesterRsrc "$stage" "$work/notarization.zip"
  xcrun notarytool submit "$work/notarization.zip" --apple-id "$APPLE_ID" --password "$APPLE_APP_SPECIFIC_PASSWORD" --team-id "$APPLE_TEAM_ID" --wait
  for name in GeoCheats Dash; do xcrun stapler staple "$stage/$name.app"; xcrun stapler validate "$stage/$name.app"; spctl --assess --type execute --verbose=2 "$stage/$name.app"; done
  printf '%s\n' 'Developer ID signed and Apple notarized; app tickets stapled and Gatekeeper assessment passed.' > "$stage/SIGNING-STATUS.txt"
fi

python3 packaging/check-bundles.py "$stage"
ditto -c -k --sequesterRsrc "$stage" out/GeoCheats-macOS.zip
mkdir -p "$work/unpacked"
ditto -x -k out/GeoCheats-macOS.zip "$work/unpacked"
python3 packaging/check-bundles.py "$work/unpacked"
hdiutil create -volname GeoCheats -srcfolder "$stage" -format UDZO -ov out/GeoCheats-macOS.dmg
hdiutil verify out/GeoCheats-macOS.dmg
mkdir -p "$work/mounted"
hdiutil attach -readonly -nobrowse -mountpoint "$work/mounted" out/GeoCheats-macOS.dmg
for name in GeoCheats Dash; do codesign --verify --deep --strict --verbose=2 "$work/mounted/$name.app"; done
hdiutil detach "$work/mounted"
cp "$stage/SIGNING-STATUS.txt" out/
