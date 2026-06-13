# Pithos: The Book Factory

## Overview

`pithos` is the **book production factory** of the Borch-AI ecosystem. It takes a validated market concept from Kiln's Foundry and executes the full manufacturing process: generating text, commissioning AI illustrations, assembling print-ready PDFs, and writing pipeline checkpoints that Kiln uses to track production status.

Pithos does not decide what to build. That is Kiln's job. Pithos decides how to build it — fast, deterministically, and cheaply enough that a failed market bet doesn't sting.

The name refers to the large ceramic storage jar used by Diogenes of Sinope — the minimalist's entire home, office, and business in one vessel. We borrow the same ethos: strip away everything unnecessary until only the work remains.

---

## Position in the Stack

Pithos is the **middle layer** of the Borch-AI stack. It is consumed by Kiln and consumes Powerword.

```
Kiln (orchestrator)
  └── Pithos (book factory)  ← you are here
        └── pw-mcp-imagegen   (DALL-E illustrations)
            pw-mcp-typst       (PDF layout engine)
            pw-mcp-kdp-math    (print math: margins, spine, bleed)
            pw-mcp-seo         (KDP metadata — Phase 4+)
            pkg/llm            (LLM access via Powerword)
```

**Kiln orchestrates Pithos** — `kiln forge` launches Pithos subprocesses (`initiate`, `brew`, `assemble`) and reads Pithos's `manifest.json` to track production milestones. Pithos does not call Kiln.

**Lamplighter observes Pithos** — pipeline checkpoints are surfaced to the Lamplighter mobile app via Firebase, so the human can step away during the 1–3 hour `brew` stage.

---

## Core Philosophy

*"I am looking for a human."* — Diogenes

Pithos does not aim to replace human creativity; it automates the logistics that prevent creativity from being profitable. A dark parody of *Goodnight Moon* is a 15-minute burst of satirical inspiration. The 23 hours of work around it — sourcing references, prompting models, resizing images, calculating bleed, formatting PDFs, formatting metadata — is what Pithos eliminates.

**Every hour of Pithos automation is an hour the human spends finding the next great existential dread to monetize.**

---

## Pipeline Stages

```
pithos initiate  →  pithos brew  →  pithos assemble
     │                   │                  │
 Creates workspace    Generates all      Compiles PDFs
 Writes manifest.json  text + images    Validates KDP specs
 Sets up book config   (1–3 hours)      Updates manifest
                                        Signals Lamplighter
```

### `pithos initiate`
Creates the book workspace directory, writes the initial `manifest.json` with the concept brief, and sets up the configuration for the `brew` stage. Kiln writes `book.ID` and `book.Niche` to the manifest before calling initiate.

### `pithos brew`
The main production stage. Invokes `pkg/llm` for text generation and `pw-mcp-imagegen` for illustrations. Long-running (1–3 hours). Checkpoints progress to `manifest.json` so a failed run can be resumed from the last checkpoint, not restarted.

### `pithos assemble`
Invokes `pw-mcp-typst` to compile text and images into print-ready PDFs conforming to KDP bleed, margin, and resolution specs. Runs `pw-mcp-kdp-math` to calculate spine width. Writes final PDF paths to `manifest.json`. Signals Kiln via `kiln_milestones`.

---

## The `manifest.json` State Contract

`manifest.json` is Pithos's state file — a machine-readable record of everything that has happened in the workspace. It is the contract between Pithos and Kiln.

**Kiln reads `manifest.json`; it never writes it.** If Kiln needs additional fields from Pithos, those fields are added to the manifest via a Pithos task (see Task 5.20: Kiln Foundry State Integration).

Key fields Kiln consumes:
```json
{
  "kiln": {
    "kiln_sync_version": 1,
    "kiln_milestones": ["initiate_complete", "brew_complete", "assemble_complete"],
    "total_cost_usd": 1.47,
    "interior_pdf_path": "/abs/path/to/interior.pdf",
    "cover_pdf_path": "/abs/path/to/cover.pdf"
  }
}
```

---

## Technology Stack

| Layer | Technology |
|---|---|
| Language | Go 1.26+ |
| CLI Framework | `spf13/cobra` + `spf13/viper` |
| LLM Access | `github.com/borch-ai/powerword/pkg/llm` |
| MCP Plugins | `pw-mcp-imagegen`, `pw-mcp-typst`, `pw-mcp-kdp-math` from Powerword |
| PDF Layout | Typst binary (system dependency) via `pw-mcp-typst` |
| State | `manifest.json` in workspace directory |
| Telemetry | `github.com/borch-ai/powerword/pkg/telemetry` |

---

## What Pithos Is Not

- **Not an orchestrator.** Pithos does not decide what to build or when to build it. Kiln does.
- **Not a deploy engine.** KDP metadata generation, asset packaging, and uploading are Kiln's responsibility (Phase 5). Pithos produces the raw assets; Kiln ships them.
- **Not a general-purpose book publisher.** Pithos is tuned for a specific aesthetic: illustrated parody children's books for cynical adults. The pipeline makes strong assumptions about page count (24), format (8.5×8.5" square), and tone (dark, satirical). Generalizing the factory is a future Pithos task, not a current goal.
- **Not a standalone tool.** Pithos is designed to be invoked by Kiln. Running it manually is supported for development and debugging; it is not the primary use pattern.
