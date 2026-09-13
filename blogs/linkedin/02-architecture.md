---
platform: linkedin
topic: architecture
length: ~220 words
draft: true
---

# LinkedIn post — Architecture

**Hook option A (the contrarian design choice):**

Unpopular architecture opinion: I drive my network-automation tooling with
`docker exec` and parse stdout. On purpose.

Annet Oil orchestrates a fleet of annet containers. The obvious "clean" move
would be a Go SDK or a gRPC server inside each container. I use the Docker Exec
API instead. Here's the trade I made:

What I gave up:
→ typed responses (I parse text)
→ real-time progress on long ops

What I got:
→ ZERO changes to the annet containers — any version, any custom build, works
  the day it ships
→ real isolation — no shared Python runtime, no version entanglement
→ a new annet flavor = a new container + one line of routing config
→ <10 ms exec latency, invisible next to a multi-second deploy

The lesson isn't "text parsing good." It's: write down the conditions that would
flip the decision. Mine are — an official annet Go SDK, a real need for live
progress, or sustained load past ~100 req/s. Until one is true, the boring
choice wins.

The other rule that carries the whole codebase: every feature is the same four
layers — pure core → REST handler → CLI command → MCP tool. Routing, auth, and
structured errors all live *below* the interface, so CLI, API, and AI agent
inherit them for free.

Deep dive on the blog 👇 [link]

What's a "boring choice" that quietly saved your architecture?

#Golang #SoftwareArchitecture #NetworkAutomation #NetDevOps

---

**Hook option B (the four-layer rule):**

The best decision in my last project wasn't a framework. It was a convention:

Every feature = 4 layers, always the same order.
pure core → REST handler → CLI command → MCP tool.

The core knows nothing about HTTP, Cobra, or Docker. Routing, auth, and typed
errors live below the interface layer — so a human at the CLI, a CI job on REST,
and an AI agent over MCP all inherit them for free. No per-interface duplication.

Adding a feature stopped being "how do I structure this?" The convention already
answered it. I spend my judgment on the actual problem instead.

Wrote up the full architecture of Annet Oil 👇 [link]

#Golang #SoftwareArchitecture #CleanCode
