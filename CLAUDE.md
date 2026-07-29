# CLAUDE.md

Guidance for working in this repository.

## What this is

**Annet Oil** is a Go wrapper that orchestrates multiple [annet](https://github.com/annetutil/annet)
containers. It exposes annet operations (`gen`, `diff`, `patch`, `deploy`) plus device
diagnostics through a **CLI** and a **REST API**, routing each command to the right annet
container based on hostname. An accompanying **Node/TypeScript MCP server**
(`mcp-annet-oil/`) surfaces the REST API as tools for AI agents.

```
AI agent ──MCP──> mcp-annet-oil (Node) ──HTTP──> Annet Oil API (Go, :8080)
                                                       │
CLI (annet-oil) ──────────────────────────────────────┤
                                                       ├─ Docker exec ─> annet containers
                                                       ├─ gNETcli ─────> device show/exec
                                                       └─ direct TCP/SSH > device check
```

Annet containers are driven via the **Docker Exec API** (not an SDK integration) — see the
"Architecture Decision" section of `README.md` for the rationale.

## Layout

- `cmd/annet-oil/` — main entrypoint (wires signal handling → `internal/cli`).
- `internal/cli/` — Cobra commands (`root.go`, plus one file per command: `gen`, `diff`,
  `patch`, `deploy`, `check`, `containers`, `routing`, `server`). `root.go` holds the shared
  service wiring; lightweight commands (e.g. `check`) override `PersistentPreRunE` to skip
  Docker.
- `internal/api/` — REST server. `server.go` mounts handlers under `/api/v0`; one handler per
  file in `handlers/`; auth/cache/logging in `middleware/`.
- `internal/annet/` — `Service` that builds and runs annet commands inside containers.
- `internal/container/` — Docker container manager. `internal/router/` — hostname→container routing.
- `internal/gnetcli/` — client for running show/diagnostic commands on devices.
- `internal/check/` — device availability checks (TCP port probe + SSH login), with batch runner.
- `internal/inventory/` — device inventory model + loader (`Device`, `Credentials`, lookups, filters).
- `internal/config/`, `internal/auth/` (RBAC), `internal/cache/`, `internal/logging/` (incl. S3 upload).
- `mcp-annet-oil/src/` — MCP server. `index.ts` declares every tool + dispatches calls;
  `client.ts` is the typed HTTP client; `command-whitelist.ts` gates `annet_execute`.
- `resources/` — example + live inventory / groups YAML. `configs/` — systemd, ssh, deploy configs.

## Build, run, test

Everything goes through the `Makefile` (`make help` lists all targets):

- `make build` — build API (Go) + MCP (Docker). `make build-api` / `make build-mcp` individually.
- `make run-api` — run the API on the host (needs `.env`; `make setup` creates it from `.env.example`).
- `make test` / `make test-coverage` — Go tests. `make lint` / `make format` / `make check` — gofmt/vet.
- Direct Go: `go build ./cmd/annet-oil`, `go test ./internal/...`.
- MCP: `cd mcp-annet-oil && npm install && npm run build` (compiles `src/` → `dist/`).

The built binary is `annet-oil` (CLI). `annet-oil server` runs the API.

## Conventions

- **Module** is `annet-oil` (Go 1.26). Internal imports are `annet-oil/internal/...`.
- **Adding a feature end-to-end** follows the pattern established by `check` — replicate all four layers:
  1. Core logic + types in a new `internal/<feature>/` package (pure, testable, no HTTP).
  2. REST handler in `internal/api/handlers/<feature>.go` exposing a `New<Feature>Handler()`
     `http.Handler`, then `r.Mount("/<feature>", ...)` in `internal/api/server.go`.
  3. Cobra command in `internal/cli/<feature>.go`, registered via `rootCmd.AddCommand` in its `init()`.
  4. MCP tool in `mcp-annet-oil/src/index.ts` — add a `Tool` entry to `tools[]`, a `case` in the
     `CallToolRequestSchema` switch, and a typed client method + result type in `client.ts`.
- **Inventory** is the source of truth for devices (`vendor`, `platform`, `role`, `credentials`).
  Credentials resolve most-specific-first: device → role group → `default` (see
  `inventory.CredentialsFor`). Vendor names are lowercased on load.
- Handlers accept both `GET` (query params) and `POST` (JSON body) where practical (see `check.go`).
- Keep JSON field names snake_case (struct tags) to match the existing API and MCP client.
- Result/report types carry structured `Error{Type,Message}` with named error-type constants,
  rather than bare error strings — mirror this for new diagnostic features.

## Notes

- There is no committed CLAUDE.md history; `README.md` and `docs/` hold the deeper design notes
  (MCP host architecture, SSH key auth, Docker MCP).
- Secrets/inventory live in `.env` and `resources/inventory.yaml` (gitignored variants exist);
  never commit real credentials.
