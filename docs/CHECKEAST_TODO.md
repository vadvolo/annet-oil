# Checkeast — Implementation TODO

Task checklist for the `checkeast` feature (see [CHECKEAST.md](./CHECKEAST.md) for the full design).
Build order is top-to-bottom: config → core → CLI → REST → MCP.

## 0. Prep
- [ ] Read [CHECKEAST.md](./CHECKEAST.md) end-to-end.
- [ ] Confirm you are on branch `feature/checkeast`.
- [ ] Skim the three reuse sources: `internal/annet/commands.go`, `internal/logging/s3.go`, `internal/check/check.go`.

## 1. Config (`internal/config/config.go`)
- [ ] Add `S3StoreConfig` struct: `Enabled, Bucket, Prefix, Region, Endpoint` (yaml tags).
- [ ] Add `CheckeastConfig` struct with `S3 S3StoreConfig`.
- [ ] Add `Checkeast CheckeastConfig \`yaml:"checkeast,omitempty"\`` to the root `Config`.
- [ ] Document the `checkeast.s3` block in `configs/config.example.yaml` (next to `logging.s3`).
- [ ] `go build ./...` compiles.

## 2. Core package (`internal/checkeast/`)
### store.go
- [ ] `Store` struct (`client *s3.Client`, `bucket`, `prefix`).
- [ ] `New(cfg config.S3StoreConfig) (*Store, error)` — return `(nil, nil)` when disabled/misconfigured.
- [ ] Client via `LoadDefaultConfig(WithRegion(...))` + `s3.NewFromConfig`.
- [ ] Endpoint override: set `o.BaseEndpoint` + `o.UsePathStyle` when `Endpoint != ""`.
- [ ] `put(ctx, key, body, contentType, gzipped) (int, error)` — gzip buffer + `PutObject` (copy from `logging/s3.go`).

### checkeast.go
- [ ] Error constants: `ErrStoreDisabled`, `ErrUpload`, `ErrEmptyDiff`.
- [ ] Types: `Error`, `Artifact`, `Report` (snake_case JSON tags).
- [ ] `Archive(ctx, req *annet.CommandRequest, resp *annet.CommandResponse) (*Report, error)`:
  - [ ] Build `runID` (UTC timestamp `20060102-150405`, + host token for multi-host).
  - [ ] Combined artifact → `{prefix}{YYYY/MM/DD}/{runID}/diff.json.gz` (gzipped JSON of req meta + resp).
  - [ ] Per-host artifact → `{prefix}{YYYY/MM/DD}/{runID}/{host}.diff` (raw `Stdout`, `text/plain`).
  - [ ] Set `Location = s3://{bucket}/{key}` per artifact.
  - [ ] Annotate empty diffs with `ErrEmptyDiff`; do NOT abort the run on a single-host upload failure.
  - [ ] Aggregate `StoredHosts` / `FailedHosts` / `Success`.

### checkeast_test.go
- [ ] Define a small `objectPutter` interface so `Store` can take a fake in tests.
- [ ] Test: key construction (combined + per-host paths).
- [ ] Test: both artifacts produced for a 2-host response.
- [ ] Test: upload error on one host is recorded but doesn't fail the whole run.
- [ ] Test: empty diff → `ErrEmptyDiff`.
- [ ] `go test ./internal/checkeast/...` passes (no live S3).

## 3. CLI (`internal/cli/checkeast.go`)
- [ ] `checkeastCmd` (`Use: "checkeast [hostname ...]"`), flags: `--filters/-g`, `--container`, `--parallel`, `--timeout`, `--quiet`, `--format`.
- [ ] Register via `rootCmd.AddCommand(checkeastCmd)` in `init()`.
- [ ] `runCheckeastCommand`: build `CommandRequest{Command:"diff", ...}`, call `annetService.ExecuteCommand`.
- [ ] Print the diff (`printCommandResponse`), then archive locations.
- [ ] Handle disabled store (`checkeastStore == nil`) with a notice, not an error.
- [ ] Add `checkeastStore *checkeast.Store` global in `internal/cli/root.go` (next to `s3Uploader`).
- [ ] Construct it in `initializeServices()` after `annetService = annet.New(...)`.

## 4. REST (`internal/api/handlers/checkeast.go` + wiring)
- [ ] `NewCheckeastHandler(service *annet.Service, store *checkeast.Store) http.Handler` (chi, POST + GET).
- [ ] Copy `parseRequest` from `handlers/diff.go` (forces `Command:"diff"`).
- [ ] Handler: run diff → if store nil return `archive` with `store_disabled` (HTTP 200) → else `Archive` → return `{ diff, archive }`.
- [ ] `internal/api/server.go`: add `checkeastStore` field to `Server`, param to `NewServer`, mount `/checkeast` after `/diff`.
- [ ] `internal/cli/server.go`: construct `checkeast.New(cfg.Checkeast.S3)` in `startAPIServer`, pass into `NewServer`.

## 5. MCP (`mcp-annet-oil/src/`)
- [ ] `index.ts`: add `annet_checkeast` to `tools[]` (reuse `commandInputSchema`).
- [ ] `index.ts`: add `case 'annet_checkeast'` calling `annetClient.checkeast(params)`, format diff + locations.
- [ ] `client.ts`: `async checkeast(request): Promise<CheckeastResponse>` → POST `/checkeast` (NO cache).
- [ ] `client.ts`: `CheckeastResponse` / `Report` / `Artifact` interfaces (snake_case).
- [ ] `cd mcp-annet-oil && npm run build` compiles.

## 6. Verify end-to-end
- [ ] `go build ./cmd/annet-oil` + `go test ./internal/...` green.
- [ ] `make lint` / `make check` clean (gofmt/vet).
- [ ] MinIO: `annet-oil checkeast <host>` → `aws --endpoint-url ... s3 ls --recursive` shows `diff.json.gz` + `<host>.diff`.
- [ ] REST: `curl -XPOST .../api/v0/checkeast -d '{"filters":["<host>"]}'` returns `diff` + `archive` with `s3://` locations.
- [ ] Disabled path (`enabled:false`): diff still returns, archive reports store disabled.
- [ ] MCP: invoke `annet_checkeast`, confirm diff + locations.

## 7. Wrap-up (optional)
- [ ] Add a `checkeast` line to the README features list.
- [ ] Update `docs/` cross-links if needed.
- [ ] Commit per logical layer; open PR against `main`.