---
title: "Annet Oil: One Front Door for Multi-Vendor Network Automation"
description: "Why I wrapped annet in a Go service — and how it turns gen/diff/patch/deploy into a REST API, a CLI, and a set of AI-agent tools."
tags: [network-automation, golang, annet, devops, netdevops]
canonical: true
draft: true
---

# Annet Oil: One Front Door for Multi-Vendor Network Automation

If you run a multi-vendor network, you already know the shape of the problem.
You have Juniper here, Cisco there, a rack of Huawei, a handful of MikroTik and
Eltex boxes someone added "temporarily" three years ago. Each vendor has its own
CLI, its own config dialect, its own idea of what "commit" means. And every
change is a small negotiation with the specific box in front of you.

[annet](https://github.com/annetutil/annet) — the open-source configuration
generator from Yandex — solves a huge part of this. It generates vendor-correct
config, computes a diff against the running device, and produces a patch. It's
genuinely good. But annet is a Python tool you run at a container's command line,
and in practice you rarely run just one annet. You end up with several: a default
one, one built for telnet-only legacy gear, one pinned to a specific vendor
image. Suddenly "run annet" means "remember which container this hostname belongs
to, `docker exec` into it, and hope you picked the right one."

**Annet Oil** is the thing that removes that last mile of friction. It's a small
Go service that puts a single front door in front of all your annet containers.

## What it actually does

Annet Oil takes the four operations that matter —

- **`gen`** — generate the target configuration for a device,
- **`diff`** — compare target vs. running,
- **`patch`** — produce the change set,
- **`deploy`** — apply it —

and exposes them through **three interfaces that share one engine**:

1. A **CLI** (`annet-oil`) for humans at a terminal.
2. A **REST API** (`:8080`) for CI/CD, scripts, and dashboards.
3. An **MCP server** so an AI agent can drive the network through the *same*
   guardrails, not a side channel.

Under the hood it does the boring, important thing you'd otherwise do by hand:
**it routes each command to the correct annet container based on the hostname.**
You ask for `core-rtr-1`; Annet Oil looks up which container owns that host and
runs the command there. You never `docker exec` again.

```bash
# You type this…
annet-oil diff -g core-rtr-1

# …and Annet Oil figures out "core-rtr-1 lives in annet-telnet",
# execs annet inside that container, and streams the diff back.
```

The same request over HTTP:

```bash
curl -X POST http://localhost:8080/api/v0/diff \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filters": ["core-rtr-1"]}'
```

Same routing, same engine, same result — just a different door.

## More than a proxy

If it stopped at "proxy annet over HTTP," it would still be useful. But the
interesting part is what a single front door lets you build *around* those four
verbs:

- **Device availability checks** — probe TCP ports and attempt an SSH login
  across the whole inventory in parallel, and get back a structured
  reachable/unreachable report (JSON or CSV). Great for a pre-change sanity
  sweep or a post-maintenance verification.
- **Inventory as the source of truth** — vendor, platform, role, and
  credentials live in one YAML file. Credentials resolve most-specific-first:
  device → role group → default. Reload it on the fly without a restart.
- **Feature sets** — a curated knowledge base that answers "does this
  vendor/model/firmware actually support this feature?" so an operator (or an
  agent) doesn't propose config the box can't run.
- **Operational state** — normalized, vendor-neutral device state (facts,
  interfaces, LLDP, MAC, ARP, routes) returned as compact JSON, regardless of
  whether it came from Juniper's `| display json` or a scraped Cisco table.
- **RFC workflow** — create a Jira change ticket, attach the diff, submit for
  review, deploy after approval, and close the ticket — as a first-class flow.

Every one of these is exposed through all three interfaces. That consistency is
the whole point.

## Why this matters now: the AI-agent angle

Here's the part I didn't expect to care about when I started.

The moment your network operations are a clean REST API with real
authentication, RBAC, a command whitelist, and structured errors, you can hand
them to an AI agent *safely*. Annet Oil ships an MCP (Model Context Protocol)
server that exposes each operation as a tool. An agent can now:

> "Generate the diff for `core-rtr-1`, open an RFC, and post the diff to it for
> review."

…and it does that by calling the **same authenticated, whitelisted, audited API
a human would** — not by SSHing into a box with a scraped password and hoping.
The guardrails aren't bolted on for the AI; they're the same ones your CI uses.

That's the thesis of the whole project: **build one well-guarded front door, and
every consumer — human, pipeline, or agent — gets the guarantees for free.**

## What it's not

Being honest about scope matters:

- It **doesn't replace annet.** annet does the hard config-generation work;
  Annet Oil orchestrates and exposes it.
- It talks to annet containers via the **Docker Exec API**, not a Go SDK. That's
  a deliberate trade-off (simplicity and zero container changes vs. text
  parsing), and it deserves its own article — which is [next in the series](./02-architecture.md).
- It's built for the "tens to low-hundreds of requests/second" world of change
  management, not as a high-throughput streaming telemetry pipe.

## Where to go next

The next post digs into the architecture: the four layers every feature follows,
why routing lives where it does, and the Docker-Exec-vs-SDK decision that shapes
the whole system.

If you run a mixed-vendor network and any of the above sounded like a Tuesday
you'd rather not repeat — I'd love to hear how you're solving it today.

*Annet Oil is built on top of [annet](https://github.com/annetutil/annet). This
is a personal project and not affiliated with the annet maintainers.*