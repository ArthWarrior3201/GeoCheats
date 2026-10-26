# GeoCheats v1

Source and GitHub Pages download page for GeoCheats and the **Dash** mod tool.

- `source-code-windows`, `source-code-macos`, `source-code-linux`: self-contained source trees. Shared Go code is mirrored so each folder builds independently.
- `mod_building_guide.txt`: package format and human/AI authoring instructions.
- `index.html`: download page using the supplied logo. Every Download click fetches GitHub's latest stable release afresh and displays a five-second loading dialog. Missing assets stay unavailable; older releases are never substituted.
- `.dash`: custom JSON recipe format; no Geode required.

## Compatibility

Windows GD 2.2091 invincibility was user-confirmed. Other modes remain experimental.
Linux gameplay uses the **Windows game under Proton/Wine**, subject to existing process-access permissions. Native macOS game recipes are **not complete**: the supplied Mac game ZIPs failed transfer and could not be analyzed. The Mac app/backend source is provided but this is not verified Mac gameplay support. Native ARM64 code patching is blocked.

## Releases and Pages

The build workflow produces a portable Windows EXE, a macOS ZIP containing both CPU app builds, and a Linux ZIP containing DEB and RPM packages. It does not publish a release automatically. Upload the artifacts to your published stable GitHub Release, retaining the filenames `GeoCheats-Windows.exe`, `GeoCheats-macOS.zip`, and `GeoCheats-Linux.zip`.

In repository Settings → Pages, select **Deploy from a branch**, **main**, **/(root)**. The download page will work after GitHub Pages is enabled. Only stable published releases count as `/releases/latest`; prereleases and drafts are excluded.

No game executable or game bundle is included in this repository.
