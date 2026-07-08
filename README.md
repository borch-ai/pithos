# Pithos

`pithos` is a Golang-powered automation pipeline designed for the rapid, low-lift production of niche-market "dark" children's book parodies. Inspired by the philosophy of Diogenes, the tool strips away the complexity of the publishing process to automate the tedious logistics of digital publishing.

The name refers to the large ceramic storage jar utilized by Diogenes of Sinope—a minimalist "business-in-a-jar" for the digital age.

---

## Features

- **Thematic Transmutation:** Automates the generation of 15-page parodic poems using LLM integration (Gemini/GPT-4o), maintaining strict adherence to the rhythm and meter of classic nursery rhymes.  
- **Visual Consistency Engine:** Orchestrates visual asset generation by invoking `pw-mcp-imagegen` with shared Style References (`--sref`) to ensure visual style remains consistent across an entire series.  
- **Layout & PDF Assembly:** Coordinates layout math (bleed, margins, spine width calculation) by delegating to `pw-mcp-kdp-math` and compiles high-fidelity, print-ready PDFs via the `pw-mcp-typst` layout engine.  
- **EPUB & Preflight Inspection:** Compiles spec-compliant EPUB digital publications using the `pw-mcp-epub` plugin, and executes preflight validations (e.g., image DPI checks, font embedding verification) via `pw-mcp-pdfcheck` to enforce KDP paperback ingest rules.  
- **Checkpoint & Resumability State Machine:** Implements a local project `manifest.json` state tracker to checkpoint progress, enabling recovery of long-running operations (e.g., 3-hour generation runs) without losing data or API tokens.

---

## Core Architecture

`pithos` is structured as a modular CLI tool focused entirely on book asset production:

1. **`pithos initiate`**: Initializes the target book workspace directory, copies template manifests, and sets up project configuration metadata.  
2. **`pithos brew`**: Performs incremental, checkpointed generation of the manuscript and page illustrations.  
3. **`pithos assemble`**: Validates print layout constraints (such as the KDP hardcover 75-page limit), compiles print-ready PDFs and digital EPUB files, and performs KDP preflight inspection.
4. **`pithos status` & `pithos clean`**: Inspects workspace health, checks generation states and telemetry costs, and repairs corrupted or orphaned assets.

---

## Installation

Clone the repository and install the binary locally:

```bash
go install github.com/borchai/pithos@latest
```

Ensure you configure a `.pithos.toml` file in your project root or `~/.config/pithos/` specifying the binary paths to the required Powerword MCP servers and API environment variables:

```toml
[mcp]
imagegen_path = "/usr/local/bin/pw-mcp-imagegen"
kdp_math_path = "/usr/local/bin/pw-mcp-kdp-math"
typst_path = "/usr/local/bin/pw-mcp-typst"
epub_path = "/usr/local/bin/pw-mcp-epub"
pdfcheck_path = "/usr/local/bin/pw-mcp-pdfcheck"

[api]
gemini_key = "YOUR_GEMINI_API_KEY"
```

---

## Usage

### 1. Initialize a Book Project Workspace
```bash
pithos initiate --output ./books/goodnight-everyone
```

### 2. Generate Book Concept and Assets
Runs the manuscript generation and crawls the image generator sequentially (checkpointed & resumable):
```bash
pithos brew --theme "radon" --style "eerie-vintage" --output ./books/goodnight-everyone
```

### 3. Validate and Compile the Layout
Validate the layout against print limits (Note: `--format hardcover` requires a page count of at least 75 pages under KDP rules):
```bash
pithos assemble --input ./books/goodnight-everyone --format paperback --bleed=true
```

### 4. Inspect Workspace Status
View a detailed diagnostic summary of a book's completion states, remaining pages, and telemetry costs:
```bash
pithos status goodnight-everyone
```

### 5. Repair and Clean Workspace
Reset stuck page generation states or prune orphaned image assets that are no longer referenced by the project manifest:
```bash
pithos clean goodnight-everyone --orphans --reset-failed
```

---

## Philosophy

> "I am looking for a human." — Diogenes

`pithos` does not aim to replace the human element of satire; it aims to automate the tedious logistics that prevent satire from being profitable. By reducing the "time-to-market" for a 24-page parody to under three hours, the user can focus on the "Alchemical" process of identifying the next great existential dread.

---

## License

Distributed under the MIT License. See `LICENSE` for details. Use for "dumb ideas" encouraged.
