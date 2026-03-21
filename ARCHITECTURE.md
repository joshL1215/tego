# Tego Architecture

> **Token Eater Go** — a forward proxy for the Anthropic API that intercepts requests and filters noisy CLI output from `tool_result` content blocks before forwarding upstream, reducing token consumption.

## How It Works

Tego sits between an API client (e.g. Claude Code) and `api.anthropic.com`. It intercepts `POST /v1/messages` requests, parses the JSON body to find `tool_result` content blocks, runs them through a filtering pipeline, and forwards the smaller payload upstream. All other API paths are passed through unmodified. Responses are streamed back with SSE flush support.

```
Client (Claude Code, etc.)
        │
        ▼
  ┌────────────┐
  │  Tego Proxy │  :8400
  │             │
  │  Parse JSON │──▶ Find tool_result blocks
  │  Filter     │──▶ Strip ANSI, drop noise, collapse repetition
  │  Re-marshal │──▶ Update Content-Length
  └─────┬───────┘
        │
        ▼
  api.anthropic.com
        │
        ▼
  Stream response back to client (SSE-aware flushing)
```

## Project Layout

```
cmd/tego/main.go             CLI entry point (Cobra). Commands: serve, test
internal/
  proxy/
    server.go                HTTP server, routing, upstream config
    handler.go               handleMessages (filter path), handlePassthrough
  filter/
    config.go                YAML config loader (~/.config/tego/filters.yaml → embedded default)
    engine.go                Filter pipeline orchestrator + FilterStats
    rules.go                 Strip ANSI, drop lines, collapse tests/blanks/repeats
    default_filters.yaml     Built-in filter rules (20+ regex patterns)
  message/
    parse.go                 JSON parsing, tool_result extraction (legacy + modern format)
  stats/
    tracker.go               Cumulative stats tracker (defined, not yet wired in)
    display.go               Stats display helper (defined, not yet wired in)
  testmode/
    testmode.go              Interactive test mode with mock upstream + sample noisy request
configs/
  default_filters.yaml       Copy of embedded filter defaults
```

## Filter Pipeline

Applied in order to each `tool_result` text content:

1. **Strip ANSI** — remove color/formatting escape sequences
2. **Drop matching lines** — regex-based removal by category (npm, pip, cargo, git, docker, make, progress bars)
3. **Collapse passing tests** — replace N+ consecutive passing test lines with a summary
4. **Collapse blank lines** — reduce N+ consecutive blanks
5. **Collapse repeated lines** — reduce N+ identical consecutive lines

## Configuration

Loaded from `~/.config/tego/filters.yaml`, falling back to the embedded `default_filters.yaml`. Config controls: `strip_ansi`, `collapse_blank_lines`, `collapse_repeated_lines`, `collapse_passing_tests`, and a list of `drop_lines` rules (each with a `pattern` regex and `category` label).

---

## SUBJECT TO CHANGE

> Things we have implemented but want to rework. Each bullet describes what the change should be.

- Keeping this blank for now

## COMPLETE

> Features that are implemented and working. These do not need significant changes for now.

- The tool is a very baseline CLI tool which brings up a local server that does forward proxying at the moment 
- 
## TODO

> Features and improvements to work on next.

- **Deduplication**: Intelligently reduce repeated stuff or even things we can logically think are info that is able to be reduced and still get the big idea (i.e. a bunch of well known npm packages get installed in the CLI output, an LLM does not need every single line, we can reduce it to [x lines of installs], then give the LLM a command that would grab the actual text from a local SQLite database if they really need it for some reason). We would want to apply this to other situations too.

- **Observability**: Good observability in the form of persistent logs as well as a nice TUI (using Go's Bubble Tea library). Logs should store diffs of stripped info, estimated token saving, percent of text removed, etc. etc... The TUI dashboard can jsut show whats going on at hte time, stats and stuff. Store stats in a persistent store too, is SQLite good for the stats? Not sure
