# ADR-0007: Web Crawling and Page Fetching

- **Status:** Aborted / Archived
- **Date:** 2025-12-18
- **Author(s):** Daniela Petruzalek
- **Deciders:** Daniela Petruzalek, Gemini CLI

## 1. Context
To assist agents in looking up external API tutorials, Go module announcements, and online documentation from sites like pkg.go.dev, we felt GoDoctor needed a tool to access raw internet web pages.

## 2. Decision
We decided to implement a web crawling tool named `endoscope` (later semantically renamed to `fetch_webpage`). It accepted a target `url` and crawling parameters, downloaded the remote HTML content, stripped CSS/scripts, and returned a markdown representation of the page to the agent.

## 3. Consequences
- **Positive:** Enabled the agent to fetch package guides and tutorials from public documentation sites in real-time.
- **Negative:** Significantly bloated the scope of the GoDoctor project. Web page fetching was slow, vulnerable to rate-limiting, and was a generic utility that other external tools (or the agent platform itself) could do much better. It took focus away from building deep, Go-specific compiler and LSP integrations.
- **Neutral:** **Aborted.** We decided to retire `fetch_webpage` entirely and focus GoDoctor strictly on native, local Go development toolchain capabilities.
