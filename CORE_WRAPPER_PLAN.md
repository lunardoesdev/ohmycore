# Core Wrapper for Multiple Xray/Mihomo Instances

## Context

Build a Go library (module `ohmycore`) plus a CLI for multiple Xray instances and one process-global Mihomo instance, each with managed config and lifecycle. Expose a public Go API, plus an optional C JSON ABI for foreign-language callers; Android is later. The C ABI and gomobile binding are separate integration surfaces. Linux is the first target.

## Approach

Ordered steps; each must leave `go build ./...` and `go test ./...` passing. Make small, focused commits as often as practical: commit each coherent change after its relevant checks pass instead of accumulating work for one large final commit; keep unrelated changes out of each commit.

1. **Module bootstrap.** Create Go module `ohmycore` and pin resolved dependencies in `go.mod`/`go.sum`. Layout: `core/` (public API and manager), `internal/xray/`, `internal/mihomo/`, `internal/links/`, `cmd/ohmycore/`, `cmd/ohmycore-cshared/`, `export/` (JSON dispatcher), `testdata/`. Linux first; no Android build yet.

2. **Public types (`core/types.go`).** Define `Kernel`, `Status`, `Instance` metadata (`ID`, `Kernel`, status/timestamps, `WorkDir`, `LastError`) and a normalized proxy `Node` with protocol-specific options preserved. Keep credentials/raw config out of public instance listings. Conversion must not silently discard fields.

3. **Manager and lifecycle (`core/manager.go`).** `core.Manager` is the single public implementation and owns synchronization, instance records, config files, and adapters; do not duplicate it in `internal/manager` (avoids an import cycle and split state). Implement `Create`, `Get`, `List`, `Start`, `Stop`, `Reload`, `Remove`, `Close`; write config atomically with restrictive permissions. `Reload` semantics are adapter-specific: Xray stop/start; Mihomo in-process apply. Define typed errors (`ErrNotFound`, `ErrAlreadyRunning`, `ErrNotRunning`, `ErrMihomoBusy`, `ErrProxyInUse`) and serialize mutating operations.

4. **Xray adapter (`internal/xray`).** Manage multiple `*xraycore.Instance` handles keyed by wrapper instance ID. Start/stop/reload and version. `Ping`/`SpeedTest` use a temporary config/instance for the requested outbound and always close it; they never change the running Xray instance. Verify the temporary config includes required DNS/routing settings and cleans up on errors/cancellation.

5. **Mihomo adapter (`internal/mihomo`).** Mihomo is a process-global singleton: allow one running Mihomo core in this process, regardless of how many wrapper records exist. Reserve/retry a loopback controller port and generate a private secret; pass both with `hub.WithExternalController`/`hub.WithSecret` so config values cannot expose or collide with the controller. Start one core and keep it alive while its proxy list changes. Do not solve additional proxy nodes by starting another Mihomo process/core.

6. **Live Mihomo proxy/config management.**
   - Keep one authoritative YAML config per instance. Reserve `ohmycore-managed-provider` and `ohmycore-managed-group`; fail before mutation if either name already exists. Namespace imported nodes by subscription source ID; sync/remove touches only that source. New nodes appear in the managed group but are never auto-selected or inserted into user routing groups. Reject removal of a selected node unless the caller supplies an explicit replacement; never silently fall back to `DIRECT`.
   - Validate a candidate via `executor.ParseWithBytes`, persist it atomically, then apply with `executor.ApplyConfig(candidate, false)` to the existing Mihomo process. Never spawn or stop/start the core for node changes. Add an integration test that keeps a long-lived flow through an unrelated existing proxy while add/update/remove runs; assert the flow completes, existing listeners stay open, the active selector is unchanged, and the same core remains running. If that fails, revisit the update mechanism instead of claiming uninterrupted updates.
   - Do not use `PATCH /configs` for proxy CRUD: it patches a limited set of general settings. Provider `PUT` refreshes an existing named provider; it does not create/delete arbitrary proxies. A helper to refresh all providers must enumerate names and call each named endpoint.
   - `Delay` uses `/proxies/<name>/delay` without changing selection.
   - `SpeedTest(ctx, inst, proxyName, options)` uses a temporary runtime-only HTTP inbound pinned to `proxyName`. Parse the manager's canonical config with `executor.ParseWithBytes`, build the temporary listener with `listener.ParseListener` (`type: http`, `listen: 127.0.0.1`, unique port/name, `proxy: proxyName`), and add it to the canonical listener map with `listener.PatchInboundListeners(..., tunnel.Tunnel, true)`. Send the download via that local HTTP proxy and restore the original listener map on every exit path. Do not call full `executor.ApplyConfig` or persist the test listener. Require a caller context/deadline and `maxBytes`; stop at the first limit, allow only one Mihomo speedtest at a time, and hold a read/use lease so the target cannot be removed mid-test. Confirm readiness/retry port collisions and test concurrent ordinary traffic. The test still consumes shared bandwidth; never run it automatically.
   - Mihomo has no controller endpoint for per-proxy throughput; the temporary pinned listener is the planned workaround.
   - `PatchConfig` is limited to Mihomo's supported runtime general settings.

7. **Share links and subscriptions (`internal/links`, public wrappers in `core`).** Parse common schemes (`vless://`, `vmess://`, `trojan://`, `ss://`, `socks://`, `http://`, `ssr://`, `hysteria2://`/`hy2://`, `tuic://`, `anytls://`, `wireguard://`) into a protocol-preserving representation. Expose independent `CheckXrayLinkSupport(link) (SupportResult, error)` and `CheckMihomoLinkSupport(link) (SupportResult, error)`; each dry-runs only that backend's converter/parser and reports unsupported options. These are local compatibility checks, not a network/reachability test. Expose separate `LinkToXrayConfig(link)` and `LinkToMihomoConfig(link)` converters that consume the original link and produce that backend's native config. Also expose separate per-backend subscription checks/converters. Never translate an Xray config into Mihomo config or vice versa; never infer one backend's support from the other's or drop unsupported fields. Use libXray's share parser as an Xray-oriented reference, not proof of Mihomo compatibility. Parse plain-text/base64 link lists and Clash/Mihomo YAML; fetch HTTP(S) with context, timeout, response-size limit, and configurable User-Agent (no rotation). Validate a complete live Mihomo update before applying it.

8. **CLI (`cmd/ohmycore`).** Add lifecycle/status commands; Xray ping/speedtest; Mihomo proxies, delay, speedtest, select, add/update/remove; import/refresh/remove subscription on a running Mihomo instance; `link check --kernel xray|mihomo|both`; separate `link convert --kernel xray` and `link convert --kernel mihomo`; equivalent per-backend subscription check/convert commands. Mihomo speedtest requires a deadline and `--max-bytes`. Global `--base-dir`; never print secrets/config contents.

9. **C ABI (`cmd/ohmycore-cshared` + `export`).** Preserve libXray's `apiVersion: 3` request/response envelope and `CGoInvoke`/`CGoFree` ownership contract, but treat our method names/payloads as our own API unless explicitly matched. Expose separate support-check and conversion methods for Xray and Mihomo links and subscriptions. Build a `package main` shared library with `-buildmode=c-shared`; a normal `go build ./export` does not prove exported C symbols. Keep a separate gomobile-friendly Go facade for Android; link all Go components into one native artifact/runtime.

10. **Tests and smoke.** Unit tests for manager persistence/rollback and backend-specific link conversions; table-test Xray and Mihomo support checks independently, including links supported by only one, unsupported options, malformed input, and proof that no cross-config converter is exposed. Mihomo integration tests cover live proxy mutations with a long-lived flow using another proxy, unchanged selector/listeners, and invalid-config rollback. Speedtest tests verify forced target routing, unchanged selection, cleanup on success/error/cancel, duration/byte caps, and ordinary traffic continuing during the test. Use local endpoints/fixtures; unsupported nodes must never be silently omitted.

11. **Android compatibility constraint (no Android work now).** Keep shared code platform-neutral: caller-provided paths, no desktop-only APIs in `core`, and future socket-protection/TUN behavior behind a narrow adapter boundary. Do not add Android code, builds, fork integration, or platform tests in this phase. Goal: a later thin Android binding/build layer without substantive changes to core logic; verify that claim only when Android work is authorized.

## Critical files & anchors

- `core/types.go` and `core/manager.go` — public API, lifecycle, persisted instance state.
- `internal/xray/adapter.go` — Xray start/stop and isolated probes.
- `internal/mihomo/adapter.go` — singleton lifecycle, in-process config apply, controller access.
- `internal/links/` — parsers, per-kernel converters, subscription reconciliation.
- `export/` and `cmd/ohmycore-cshared/` — JSON dispatcher and C ABI entry point.
- `cmd/ohmycore/` — CLI.

## Verification

- `go build ./...`, `go vet ./...`, and `go test ./...` pass; unit tests are offline and deterministic.
- Xray integration covers start/stop and verifies probe cleanup without changing the running instance.
- Mihomo integration keeps a long-lived local flow through proxy A while adding/updating/removing proxy B; it must complete without selector changes, listener closure, or core restart. Also verify invalid-config rollback and explicit replacement/rejection for selected-node removal.
- Smoke uses local endpoints and exercises both kernels; no public internet dependency.
- C ABI verification builds a shared library with `-buildmode=c-shared`, inspects `CGoInvoke`/`CGoFree`, and calls/frees a response from a small C consumer. Do not treat `go build ./export` as proof of exported symbols.
- Android is excluded from current verification; only platform-neutral boundaries are maintained now.

## Assumptions & contingencies

- Mihomo's process-global state allows one active core per process. Additional Mihomo proxy nodes are managed inside that core; a second core process is not the mechanism for scaling the proxy list.
- Xray supports multiple in-process `core.Instance` handles; verify this with lifecycle integration tests for the pinned dependency.
- Xray configs are JSON; Mihomo configs are YAML. Persist the exact accepted config and apply candidate updates atomically; do not expose secrets through instance listings or CLI output.
- Mihomo runtime updates use parsed in-process config application with `force=false`, not `PATCH /configs` for proxies. The core process is not restarted, but any stronger no-interruption guarantee must be proven by integration tests; traffic already using a proxy being removed may fail.
- The Mihomo controller binds only to loopback with a generated secret; port selection retries if another process wins the bind race.
- Probe contract: Xray probes use temporary instances; Mihomo delay targets one proxy without switching; Mihomo SpeedTest pins a temporary loopback HTTP inbound to the requested proxy, obeys context/deadline and byte cap, then removes it without changing selection.
- Android remains out of scope until the build/fork constraints are verified; keep the public Go API, cgo ABI, and gomobile facade distinct.
