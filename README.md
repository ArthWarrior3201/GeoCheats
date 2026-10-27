# GeoCheats v1

Source and GitHub Pages download page for GeoCheats and the **Dash** mod tool.

- `source-code-windows`, `source-code-macos`, `source-code-linux`: self-contained source trees. Shared Go code is mirrored so each folder builds independently.
- `mod_building_guide.txt`: package format and human/AI authoring instructions.
- `index.html`: download page using the supplied logo. Every Download click fetches GitHub's latest stable release afresh and displays a five-second loading dialog. Missing assets stay unavailable; older releases are never substituted.
- `.dash`: custom JSON recipe format; no Geode required.

## Compatibility

Windows GD 2.2091 invincibility was user-confirmed. Other modes remain experimental.
Linux gameplay uses the **Windows game under Proton/Wine**, subject to existing process-access permissions. Native macOS has an **experimental Intel invincibility discovery path**: link the installed game and GeoCheats looks for its exact named death-handler symbol and a supported entry prologue. No Windows addresses are reused. Stripped, universal, and native ARM64 game binaries are refused; the other six modes still require native Mac profiles. No real Mac gameplay has been verified.

## Remembered mods

Switches are saved per executable fingerprint and stay remembered until disabled. GeoCheats reconnects to the linked game automatically and restores remembered modes. Closing the browser leaves the controller running; reopening the app reuses that controller. On first enable, a removable **Start GeoCheats at login** entry is configured; the control window lets you disable it. Keep portable binaries or the Mac app in a stable location.

Quit restores live patches and stops the controller, retaining preferences for the next launch/login. Recovery checks recognized code patches and saved speed baselines after a controller crash. Preferences have an atomic backup. This does not promise survival of deleted app data, OS permission changes, game updates, or a stopped controller; field modes require the controller running. Linux gameplay and native Mac memory access still need real-device testing.

## Releases and Pages

The build workflow produces a portable Windows EXE, a macOS ZIP containing both CPU app builds, and a Linux ZIP containing DEB and RPM packages. It does not publish a release automatically. Upload the artifacts to your published stable GitHub Release, retaining the filenames `GeoCheats-Windows.exe`, `GeoCheats-macOS.zip`, and `GeoCheats-Linux.zip`.

In repository Settings → Pages, select **Deploy from a branch**, **main**, **/(root)**. The download page will work after GitHub Pages is enabled. Only stable published releases count as `/releases/latest`; prereleases and drafts are excluded.

No game executable or game bundle is included in this repository.
