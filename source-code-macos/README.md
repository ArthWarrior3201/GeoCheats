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
native Mac game recipes are unavailable pending a native game executable.
Native ARM64 code patching is explicitly blocked. Do not interpret a Mac app
build as verified Mac gameplay support.

Read **mod_building_guide.txt** for all commands, schema, exact compatibility,
known limitations and the human/AI development handoff. The source is ready
to upload to a GitHub repository; no game executable is included.

Build: `go test ./...`, then `go build ./cmd/geocheats` and `go build ./cmd/dash`.
The pinned Go build dependency is purego (BSD license), compiled into relevant
binaries. OS libraries, a browser and OS process-access permissions still apply.
