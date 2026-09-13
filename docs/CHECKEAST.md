# Checkeast — Calculate a Diff and Archive it to S3

> **Status:** design / implementation guide. This document describes a feature to build;
> it is written so you can implement it yourself and understand every moving part.

## What is `checkeast`?

`checkeast` (a play on *check* + *чекист*) is a new capability that:

1. **Calculates a configuration diff** for one or more devices — exactly what `annet diff`
   already does.
2. **Archives that diff to S3** — saving **both** a combined JSON of the whole run **and** one
   raw `.diff` text file per host.
3. **Returns the diff plus the S3 locations** where it was stored.

It is a *dedicated* feature (its own endpoint, CLI command, and MCP tool), sitting next to `diff`
rather than being a flag on it. The point is to keep an auditable, timestamped record of every diff
in object storage.

```
checkeast(hosts)
   ├─ 1. run diff   ──> annet.Service.ExecuteCommand({command:"diff", ...})   (reused as-is)
   │                     └─> *annet.CommandResponse  (per-host diff text in CommandResult.Stdout)
   ├─ 2. archive    ──> checkeast.Store.Archive(ctx, req, resp)
   │                     ├─ combined JSON -> {prefix}{YYYY/MM/DD}/{runID}/diff.json.gz
   │                     └─ per host      -> {prefix}{YYYY/MM/DD}/{runID}/{host}.diff
   └─ 3. return     ──> { diff: <CommandResponse>, archive: <Report with s3:// locations> }
```

## Background: how a diff works today

Understanding the existing diff path is the prerequisite — `checkeast` *reuses* it.

`diff` is **not** special-cased. It is one of four commands (`gen`, `diff`, `patch`, `deploy`) that
all flow through a single generic pipeline. The "diff" identity is just the string `"diff"` in the
`Command` field.

| Layer | File | Role |
|-------|------|------|
| Core  | `internal/annet/commands.go` | `Service.ExecuteCommand` builds the annet argv, runs it in a container, aggregates results |
| REST  | `internal/api/handlers/diff.go` | `POST/GET /api/v0/diff` → parses request → calls the service |
| CLI   | `internal/cli/diff.go` | `annet-oil diff <hosts>` → builds the request → calls the service |
| MCP   | `mcp-annet-oil/src/index.ts`, `client.ts` | `annet_diff` tool → HTTP POST to the API |

The request and response types (in `internal/annet/commands.go`):

```go
type CommandRequest struct {
    Command    string   `json:"command"`            // "diff"
    Filters    []string `json:"filters,omitempty"`  // hostnames (routing)
    Generators []string `json:"generators,omitempty"` // -g generator filters
    Container  string   `json:"container,omitempty"`
    Parallel   bool     `json:"parallel,omitempty"`
    Timeout    int      `json:"timeout,omitempty"`
    Quiet      bool     `json:"quiet,omitempty"`
    // ... exclude_generators, dry_run, extra_args, environment
}

type CommandResponse struct {
    Success      bool                      `json:"success"`
    Results      map[string]*CommandResult `json:"results,omitempty"` // keyed by hostname
    TotalHosts   int                       `json:"total_hosts"`
    SuccessHosts int                       `json:"success_hosts"`
    FailedHosts  int                       `json:"failed_hosts"`
}

type CommandResult struct {
    Container string `json:"container"`
    ExitCode  int    `json:"exit_code"`
    Stdout    string `json:"stdout"`   // <-- the actual diff text lives here, per host
    Stderr    string `json:"stderr"`
    Error     string `json:"error,omitempty"`
}
```

**Key takeaway for `checkeast`:** call `Service.ExecuteCommand(ctx, req)` with `Command:"diff"`, then
read the per-host diff from `resp.Results[host].Stdout`. You never re-implement diffing.

## Background: how S3 works today

The project already talks to S3 — but only for **log archival** in `internal/logging/s3.go`. That
uploader is *not* directly reusable for diffs because it:

- is tied to a **file on disk** and a **rotation ticker** (uploads `cfg.Output` every hour),
- is **AWS-only** (no custom endpoint, so it can't target MinIO),
- lives as a **CLI-only global** (`internal/cli/root.go`), so the REST/MCP path has no S3 access.

What you *should* reuse is the **upload technique**, not the type. The important lines
(`internal/logging/s3.go`):

```go
// Client creation — LoadDefaultConfig pulls credentials from the AWS default chain
// (env vars AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY, shared config, IAM role, ...).
awsCfg, _ := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.S3.Region))
client := s3.NewFromConfig(awsCfg)

// Upload — gzip into a buffer, then PutObject.
var buf bytes.Buffer
gz := gzip.NewWriter(&buf); gz.Write(data); gz.Close()
client.PutObject(ctx, &s3.PutObjectInput{
    Bucket:          aws.String(bucket),
    Key:             aws.String(key),
    Body:            io.NopCloser(&buf),
    ContentType:     aws.String("application/gzip"),
    ContentEncoding: aws.String("gzip"),
})
```

The existing object key pattern is `{prefix}{YYYY/MM/DD}/{basename}.gz`. We follow the same
date-partitioned idea but add a per-run folder.

### The one new S3 trick: a custom endpoint (for MinIO / S3-compatible storage)

To let `checkeast` target non-AWS storage, pass a `BaseEndpoint` when creating the client:

```go
client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
    if endpoint != "" {
        o.BaseEndpoint = aws.String(endpoint) // e.g. "http://localhost:9000"
        o.UsePathStyle  = true                // required by MinIO
    }
})
```

Credentials still come from the AWS default chain (env vars are the easy path for MinIO).

## The 4-layer implementation

The project convention (see `CLAUDE.md`) is that every feature is built in four layers, mirroring the
`check` feature. Build them in this order — each layer depends on the one above.

### Layer 1 — Core package `internal/checkeast/`

Pure, testable, no HTTP and no Docker. Two files plus a test.

**`internal/checkeast/store.go`** — the S3 client and a generic "put bytes" helper.

```go
package checkeast

// Store archives diffs to S3-compatible storage. A nil *Store means "disabled";
// callers must null-check it (same convention as logging.NewS3Uploader).
type Store struct {
    client *s3.Client
    bucket string
    prefix string
}

// New returns (nil, nil) when the store is disabled or misconfigured, so callers
// can simply check `if store != nil`.
func New(cfg config.S3StoreConfig) (*Store, error) {
    if !cfg.Enabled || cfg.Bucket == "" {
        return nil, nil
    }
    awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
    if err != nil { return nil, err }
    client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
        if cfg.Endpoint != "" {
            o.BaseEndpoint = aws.String(cfg.Endpoint)
            o.UsePathStyle  = true
        }
    })
    return &Store{client: client, bucket: cfg.Bucket, prefix: cfg.Prefix}, nil
}

// put uploads one object and returns the number of bytes written.
func (s *Store) put(ctx context.Context, key string, body []byte, contentType string, gzipped bool) (int, error) {
    // gzip into a bytes.Buffer when gzipped==true, then PutObject —
    // copy the pattern from internal/logging/s3.go.
}
```

**`internal/checkeast/checkeast.go`** — orchestration + result types. Note the structured-error
convention (`Error{Type, Message}` with named constants) borrowed from `internal/check/check.go`.

```go
// Error types (mirror internal/check/check.go style).
const (
    ErrStoreDisabled = "store_disabled"
    ErrUpload        = "upload_err"
    ErrEmptyDiff     = "empty_diff"
)

type Error struct {
    Type    string `json:"type"`
    Message string `json:"message"`
}

type Artifact struct {
    Key         string `json:"key"`
    Location    string `json:"location"` // s3://bucket/key
    ContentType string `json:"content_type"`
    Size        int    `json:"size"`
    Error       *Error `json:"error,omitempty"`
}

type Report struct {
    RunID       string               `json:"run_id"`
    Timestamp   string               `json:"timestamp"`
    Bucket      string               `json:"bucket"`
    Combined    *Artifact            `json:"combined,omitempty"`
    PerHost     map[string]*Artifact `json:"per_host,omitempty"`
    StoredHosts int                  `json:"stored_hosts"`
    FailedHosts int                  `json:"failed_hosts"`
    Success     bool                 `json:"success"`
    Error       *Error               `json:"error,omitempty"`
}

// Archive stores the diff result. It does NOT run the diff — the caller runs the
// diff and passes the response in, keeping this package pure and unit-testable.
func (s *Store) Archive(ctx context.Context, req *annet.CommandRequest, resp *annet.CommandResponse) (*Report, error) {
    // 1. runID := timestamp (+ host token). timestamp := time.Now().UTC().
    // 2. Combined: json.Marshal a struct wrapping req metadata + resp; gzip;
    //    put -> {prefix}{YYYY/MM/DD}/{runID}/diff.json.gz  (application/gzip)
    // 3. Per host: for host, res := range resp.Results:
    //    put res.Stdout as text/plain -> {prefix}{YYYY/MM/DD}/{runID}/{host}.diff
    //    (annotate empty diffs with ErrEmptyDiff, do not abort the whole run)
    // 4. Aggregate counts; set Location = "s3://"+bucket+"/"+key for each artifact.
}
```

**`internal/checkeast/checkeast_test.go`** — inject a *fake putter* (define a tiny interface the
`Store` calls, e.g. `objectPutter`) so tests assert key construction, both-artifact output, and error
aggregation **without touching S3**.

> **Test gotcha:** if any test loads the inventory, be aware inventory is a process-global singleton —
> tests that call `inventory.Load` can bleed state into each other. See the note in the project memory
> about the feature-pattern inventory gotcha.

### Layer 2 — REST handler `internal/api/handlers/checkeast.go`

```go
func NewCheckeastHandler(service *annet.Service, store *checkeast.Store) http.Handler {
    r := chi.NewRouter()
    h := &CheckeastHandler{service: service, store: store}
    r.Post("/", h.Handle)
    r.Get("/", h.Handle)
    return r
}
```

- **Request parsing:** copy `parseRequest` from `internal/api/handlers/diff.go` (it forces
  `Command:"diff"` and reads the same fields from JSON body or query params).
- **Handler flow:**
  1. `resp, _ := h.service.ExecuteCommand(ctx, req)`
  2. If `h.store == nil` → return `{ diff: resp, archive: {success:false, error:{type:"store_disabled"}} }`
     with HTTP 200 (archival being off is not an error).
  3. Else `report := h.store.Archive(ctx, req, resp)` → return `{ diff: resp, archive: report }`.
- **Mount it** in `internal/api/server.go`, right after the `/diff` mount:
  ```go
  r.Mount("/checkeast", handlers.NewCheckeastHandler(s.annetService, s.checkeastStore))
  ```
- **Wire the store into the server** — this is the extra plumbing versus a plain handler:
  - Add `checkeastStore *checkeast.Store` to the `Server` struct in `server.go`.
  - Add it as a parameter to `NewServer(...)`.
  - Construct it in `startAPIServer` (`internal/cli/server.go`) from config and pass it in:
    `store, _ := checkeast.New(cfg.Checkeast.S3)`.

### Layer 3 — CLI command `internal/cli/checkeast.go`

Model it on `internal/cli/diff.go`.

```go
var checkeastCmd = &cobra.Command{
    Use:   "checkeast [hostname ...]",
    Short: "Calculate a diff and archive it to S3",
    RunE:  runCheckeastCommand,
}
// flags: --filters/-g, --container, --parallel, --timeout, --quiet, --format
// register in init(): rootCmd.AddCommand(checkeastCmd)

func runCheckeastCommand(cmd *cobra.Command, args []string) error {
    req := &annet.CommandRequest{Command: "diff", Filters: args, Generators: checkeastFilters, /* ... */}
    resp, err := annetService.ExecuteCommand(cmd.Context(), req)
    if err != nil { return err }
    // print the diff (reuse printCommandResponse)
    if checkeastStore == nil {
        fmt.Fprintln(os.Stderr, "S3 archival disabled (checkeast.s3.enabled=false)")
        return nil
    }
    report, _ := checkeastStore.Archive(cmd.Context(), req, resp)
    // print report.Combined.Location and each report.PerHost[...].Location
    return nil
}
```

Add the store as a package global next to `s3Uploader` in `internal/cli/root.go`, and construct it in
`initializeServices()` right after `annetService = annet.New(...)`:

```go
checkeastStore, err = checkeast.New(cfg.Checkeast.S3)
if err != nil { return fmt.Errorf("failed to init checkeast store: %w", err) }
```

### Layer 4 — MCP tool

- **`mcp-annet-oil/src/index.ts`:** add an `annet_checkeast` entry to `tools[]` (reuse
  `commandInputSchema`), and a `case 'annet_checkeast'` in the call switch that calls
  `annetClient.checkeast(params)` and formats the diff + archive locations.
- **`mcp-annet-oil/src/client.ts`:** add
  ```ts
  async checkeast(request: CommandRequest): Promise<CheckeastResponse> {
      const response = await this.client.post('/checkeast', request);
      return response.data;           // do NOT cache — it writes to S3
  }
  ```
  plus `CheckeastResponse` / `Report` / `Artifact` interfaces mirroring the Go JSON tags (snake_case).

## Configuration

Add a dedicated config block so diff archives are independent of the log-archival S3 config and can
use a custom endpoint. In `internal/config/config.go`:

```go
type S3StoreConfig struct {
    Enabled  bool   `yaml:"enabled,omitempty"`
    Bucket   string `yaml:"bucket,omitempty"`
    Prefix   string `yaml:"prefix,omitempty"`   // e.g. "diffs/annet-oil/"
    Region   string `yaml:"region,omitempty"`
    Endpoint string `yaml:"endpoint,omitempty"` // optional: MinIO / S3-compatible
}

type CheckeastConfig struct {
    S3 S3StoreConfig `yaml:"s3,omitempty"`
}

// add to the root Config struct:
//   Checkeast CheckeastConfig `yaml:"checkeast,omitempty"`
```

Have `internal/checkeast.New` accept `config.S3StoreConfig` directly (don't duplicate the type).

Example `config.yaml` block (document it next to the existing `logging.s3` example):

```yaml
checkeast:
  s3:
    enabled: true
    bucket: "network-diffs"
    prefix: "diffs/annet-oil/"
    region: "us-east-1"
    endpoint: ""          # leave empty for AWS; set to e.g. http://localhost:9000 for MinIO
```

AWS credentials are supplied via the standard AWS environment, not the YAML:

```bash
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
```

## Object layout in S3

For a run over hosts `r1`, `r2` on 2026-08-13:

```
diffs/annet-oil/2026/08/13/20260813-142530/diff.json.gz   # whole run: request meta + full CommandResponse
diffs/annet-oil/2026/08/13/20260813-142530/r1.diff        # raw diff text for r1 (CommandResult.Stdout)
diffs/annet-oil/2026/08/13/20260813-142530/r2.diff        # raw diff text for r2
```

- `diff.json.gz` — machine-readable, gzip-compressed; correlates a whole invocation.
- `<host>.diff` — human-readable per-device diff; easy to `aws s3 cp` and read.

## Files to create / modify

**Create**
- `internal/checkeast/store.go`
- `internal/checkeast/checkeast.go`
- `internal/checkeast/checkeast_test.go`
- `internal/api/handlers/checkeast.go`
- `internal/cli/checkeast.go`

**Modify**
- `internal/config/config.go` — `S3StoreConfig`, `CheckeastConfig`, `Config.Checkeast`
- `configs/config.example.yaml` — document the `checkeast.s3` block
- `internal/api/server.go` — `Server` field, `NewServer` param, `/checkeast` mount
- `internal/cli/server.go` — construct the store, pass into `NewServer`
- `internal/cli/root.go` — `checkeastStore` global + construction in `initializeServices`
- `mcp-annet-oil/src/index.ts`, `mcp-annet-oil/src/client.ts` — the tool + client method

## What to reuse (don't reinvent)

| Need | Reuse from |
|------|-----------|
| Run the diff | `annet.Service.ExecuteCommand` — `internal/annet/commands.go` |
| gzip + `PutObject` upload | `internal/logging/s3.go` |
| "nil means disabled" store convention | `logging.NewS3Uploader` returning `(nil, nil)` |
| REST request parsing | `parseRequest` in `internal/api/handlers/diff.go` |
| Structured `Error{Type,Message}` + constants | `internal/check/check.go` |
| CLI command skeleton | `internal/cli/diff.go` and `internal/cli/check.go` |

## Verifying it end-to-end

1. **Build & unit-test:**
   ```bash
   go build ./cmd/annet-oil
   go test ./internal/checkeast/...   # fake putter, no live S3
   ```
2. **Local MinIO (exercises the endpoint override):**
   ```bash
   # start MinIO, create the bucket, then:
   export AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin
   # set checkeast.s3.{enabled,bucket,region,endpoint} in your config
   annet-oil checkeast <host>
   aws --endpoint-url http://localhost:9000 s3 ls --recursive s3://network-diffs/diffs/annet-oil/
   # expect both diff.json.gz and <host>.diff
   ```
3. **REST:**
   ```bash
   curl -XPOST localhost:8080/api/v0/checkeast \
     -H "Authorization: Bearer <token>" \
     -d '{"filters":["<host>"]}'
   # response has "diff" and "archive" with s3:// locations
   ```
4. **Disabled path:** set `enabled: false`, confirm the diff still returns and the archive report
   reports the store disabled (no error, nothing uploaded).
5. **MCP:** `cd mcp-annet-oil && npm run build`, then invoke `annet_checkeast`.

## Suggested build order

1. Config types (`config.go`) — nothing compiles against them yet, safe first step.
2. Core package (`internal/checkeast/`) + its test — get archiving correct against a fake putter.
3. CLI command — the fastest way to run it end-to-end against MinIO.
4. REST handler + server wiring — then test with `curl`.
5. MCP tool — last, since it just proxies the REST endpoint.