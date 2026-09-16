# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

TDTech Toolkit — a Go/Bubbletea CLI (`tdtk`) for accessing this org's GCP and GKE resources: opening an IAP SOCKS5 bastion tunnel, switching between GKE cluster profiles, and running kubectl/k9s against them. `tdtk` takes no arguments or subcommands — running it always launches the interactive main menu, and every action (setup, GCP, GKE, doctor) is reached by picking through that menu.

## Running and testing

```bash
go run ./cmd/tdtk           # the only entrypoint: opens the interactive main menu
go build -o tdtk ./cmd/tdtk # build the binary (config/clusters.conf is resolved relative to it)
```

There is no test suite. Validate changes with `go build ./...` and `go vet ./...`.

## Architecture

- **`cmd/tdtk/main.go`** — entrypoint. `main()` just calls `runMenu()`; there is no argument parsing.
  - `runMenu()` loops showing `tui.NewMainMenu()` and dispatching on `tui.MenuResult` to `runSetup`/`runGCPMenu`/`runGKEMenu`/`runDoctor`/`usage`, returning to the menu after each until the user quits (`MenuNone`). `runGCPMenu`/`runGKEMenu` are their own loops over `tui.NewGCPMenu`/`tui.NewGKEMenu`.
  - `requireCmds` (gcloud/kubectl/kubectx/k9s, checked per-action) gates the specific menu action that needs the tool, not the menu itself — so the menu (and its own "Setup tools" option) is always reachable even with nothing installed yet.
  - `pause()` is the shared "press enter to continue" used after a menu action prints output, so the user sees it before the parent menu redraws.
- **`internal/logx`** — `Info`/`Warn`/`Die`. `Die` is the standard way to abort with a message (prints and exits non-zero).
- **`internal/gcpx`** (`bastion.go`) — gcloud helpers plus the bastion tunnel:
  - `Bastion(env)` opens a **foreground** SSH `-D` SOCKS5 tunnel through IAP to a bastion instance chosen per env by `BastionInstance` (`cjx-bastion` for prod, `tech-bastion` for nonprod; zone/port are the `BastionZone`/`BastionSocksPort` constants), mapped to project `tech-svc-prod`/`tech-svc-nonprod` via `BastionProject`.
  - `EnsureBastion(env)` is the **background** counterpart used by `gkex` and by the GCP menu's "Run bastion (background)" action: checks if port 1080 is already open, otherwise starts the tunnel detached + a PID file under `StateDir()` (`~/.cache/tdtk`, override with `TDTK_STATE_DIR`), then polls until the port accepts connections (30s timeout).
  - `RememberBastionEnv` persists the last-used env to the state dir (informational; nothing currently reads it back).
- **`internal/gkex`** (`gke.go`) — kubectl/kubectx helpers, driven by `config/clusters.conf` via `internal/config`:
  - `Use(profile)` looks up the profile, runs `gcloud container clusters get-credentials`, then renames the resulting raw kubectl context to the profile name via `kubectx` (so profile names, not raw GKE context strings, are what users interact with).
  - `ListNamespaces()` / `ListServices(ns)` shell out to `kubectl get namespaces|svc -o jsonpath=...` to drive the port-forward wizard's live namespace/service pickers (as opposed to the free-text prompts this replaced).
  - `PortForwardExec(ns, svc, portMap)` runs `kubectl port-forward -n <ns> svc/<svc> <local:remote>`; the full interactive flow (pick profile → namespace → service → port) is orchestrated by `runGKEPortForwardWizard` in `main.go`, chaining `tui.NewProfileSelect` → `gkex.ListNamespaces`/`tui.NewStringSelect` → `gkex.ListServices`/`tui.NewStringSelect` → `tui.NewPortMapInput`.
  - **`EnsureBastionForProfile(profile)` resolves the bastion env from the profile name (`config.BastionEnv`), calls `gcpx.EnsureBastion`, and sets `HTTPS_PROXY=socks5://localhost:<port>` in the process env** — this is load-bearing for reaching private GKE control planes and must stay wired up when touching this file. If no env can be resolved (profile name doesn't start with `prod`/`nonprod`), it warns and proceeds without a tunnel rather than failing. `main.go`'s `switchToProfile` calls it then `Use(profile)`; `ensureProxyForProfile` calls it again right before handing off to k9s/`PortForwardExec`, as a cheap re-affirming checkpoint (`gcpx.EnsureBastion` no-ops once the tunnel port is already open).
- **`internal/config`** (`clusters.go`) — loads and validates `config/clusters.conf`. Format: `profile:project:location_type:location:cluster` (`location_type` is `zone` or `region`). Comments (`#`) and blank lines are skipped by `Lines()`. **The bastion env (`prod`/`nonprod`) is a property of the cluster *profile*, not the GCP project** — it's derived purely from a naming convention (`BastionEnv`: profile name must start with `prod` or `nonprod`, e.g. `prod-a`, `nonprod-reporting`). This lets one env span multiple cluster profiles and even multiple GCP projects; there is no reverse-mapping from project id to env. `File()` resolves the config path from `TDTK_CLUSTERS_FILE`, or `config/clusters.conf` next to the running binary.
- **`internal/doctorx`** (`doctor.go`) — `CheckTools`/`CheckClustersConfig` check required tools are on PATH (`gcloud`, `kubectl`, `kubectx`, `k9s`, with brew install hints from `InstallHint`) and validate `config/clusters.conf` structurally (5 fields per line, valid `location_type`, profile name starts with `prod`/`nonprod`). Rendered by the `doctor` TUI (all green) and reused by the `setup` TUI (missing tools only, with an offer to `brew install` them).
- **`internal/tui`** — one Bubbletea model per screen, each run via its own `tea.NewProgram(...).Run()` call from `main.go` (not one composed nested model); a screen's result is read back with a `XxxResult(finalModel)` accessor after the program exits:
  - `menu.go` — `choiceMenuModel`, a single list-based picker shared by `NewMainMenu`/`NewGCPMenu`/`NewGKEMenu` (they differ only in title/items); `MenuResult` reads back the picked `MenuChoice` (`MenuNone` if the user pressed `q`/`esc`).
  - `setup.go` — `NewSetupModel`/`SetupResult`: spinner while `doctorx.CheckTools` runs, then a status list; `i` requests installing missing tools (`main.go` then shells out to `sh -c` each `doctorx.InstallHint`).
  - `doctor.go` — `NewDoctorModel`/`DoctorOK`: same shape as setup but also validates `config/clusters.conf` via `doctorx.CheckClustersConfig`.
  - `profileselect.go` / `select.go` — `NewProfileSelect` (cluster profiles, with project/location/cluster as the description) and the generic `NewStringSelect` (plain string lists: GCP projects, bastion env, namespaces, services).
  - `portforward.go` — `NewPortMapInput`/`PortMapResult`: the last step of the wizard, a single `local:remote` text input validated against `portMapRe`.
- **`config/clusters.conf`** — the only per-user/per-org data file; real cluster profiles are filled in here (currently only commented examples are committed).

## Conventions

- Packages are suffixed `x` where they'd otherwise collide with stdlib/import names (`gcpx`, `gkex`, `doctorx`, `logx`).
- Fail loudly via `logx.Die("message")` rather than swallowing errors; every other function returns an `error` for the caller to handle or propagate. Menu-driven flows in `main.go` prefer `logx.Warn` + `pause()` over `Die` for recoverable failures, so a mistake drops the user back at the parent menu instead of exiting the whole program.
- New `internal/*` packages should follow the existing import direction: `gkex` depends on `config` and `gcpx`; `doctorx` depends on `config`; `tui` depends on `config` and `doctorx` (for rendering) but never on `gcpx`/`gkex` — actions live in `main.go`, not in the TUI models. Nothing in `internal/*` depends on `cmd/tdtk`.
- Every `internal/tui` screen is a standalone Bubbletea model run through its own `tea.NewProgram`, with plain function calls in `main.go` gluing screens together in sequence — there is no single top-level Bubbletea app composing all screens as nested states.
