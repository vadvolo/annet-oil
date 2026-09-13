# Annet Oil — Blog & Content Plan

This folder holds draft articles about **Annet Oil** for different publishing
channels. Everything here is a draft for review before posting.

## Folder layout

```
blogs/
├── README.md            # this file — content plan + channel strategy
├── netdev.dev/          # long-form English (your blog) — the canonical version
├── linkedin/            # short English posts, hook-first, 1 idea each
└── habr/                # long-form Russian translations/adaptations
```

Each numbered file maps to one topic in the series, so `01-*` is the same story
told three ways (long English / short English / long Russian).

## Series roadmap

| # | Topic | Status |
|---|-------|--------|
| 01 | Introduction — what Annet Oil is and the problem it solves | ✅ draft |
| 02 | Architecture — the four layers and the Docker Exec decision | ✅ draft |
| 03 | Routing & inventory — how a hostname finds its container | 📝 planned |
| 04 | The MCP server — giving an AI agent safe hands on the network | 📝 planned |
| 05 | RFC workflow — change management with Jira in the loop | 📝 planned |
| 06 | Feature sets & operational state — teaching the agent what a box can do | 📝 planned |
| 07 | Device availability checks at scale (`check` / CheckEast) | 📝 planned |

## Channel strategy (how each version differs)

- **netdev.dev (your blog, English)** — the *canonical, long-form* piece.
  Diagrams, code, design rationale. Everything else is derived from this.
- **LinkedIn (English, short)** — one hook, one idea, one takeaway. 150–250
  words. End with a soft question to drive comments. Link back to netdev.dev.
- **Habr (Russian, long-form)** — a full adaptation, not a literal translation.
  Habr's audience is deeply technical and rewards honest trade-off discussion
  and real code. Use the `<cut>` tag after the intro.

## Other channels worth considering

- **dev.to / Hashnode** — cross-post the canonical English article (set the
  canonical URL to netdev.dev so you don't split SEO). Zero extra writing.
- **Reddit** — r/networking and r/devops for the intro; r/golang for the
  architecture/Docker-Exec piece. Lead with the problem, not the product.
- **Hacker News (Show HN)** — save for a milestone (v1.0, or the MCP/AI-agent
  angle, which is the most HN-friendly hook).
- **GitHub README / Discussions** — the intro article makes a great expanded
  README landing section.
- **NANOG / RIPE mailing lists or a lightning talk** — the network-automation
  crowd; strong fit for the RFC-workflow and feature-set topics.
- **Newsletter mentions** — `Console`, `Networking Notes`, `Golang Weekly`
  (for the Docker-Exec-vs-SDK design piece).

## Voice & house style

- Write for a senior network/platform engineer who is skeptical of hype.
- Lead with the problem before the solution. Show real commands and output.
- Be honest about limitations — the Docker-Exec section already models this.
- Keep vendor names accurate (Juniper, Cisco, Huawei, MikroTik, Eltex, etc.).
- Prefer "we/you" over passive voice. Short paragraphs.