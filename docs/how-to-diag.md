# Diagnostic playbooks (`/api/v0/diag`)

Read-only, per-alarm diagnostic playbooks run through Annet/gnetcli. Bots and
agents get **structured protobuf/JSON** (schema in `proto/diag/v1/diag.proto`),
never raw CLI text — although each step also carries `raw_output` so nothing is
lost.

## Endpoints

All under `/api/v0/diag` (Bearer auth, same token as the rest of the API).

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/diag/run` | Run one alarm's playbook against a device |
| GET | `/diag/playbooks` | List families, alarms, and command sets |
| GET | `/diag/schema` | The `.proto` schema text (for client codegen) |

### POST /diag/run

```json
{
  "host": "172.22.0.52",
  "alarm": "bgp",
  "peer": "10.132.128.122",
  "port": "TenGigE0/0/0/6",
  "target": "8.8.8.8",
  "location": "0/0/CPU0",
  "last_hop": "10.254.255.2",
  "vendor": "cisco",       // optional inventory override
  "platform": "iosxr",     // optional; distinguishes IOS vs IOS-XR
  "no_probes": false,       // availability: skip the MikroTik probe pings
  "timeout_s": 30
}
```

`host` + `alarm` are required. Placeholders `<PORT>/<PEER>/<CILJ>/<lok>` in the
playbook are filled from `port`/`peer`/`target`/`location`; a step whose
placeholder is unset is returned as `STEP_SKIPPED`.

Response is a `DiagnosticRun` (protojson by default; add `Accept:
application/protobuf` or `?format=pb` for binary). Each `Step` has a `status`,
`duration_ms`, `raw_output`, and a typed `parsed` payload when a parser matched
(facts, interfaces, neighbors, optics, routes, arp, mac, stp, env, resources,
logs, ping, redundancy, inventory, platform).

## Alarms & families

Alarms: `availability, port_optics, stp, ospf, bgp, bfd, isis, redundancy,
mpls_ldp, l2vpn, cpu_mem, hardware_env, restart_uptime` (+ `monitoring_gap`).

Families (from inventory vendor+platform): `iosxr, ios, eltex, routeros, fs`
(plus `ups`/no-SSH → LibreNMS and `generic_core` fallback). Documented gaps
(e.g. STP on IOS-XR, MPLS/L2VPN on Eltex/FS) come back in `gaps[]`.

Availability additionally runs ping + traceroute from the two MikroTik probes
(`192.168.224.5`, `95.140.112.230`) to the device, the last hop, and the core
anchors (`172.22.2.4`, `172.22.2.16`), within a 90s step budget.

## Regenerating the schema

Edit `proto/diag/v1/diag.proto`, then:

```bash
make proto   # regenerates internal/diag/diagpb + the embedded schema copy
go test ./internal/diag/
```

Parsers live in `internal/diag/parse_*.go` and are unit-tested against real
device fixtures in `internal/diag/testdata/`.
