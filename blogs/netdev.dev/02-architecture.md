---
title: "Inside Annet Oil: Four Layers, One Hostname, and the Docker-Exec Decision"
description: "The architecture of Annet Oil — how a request flows from any interface to the right annet container, why every feature has the same four layers, and the trade-off at the center of the design."
tags: [network-automation, golang, architecture, annet, docker]
canonical: true
draft: true
---

# Inside Annet Oil: Four Layers, One Hostname, and the Docker-Exec Decision

In the [first post](./01-what-is-annet-oil.md) I described *what* Annet Oil is: a
single front door in front of a fleet of [annet](https://github.com/annetutil/annet)
containers, exposed as a CLI, a REST API, and an MCP server. This post is about
*how* it's built — and, more usefully, about two design decisions that turned out
to matter more than anything else.

## The big picture

```
AI agent ──MCP──> mcp-annet-oil (Node) ──HTTP──> Annet Oil API (Go, :8080)
                                                       │
CLI (annet-oil) ──────────────────────────────────────┤
                                                       ├─ Docker Exec ─> annet containers
                                                       ├─ gNETcli ─────> device show/exec
                                                       └─ direct TCP/SSH > device check
```

Three things enter from the left — an AI agent (through MCP), a human (through
the CLI), a pipeline (through HTTP). They all converge on one Go core. From there
the core fans out to three kinds of downstream work: running annet inside
containers, running show/diagnostic commands on live devices via gNETcli, and
probing devices directly for availability checks.

The key property: **the interface you came in through doesn't change what the
core does.** The CLI and the API are thin shells over the same service objects.

## Decision #1: every feature is the same four layers

The most important architectural rule in Annet Oil isn't a diagram — it's a
convention. Every feature, from `diff` to `check` to `featureset`, is built as
**four stacked layers**, and they're always the same four:

1. **Core logic** — a pure Go package under `internal/<feature>/`. No HTTP, no
   Cobra, no Docker. Just types and functions you can unit-test in isolation.
2. **REST handler** — `internal/api/handlers/<feature>.go` exposes the core over
   HTTP and gets mounted in `server.go`.
3. **CLI command** — `internal/cli/<feature>.go` wires the same core into a
   Cobra command.
4. **MCP tool** — an entry in the Node/TypeScript MCP server that calls the REST
   handler, so an agent gets the feature too.

Take the device-availability check as the reference implementation. The pure
logic (TCP probe + SSH login attempt, plus a batch runner) lives in
`internal/check/`. It knows nothing about how it's invoked. Then:

```bash
# Layer 3 (CLI): batch the whole inventory in parallel
annet-oil check --concurrency 100 -o availability-report.json

# Layer 2 (REST): the same core, over HTTP
curl -X POST http://localhost:8080/api/v0/check \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"host": "10.0.0.1", "ports": [22, 23, 10022], "login": true}'
```

Both paths call into the identical `internal/check` code. The CLI adds flag
parsing; the handler adds JSON marshalling and auth. Neither reimplements the
logic. When I add a feature, I'm not deciding *how* to structure it — the
convention already decided. That's the point of a convention: it removes a
category of choices so you can spend your judgment on the actual problem.

One nice side effect: results carry **structured errors** — a typed
`Error{Type, Message}` with named constants rather than bare strings — so every
consumer, including the agent, can branch on error *type* instead of
string-matching a message.

## Decision #2: routing by hostname is the core's real job

annet doesn't know your topology of containers. You might run:

- `annet-default` for the bulk of modern gear,
- `annet-telnet` for legacy boxes that only speak telnet,
- `annet-orion` pinned to a specific vendor image.

The question "which container should run this command for `core-rtr-1`?" has to
be answered *somewhere*, every single time. In Annet Oil it's answered in one
place — the router — driven by a simple JSON map:

```json
{
  "routes": {
    "router1.example.com":     "annet",
    "old-router.example.com":  "annet-telnet",
    "orion-device1.example.com": "annet-orion"
  }
}
```

A request names a host; the router resolves it to a container; the annet service
`docker exec`s the command there. Because routing is centralized and data-driven,
adding a new container or moving a host between containers is a config edit, not
a code change. And because it happens *below* the interface layer, the CLI, the
API, and the agent all get identical routing behavior for free.

## Decision #3: Docker Exec, not an SDK

This is the one people push back on, so it's worth stating plainly.

Annet Oil drives annet through the **Docker Exec API** — effectively
`docker exec <container> annet <args>`, done properly through the Docker SDK for
Go — and parses stdout/stderr. It does **not** integrate annet as a library or
speak to an in-container gRPC server.

Why give up structured responses for text parsing?

**What you get:**

- **Zero changes to the annet containers.** They stay stock. Any annet version,
  any custom build, works the day it ships — no compatibility matrix.
- **Real isolation.** Each annet container is a sealed box with its own
  dependencies; Docker enforces the boundary. No shared Python runtime, no
  version entanglement between the orchestrator and the tool it orchestrates.
- **Trivial extensibility.** A new annet flavor is a new container plus one line
  in the routing map.
- **Good-enough performance.** Exec latency is typically <10 ms — negligible next
  to the seconds an actual `diff` or `deploy` against a device takes. This is a
  change-management system, not a telemetry firehose.

**What you pay:**

- **Text parsing.** You're reading stdout/stderr, not a typed struct. Fragile if
  annet's output format shifts.
- **No real-time progress** for long operations — you get the result when the
  exec returns.
- **Coarser error handling** than a structured API would give you.

I considered the alternatives. Embedding a Python runtime (or a sidecar Python
service) to use annet's SDK would couple Annet Oil to annet's internal API and
drag a language runtime into a Go service. Adding a gRPC/REST server *inside*
each annet container would bloat the images and multiply the moving parts. Both
solve problems I don't have yet.

So the rule I settled on is explicit about *when to revisit it*: migrate off
Docker Exec only if annet ships an official Go SDK, or if I genuinely need
real-time progress, or if sustained load pushes past ~100 req/s. Until one of
those is true, the boring choice is the correct one. **Writing down the
conditions that would flip a decision is what keeps it from calcifying into
dogma.**

## Two more edges worth knowing

**gNETcli for live devices.** `gen`/`diff`/`patch`/`deploy` go through annet. But
diagnostics — show commands, operational state — need to talk to the *device*,
not to annet. That path uses [gNETcli](https://github.com/annetutil/gnetcli),
and it's why the fan-out diagram has three arrows, not one.

**MCP runs in a container, the API runs on the host.** The API needs the Docker
socket to exec into annet containers, so it lives on the host. The MCP server is
sandboxed in its own container and reaches the API over HTTP
(`host.docker.internal:8080`). The agent gets an isolated, minimal surface; the
privileged Docker access stays on the host where it belongs. Same auth token on
both sides ties them together.

## The through-line

If there's one idea holding the architecture together, it's this: **push every
important decision below the interface layer, do it once, and let all three front
doors inherit it.** Routing, auth, structured errors, the four-layer feature
shape — none of them are duplicated per interface. A human at the CLI, a CI job
hitting REST, and an AI agent through MCP are all just callers of the same
guarded core.

Next in the series: **routing and inventory in depth** — how a hostname resolves
to a container *and* to a set of credentials, and why the inventory is the real
source of truth.
