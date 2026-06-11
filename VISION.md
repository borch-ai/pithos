# Pithos: The Minimalist Publishing Pipeline

## Overview

`pithos` is a Golang-powered automation pipeline designed for the rapid, low-lift production of niche-market "dark" children's book parodies. Inspired by the philosophy of Diogenes, the tool focuses on stripping away the complexity of the publishing process to reveal the cynical absurdity of the modern world.

The name refers to the large ceramic storage jar utilized by Diogenes of Sinope—a minimalist "business-in-a-jar" for the digital age.

## Core Philosophy

"I am looking for a human." — Diogenes

`pithos` does not aim to replace the human element of satire; it aims to automate the tedious logistics that prevent satire from being profitable. By reducing the "time-to-market" for a 24-page parody to under three hours, the user can focus on the "Alchemical" process of identifying the next great existential dread.

## Architecture & Integration

Pithos is designed as a standalone orchestrator that leverages existing, battle-tested ecosystems rather than reinventing the wheel.

### 1. The Pithos Orchestrator
A focused CLI application built in Go. It provides deterministic, rigid pipelines (`initiate`, `brew`, `assemble`, `deploy`) to manage the state of a book from concept to KDP upload.

### 2. Powerword MCP Ecosystem
Instead of building custom REST clients for LLMs and Image Generators, Pithos directly communicates with **Powerword's Model Context Protocol (MCP)** plugins using the standard `go-sdk`.
- **`pw-mcp-imagegen`**: Generates DALL-E or Midjourney visual assets with consistent style references.
- **`pw-mcp-kdp-math`**: Computes print margins, spine width, and bleed math for PDF layout.
- **`pw-mcp-seo`**: Scrapes keyword volumes and generates Amazon A+ content metadata.

### 3. Lamplighter Telemetry
Generating a 24-page illustrated book is a long-running process (often taking up to 3 hours). Pithos establishes a direct WebRTC connection to **Lamplighter**, allowing the user to step away from their desk.
- Lamplighter will ping the user on their Android device when human approval is required (e.g., approving the cover art or rejecting a generated poem).
- Token usage and cost tracking for the pipeline are relayed in real-time to the Lamplighter dashboard.
