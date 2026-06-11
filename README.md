# Pithos

`pithos` is a Golang-powered automation pipeline designed for the rapid, low-lift production of niche-market "dark" children's book parodies. Inspired by the philosophy of Diogenes, the tool strips away the complexity of the publishing process to automate the tedious logistics of digital publishing.

The name refers to the large ceramic storage jar utilized by Diogenes of Sinope—a minimalist "business-in-a-jar" for the digital age.

---

## Features

- **Thematic Transmutation:** Automates the generation of 15-page parodic poems using LLM integration (Gemini/GPT-4o), maintaining strict adherence to the rhythm and meter of classic nursery rhymes.  
- **Visual Consistency Engine:** Orchestrates visual asset generation by invoking `pw-mcp-imagegen` with shared Style References (`--sref`) to ensure the "Great Green Room" remains aesthetically consistent across an entire series.  
- **Layout Automation:** Coordinates layout math (bleed, margins, spine width calculation) by delegating to `pw-mcp-kdp-math` and generating print-ready layouts or JSON manifests. Enforces Amazon KDP rules (e.g., page-count constraints for hardcovers).  
- **KDP Metadata Generator:** Queries the `pw-mcp-seo` plugin to retrieve optimized Amazon keywords, categories, and "A+ Content" descriptions based on real-time market search volume.  
- **Viral Asset Builder:** Queries a video generation MCP plugin (`pw-mcp-video`) to produce 15-second ASMR-style promotional trailers for TikTok and Reels.
- **Checkpoint & Resumability State Machine:** Implements a local project `manifest.json` state tracker to checkpoint progress, enabling recovery of long-running operations (e.g., 3-hour generation runs) without losing data or API tokens.

---

## Core Architecture

`pithos` is structured as a modular CLI tool:

1. **`pithos initiate`**: Initializes the target book workspace directory, copies template manifests, and sets up project configuration metadata.  
2. **`pithos brew`**: Performs incremental, checkpointed generation of the manuscript and page illustrations.  
3. **`pithos assemble`**: Validates the technical layout constraints (such as the KDP hardcover 75-page limit) and coordinates layout manifests.  
4. **`pithos deploy`**: Package-bundles assets, generates SEO keywords, and prepares metadata files for final KDP submission.

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
seo_path = "/usr/local/bin/pw-mcp-seo"
video_path = "/usr/local/bin/pw-mcp-video"

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

---

## Philosophy

> "I am looking for a human." — Diogenes

`pithos` does not aim to replace the human element of satire; it aims to automate the tedious logistics that prevent satire from being profitable. By reducing the "time-to-market" for a 24-page parody to under three hours, the user can focus on the "Alchemical" process of identifying the next great existential dread.

---

## License

Distributed under the MIT License. See `LICENSE` for details. Use for "dumb ideas" encouraged.
