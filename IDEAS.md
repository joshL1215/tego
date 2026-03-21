# Ideas

## Deduplication Storage

SQLite with content-addressable hashing and TTL-based cleanup.

- **Hash as key**: SHA256 (or similar) of the omitted text block serves as the primary key. Deterministic IDs, natural dedup if the same output appears twice.
- **TTL cleanup**: Delete rows older than a configurable window (e.g. 24h) so the DB doesn't grow forever.
- **Primary value is within-session dedup**, not cross-session caching. Exact text matches across sessions are rare in practice (versions, timestamps, ordering all vary).
- **Retrieval**: CLI only (`tego retrieve <id>`). LLMs like Claude Code already know they have bash access, so embedding a shell command in the summary text is zero-cost. No need for an HTTP endpoint or injecting tool definitions — both would waste context. The omitted block just says something like `[187 lines omitted — run 'tego retrieve abc123' to view]`.
- **Why not in-memory only**: Proxy restart mid-session would make omitted text unrecoverable. SQLite is basically free and gives crash resilience.
- **Why not cross-session caching as a feature**: Not worth the complexity for how rarely exact matches would occur. The hash still provides it for free if it does happen.

## Deduplication Detection Approach

Layered: rule-based first, heuristic fallback second.

### Rule-based (primary)

Extend the existing category-based regex infrastructure with "dedup rules" that recognize known verbose output shapes:
- npm install listings (`added package@version`)
- pip install lines
- `ls`/`find` output (lines that are just file paths)
- Compiler warnings following a consistent format (gcc/clang)
- Stack trace frames
- Verbose git log/diff output

High-confidence, cheap to match, and enables good summary lines ("187 npm installs omitted" is more useful than "187 similar lines omitted").

### Heuristic fallback

For stuff the rules don't catch. Two levels:

1. **Exact/near-exact repeat detection** — already exists (collapse repeated lines), but could lower the threshold for longer blocks.
2. **Prefix/pattern clustering** — if N+ consecutive lines share the same first N characters or match the same structural regex (e.g., `^\s+\S+@\d`), treat them as a collapsible group. Catches the long tail without needing a rule for every tool's output format.

Rules run first, heuristic catches what's left. Keeps it fast and predictable.
