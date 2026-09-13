---
platform: linkedin
topic: introduction
length: ~200 words
draft: true
---

# LinkedIn post — Introduction

**Hook option A (problem-first):**

If you run a multi-vendor network, you know the tax: Juniper here, Cisco there,
some Huawei, a few MikroTik and Eltex boxes. Every change is a small negotiation
with whatever CLI is in front of you.

annet (the open-source config generator) fixes a lot of that — it generates
vendor-correct config, diffs it against the device, and builds the patch. But in
practice you run *several* annet containers (default, telnet-only legacy,
vendor-pinned), and "run annet" quietly becomes "remember which container owns
this hostname and docker exec into it."

So I built **Annet Oil** — a small Go service that puts ONE front door in front
of all your annet containers.

You type:
`annet-oil diff -g core-rtr-1`

It figures out which container owns that host, runs annet there, and streams the
diff back. No more docker exec. No more guessing.

The same four verbs — gen / diff / patch / deploy — are exposed three ways:
→ a CLI for humans
→ a REST API for CI/CD
→ an MCP server so an AI agent can drive the network through the *same*
  authenticated, whitelisted guardrails a human uses.

One well-guarded front door; every consumer — human, pipeline, or agent — gets
the guarantees for free.

Full write-up on my blog 👇 [link]

How are you taming multi-vendor config today?

#NetworkAutomation #NetDevOps #Golang #Networking

---

**Hook option B (one-liner, punchier):**

I got tired of remembering which annet container owns which router.

So I gave my whole annet fleet a single front door — one CLI, one REST API, and
(the fun part) one MCP server that lets an AI agent run gen/diff/patch/deploy
through the *same* auth and guardrails a human uses.

It's called Annet Oil. Write-up in comments. 👇

#NetworkAutomation #NetDevOps #Golang
