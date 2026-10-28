# GeoCheats v1

Native desktop Geometry Dash mod controller and Dash mod builder for Windows,
macOS and Linux. The apps render their own windows and use OS file choosers;
no browser, web page, WebView, or local web server provides the controls.

On launch, a valid saved link is reused. Otherwise the apps search Steam's
standard installation, manifest and extra libraries. If Geometry Dash is found,
its executable is linked automatically. If it is missing, click **Choose file**
and select GeometryDash.exe (Windows/Proton) or Geometry Dash.app /
GeometryDash.app (macOS). Once linked, the file prompt disappears.

Compatible mod switches can be enabled while the game is closed. Their state
is saved and applied automatically when the matching game starts. Closing the
GeoCheats window hides it and keeps the controller running; reopen it from the
tray/menu-bar icon or by launching the app again. Quit restores runtime changes
and keeps saved preferences for next time.

Dash has a small form: name, ID, address, expected bytes, replacement bytes,
and **Create .dash**. Native open/save dialogs handle files. Advanced JSON is
optional. Dash provides the authoring controls directly in its app.

## Compatibility

Windows GD 2.2091 invincibility was user-confirmed. The other six built-in modes
remain experimental. Linux uses the Windows game under Proton/Wine and the
user's existing process-access permissions. Native macOS linking is independent
of mod compatibility: experimental Intel invincibility requires a supported
thin Mach-O symbol/profile. Native ARM64/universal game patches and the other
Mac modes remain unsupported. An app launch/build test is not a gameplay test.

## Downloads and build

The Mac ZIP has exactly two top-level bundles: **GeoCheats.app** and **Dash.app**.
Documentation, signing status and icons are inside the bundles.
A separate DMG contains those apps and an Applications shortcut. Builds without
Developer ID credentials are ad-hoc signed, not Apple-notarized.

Source folders are self-contained. Build scripts and the GitHub Actions workflow
compile native GUIs using Go 1.24+, Fyne 2.8.2, CGO and OS graphics libraries.
Windows uses MinGW-w64, macOS uses native AppKit/Cocoa with Xcode command-line
tools, and Linux uses GL/X11/Wayland headers. The Mac front end does not use
OpenGL or a browser renderer.
Linux packages declare their OS graphics and Zenity file-dialog dependencies.
No Geode, Python, Node, Go installation, or separate GUI runtime is required by
end users. See mod_building_guide.txt and packaging scripts for exact commands.

The GitHub Pages site remains a separate download page. Every Download click
fetches GitHub's latest published stable release, with the five-second popup.
CI builds artifacts; publishing releases and enabling Pages remain manual.
No proprietary game executable is distributed here.
