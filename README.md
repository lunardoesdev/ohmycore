# ohmycore

`ohmycore` is a Go library and CLI for managing multiple proxy core instances
(Xray in-process, Mihomo process-global singleton) with managed configs,
lifecycle, and runtime node management. It also provides share-link and
subscription parsing with per-backend support checks and config generation,
plus a C JSON ABI (mirroring libXray's envelope) for foreign-language callers.

Linux is the first target. Android readiness is a design constraint, not a
separate build yet.

## Features

- Multiple Xray `core.Instance` handles in one process.
- One Mihomo core per process; proxy nodes are managed inside it without
  restarting the core.
- Atomic config persistence with restrictive permissions.
- Xray isolated probes (`Ping`/`SpeedTest`) via temporary instances.
- Mihomo `Delay` without changing selection; `SpeedTest` via a temporary
  loopback HTTP inbound pinned to the requested proxy.
- Runtime Mihomo proxy add/update/remove, selection, provider reload, and
  config patch without core restart.
- Share-link and subscription parsing with independent per-backend support
  checks and native config generation.
- C JSON ABI (`CGoInvoke`/`CGoFree`) for FFI consumers.
- CLI for lifecycle, status, and link operations.

## Requirements

- Go 1.27+
- Linux (first target)
- For cgo builds: a C compiler (gcc/clang)

## Build

```bash
go build ./...
go vet ./...
go test ./...
```

C shared library:

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o libohmycore.so ./cmd/ohmycore-cshared
nm -D libohmycore.so | grep -E 'CGoInvoke|CGoFree'
```

CLI:

```bash
go build -o ohmycore ./cmd/ohmycore
```

## Quick Start

Create and start an Xray instance:

```bash
ohmycore --base-dir /tmp/ohmycore instance create --kernel xray --config ./testdata/xray.json
# prints instance ID
ohmycore --base-dir /tmp/ohmycore instance start <id>
ohmycore --base-dir /tmp/ohmycore instance status <id>
ohmycore --base-dir /tmp/ohmycore instance stop <id>
```

Smoke test:

```bash
bash scripts/smoke.sh
```

## Architecture

```
core/              public API and Manager
internal/xray/     Xray adapter (multi-instance)
internal/mihomo/    Mihomo adapter (singleton + runtime management)
internal/links/     share-link and subscription parsing/conversion
cmd/ohmycore/       CLI
cmd/ohmycore-cshared/ C shared library entry point
export/            JSON dispatcher and cgo exports
testdata/          fixtures
```

### Manager

`core.Manager` is the single public implementation. It owns instance records,
config files, synchronization, and adapters. Adapters are registered per kernel:

```go
m := core.NewManager(baseDir)
m.RegisterAdapter(core.KernelXray, xray.New())
m.RegisterAdapter(core.KernelMihomo, mihomo.New())
```

Persistence: each instance has `baseDir/instances/<id>/config.json` (Xray)
or `config.yaml` (Mihomo), plus a `status` file. Instances are loaded from
disk on `NewManager`.

All methods accept `{"apiVersion":3,"method":"...","payload":{...}}`.

| Method | Payload fields | Response `data` |
|---|---|---|
| `createInstance` | `kernel` (xray/mihomo), `config` (string) | `{id, kernel, status, work_dir, ...}` |
| `startInstance` | `id` | `{}` |
| `stopInstance` | `id` | `{}` |
| `removeInstance` | `id` | `{}` |
| `listInstances` | (none) | `[{id, kernel, status, ...}, ...]` |
| `getInstance` | `id` | `{id, kernel, status, ...}` |
| `reloadConfig` | `id`, `config` | `{}` |
| `xrayPing` | `config`, `outboundTag`, `url`, `timeout` (ms) | `{ms}` |
| `xraySpeedTest` | `config`, `outboundTag`, `downloadUrl`, `duration` (ms) | `{bytesPerSec}` |
| `mihomoProxies` | `id` | `{proxies: {...}}` |
| `mihomoDelay` | `id`, `proxy`, `url`, `timeout` (ms) | `{delay}` |
| `mihomoSpeedTest` | `id`, `proxy`, `downloadUrl`, `maxBytes`, `duration` (ms) | `{bytesPerSec}` |
| `mihomoSelect` | `id`, `group`, `proxy` | `{}` |
| `mihomoAddProxy` | `id`, `proxy` (object) | `{}` |
| `mihomoRemoveProxy` | `id`, `name` | `{}` |
| `mihomoUpdateProxy` | `id`, `name`, `proxy` (object) | `{}` |
| `mihomoPatchConfig` | `id`, `patch` (object) | `{}` |
| `mihomoReloadProviders` | `id` | `{}` |
| `checkXrayLinkSupport` | `link` | `{supported, scheme, reason?}` |
| `checkMihomoLinkSupport` | `link` | `{supported, scheme, reason?}` |
| `linkToXrayConfig` | `link` | config JSON string |
| `linkToMihomoConfig` | `link` | config YAML string |
| `version` | (none) | `{xray, mihomo}` |

Notes:
- `createInstance` stores config bytes; `startInstance` reads from disk.
- `mihomoSpeedTest` requires `maxBytes` and `duration`; it uses a temporary
  loopback HTTP inbound pinned to `proxy` and does not change selection.
- `check*LinkSupport` is a local compatibility check, not a reachability test.
- `linkTo*Config` converts a single link; subscriptions are handled by CLI.
### Mihomo Runtime Management

Mihomo keeps process-global state, so only one core runs per process. Additional
proxy nodes are managed inside that core:

- Imported nodes go into a reserved managed provider/group
  (`ohmycore-managed-provider` / `ohmycore-managed-group`).
- Additions never change the active selection.
- Removal of a selected node requires an explicit replacement; otherwise it is
  rejected (no silent fallback to `DIRECT`).
- Config updates use `executor.ParseWithBytes` + `executor.ApplyConfig(candidate, false)`.
- `PATCH /configs` is not used for proxy CRUD (it only patches general settings).
- `Delay` uses `/proxies/<name>/delay` without switching.
- `SpeedTest` creates a temporary loopback HTTP inbound pinned to the requested
  proxy, measures throughput, then removes it. Context/deadline and `maxBytes`
  are required; only one Mihomo speedtest runs at a time.

### Links and Subscriptions

Supported schemes: `vless://`, `vmess://`, `trojan://`, `ss://`, `socks://`,
`http://`, `ssr://`, `hysteria2://`/`hy2://`, `tuic://`, `anytls://`,
`wireguard://`.

Per-backend support is independent:

```go
rx, _ := links.CheckXrayLinkSupport(link)
rm, _ := links.CheckMihomoLinkSupport(link)
// rx.Supported and rm.Supported are independent
```

Conversion is separate and produces the backend's native config:

```go
xcfg, _ := links.LinkToXrayConfig(link)     // Xray JSON
mcfg, _ := links.LinkToMihomoConfig(link)   // Mihomo YAML
```

There is no Xray↔Mihomo config translator. Unsupported options are reported,
not silently dropped.

Subscriptions: plain-text/base64 link lists and Clash/Mihomo YAML proxy lists.
Fetch over HTTP(S) with context, timeout, and response-size limits.

CLI:

```bash
ohmycore link check <link> --kernel xray|mihomo|both
ohmycore link convert <link> --kernel xray
ohmycore link convert <link> --kernel mihomo
```

## C ABI

The C entry point mirrors libXray's envelope:

```c
char* CGoInvoke(char* requestJSON);
void  CGoFree(char* value);
```

Request:

```json
{"apiVersion":3,"method":"...","payload":{...}}
```

Response:

```json
{"success":bool,"data":any,"error":string}
```

Caller must free the returned string with `CGoFree`.

Available methods include: `createInstance`, `startInstance`, `stopInstance`,
`removeInstance`, `listInstances`, `getInstance`, `reloadConfig`, `xrayPing`,
`xraySpeedTest`, `mihomoProxies`, `mihomoDelay`, `mihomoSpeedTest`,
`mihomoSelect`, `mihomoPatchConfig`, `mihomoReloadProviders`,
`checkXrayLinkSupport`, `checkMihomoLinkSupport`, `linkToXrayConfig`,
`linkToMihomoConfig`, `version`.

Example:

```c
char* req = "{\"apiVersion\":3,\"method\":\"version\",\"payload\":{}}";
char* res = CGoInvoke(req);
printf("%s\n", res);
CGoFree(res);
```

## Android Compatibility

Android is not implemented in this phase. The code keeps platform-neutral
boundaries (caller-provided paths, narrow adapter interfaces) so a later thin
binding/build layer can be added without substantive changes to core logic.

## Testing

Unit tests are offline and deterministic:

```bash
go test ./...
```

Integration tests (require real cores, may be slow):

```bash
go test -tags=integration ./internal/xray/... ./internal/mihomo/...
```

## Verification Commands

```bash
go build ./...
go vet ./...
go test ./...
CGO_ENABLED=1 go build -buildmode=c-shared -o /tmp/libohmycore.so ./cmd/ohmycore-cshared
nm -D /tmp/libohmycore.so | grep -E 'CGoInvoke|CGoFree'
bash scripts/smoke.sh
```

## License

See LICENSE. Dependencies (xray-core, mihomo) retain their own licenses.
