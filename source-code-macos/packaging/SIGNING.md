# Mac package integrity and Apple trust

`build-macos.sh` must run on macOS. It creates universal Intel/Apple Silicon apps,
adds ICNS resources and bundle metadata, signs nested CLI code first and seals
each completed app. It validates signatures, icon decoding, both CPU slices,
real LaunchServices startup and rendered native controls and app-icon checks. It repeats
app checks after extraction and verifies/mounts the final DMG.

By default the build uses an ad-hoc identity. This is a valid local integrity
signature, **not a Developer ID identity or Apple notarization**. A successful
native window launch does not prove that a downloaded quarantined app will satisfy
Gatekeeper on another computer. The build does not remove quarantine, disable
Gatekeeper/SIP, or alter the game's signature. A DMG is packaging, not a trust
bypass. Check SIGNING-STATUS.txt in the actual downloaded package.

For Developer ID distribution, configure these repository Actions secrets:

- MACOS_CERTIFICATE_P12_BASE64: base64 of an exported Developer ID Application
  certificate including its private key.
- MACOS_CERTIFICATE_PASSWORD: the P12 export password.
- APPLE_ID, APPLE_APP_SPECIFIC_PASSWORD, APPLE_TEAM_ID: notarization credentials.

Keep these in GitHub Secrets; do not commit or send private signing material in
chat. The workflow imports the identity into a temporary keychain, signs with
Hardened Runtime, submits to Apple's notary service, staples app tickets and
requires a successful Gatekeeper assessment. If a certificate is supplied but
notarization credentials are missing, it fails instead of silently publishing
an unnotarized Developer ID package. That optional credentialed path requires
the repository owner's Apple Developer credentials; it is not exercised by an
ad-hoc build.

For a locally reported launch failure, collect diagnostics on the extracted app:

```sh
codesign --verify --deep --strict --verbose=4 /Applications/GeoCheats.app
codesign -d --verbose=4 /Applications/GeoCheats.app
spctl --assess --type execute --verbose=4 /Applications/GeoCheats.app
/Applications/GeoCheats.app/Contents/MacOS/GeoCheats
```

These distinguish bundle corruption, signature problems, trust rejection and
runtime errors. Node action deprecation/cache warnings are separate CI concerns.
GitHub's ARM runner capacity notice is infrastructure scheduling information.

Primary references:
https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution
https://developer.apple.com/documentation/security/resolving-common-notarization-issues
https://support.apple.com/guide/security/sec3ad8e6e53/web
