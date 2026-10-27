# GeoCheats v1

Standalone local mod controller and **Dash** authoring CLI. No Geode, Scoop,
Python, Node, or separately installed language runtime is required by users.
The app opens its controls in the installed browser on a private loopback URL.

Link a game while it is closed; connect separately after starting it. Paths
and Steam discovery work with the current account rather than a fixed username.

Seven recipes are included: Invincibility, High Jump, Super Fast, Super Slow,
No Jump, Fly, and Instant Complete (C+G+D). Recipes target the exact Windows
GD 2.2091 fingerprint documented in `mod_building_guide.txt`. Invincibility
was user-confirmed; the new modes still require gameplay testing.

Windows and Linux/Proton backends are implemented. macOS applications and
inspection tools build, with an experimental Intel Mach memory backend, but
native Intel invincibility can be discovered from the exact installed game's
named death-handler symbol and supported entry prologue. Missing symbols,
unsupported prologues, and universal games are refused. This is experimental;
the other six native Mac modes still need separately verified profiles.
Native ARM64 code patching is explicitly blocked. Do not interpret a Mac app
build as verified Mac gameplay support.

Switches stay remembered per executable fingerprint until disabled. The controller
reconnects and reapplies them when the linked game starts. Closing the browser
keeps it running; launching GeoCheats again opens that same controller. On first
enable, a removable login item is added. Disable it with Start GeoCheats at login.
Keep the binary/app at a stable path. Quit restores runtime patches but retains
saved switches for the next launch/login. Saved settings have an atomic backup;
recognized code patches and speed baselines can recover after controller crashes.
Deleted settings, changed permissions, game updates and a stopped controller
cannot be made equivalent to a running mod. No system driver is installed.

Read **mod_building_guide.txt** for all commands, schema, exact compatibility,
known limitations and the human/AI development handoff. The source is ready
to upload to a GitHub repository; no game executable is included.

Build: `go test ./...`, then `go build ./cmd/geocheats` and `go build ./cmd/dash`.
The pinned Go build dependency is purego (BSD license), compiled into relevant
binaries. OS libraries, a browser and OS process-access permissions still apply.
