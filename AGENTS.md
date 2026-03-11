# Repository Guidelines

## Project Structure & Module Organization
The Go backend lives under `backend/` with request handlers in `api/handler`, shared utilities in `common/`, data access in `data/`, and business logic in `service/`. The React frontend sits in `frontend/src` with translations in `frontend/public/locales` and build artifacts generated into `frontend/dist`. Persistent assets such as the SQLite database and uploads are stored in `data/` and `upload/`, while deployment aides live in `deploy/`, `Dockerfile`, and `docker-compose.yaml`.

## Build, Test, and Development Commands
- `./run.sh` — launches the backend on `:3000` and the Vite dev server on `:5173` with hot reload.
- `PORT=8080 ./build.sh` — produces a production binary and bundles the frontend.
- `go test ./...` — executes the Go unit tests across the backend.
- `cd frontend && npm run build` — type-checks, lints, and compiles the React app.

## Coding Style & Naming Conventions
Go code must pass `gofmt` (tabs for indentation) and follow idiomatic package naming (`lower_case` for directories, `CamelCase` for exported types). Keep API handlers in `backend/api/handler` named `*_handler.go` and tests as `*_test.go`. TypeScript and JSX files use two-space indentation, TypeScript strict mode, and Tailwind utility ordering as emitted by `shadcn`. Run `npm run lint` before pushing to ensure ESLint (flat config) passes.

## Testing Guidelines
Backend features require table-driven tests under matching `*_test.go` files; ensure new logic is covered by `go test ./...` and update `coverage.out` when reporting coverage. Frontend components should use Vitest with Testing Library—add specs under `frontend/src/**/__tests__/` or alongside components with `.test.tsx` suffix. For UI flows, prefer `npm run test:coverage` to exercise V8 coverage and attach results in PRs touching critical paths.

## Commit & Pull Request Guidelines
Follow the Conventional Commits style visible in `git log` (e.g., `feat(proxy): add SSE support`). Keep messages scoped, written in the imperative, and limited to 72 characters in the subject. Pull requests should link related issues, summarize behavior changes, note migrations or env needs, and include before/after screenshots for UI updates. Confirm both `go test ./...` and `npm run test` results in the PR description before requesting review.

## Environment & Configuration Tips
Copy `.env_example` to `.env` and override only the keys you touch; avoid committing secrets. SQLite state is persisted in `data/one-mcp.db`, so remove it if you need a clean slate. When integrating external services, prefer storing credentials in `.env` and referencing them via `config/` structs rather than hardcoding values.


# Memorix — Automatic Memory Rules

You have access to Memorix memory tools. Follow these rules to maintain persistent context across sessions.

## RULE 1: Session Start — Load Context

At the **beginning of every conversation**, BEFORE responding to the user:

1. Call `memorix_session_start` to get the previous session summary and key memories (this is a direct read, not a search — no fragmentation risk)
2. Then call `memorix_search` with a query related to the user's first message for additional context
3. If search results are found, use `memorix_detail` to fetch the most relevant ones
4. Reference relevant memories naturally — the user should feel you "remember" them

## RULE 2: Store Important Context

**Proactively** call `memorix_store` when any of the following happen:

### What MUST be recorded:
- Architecture/design decisions → type: `decision`
- Bug identified and fixed → type: `problem-solution`
- Unexpected behavior or gotcha → type: `gotcha`
- Config changed (env vars, ports, deps) → type: `what-changed`
- Feature completed or milestone → type: `what-changed`
- Trade-off discussed with conclusion → type: `trade-off`

### What should NOT be recorded:
- Simple file reads, greetings, trivial commands (ls, pwd, git status)

### Use topicKey for evolving topics:
For decisions, architecture docs, or any topic that evolves over time, ALWAYS use `topicKey` parameter.
This ensures the memory is UPDATED instead of creating duplicates.
Use `memorix_suggest_topic_key` to generate a stable key.

Example: `topicKey: "architecture/auth-model"` — subsequent stores with the same key update the existing memory.

### Track progress with the progress parameter:
When working on features or tasks, include the `progress` parameter:
```json
{
  "progress": {
    "feature": "user authentication",
    "status": "in-progress",
    "completion": 60
  }
}
```
Status values: `in-progress`, `completed`, `blocked`

## RULE 3: Resolve Completed Memories

When a task is completed, a bug is fixed, or information becomes outdated:

1. Call `memorix_resolve` with the observation IDs to mark them as resolved
2. Resolved memories are hidden from default search, preventing context pollution

This is critical — without resolving, old bug reports and completed tasks will keep appearing in future searches.

## RULE 4: Session End — Store Decision Chain Summary

When the conversation is ending, create a **decision chain summary** (not just a checklist):

1. Call `memorix_store` with type `session-request` and `topicKey: "session/latest-summary"`:

   **Required structure:**
   ```
   ## Goal
   [What we were working on — specific, not vague]

   ## Key Decisions & Reasoning
   - Chose X because Y. Rejected Z because [reason].
   - [Every architectural/design decision with WHY]

   ## What Changed
   - [File path] — [what changed and why]

   ## Current State
   - [What works now, what's pending]
   - [Any blockers or risks]

   ## Next Steps
   - [Concrete next actions, in priority order]
   ```

   **Critical: Include the "Key Decisions & Reasoning" section.** Without it, the next AI session will lack the context to understand WHY things were done a certain way and may suggest conflicting approaches.

2. Call `memorix_resolve` on any memories for tasks completed in this session

## RULE 5: Compact Awareness

Memorix automatically compacts memories on store:
- **With LLM API configured:** Smart dedup — extracts facts, compares with existing, merges or skips duplicates
- **Without LLM (free mode):** Heuristic dedup — uses similarity scores to detect and merge duplicate memories
- **You don't need to manually deduplicate.** Just store naturally and compact handles the rest.
- If you notice excessive duplicate memories, call `memorix_deduplicate` for batch cleanup.

## Guidelines

- **Use concise titles** (~5-10 words) and structured facts
- **Include file paths** in filesModified when relevant
- **Include related concepts** for better searchability
- **Always use topicKey** for recurring topics to prevent duplicates
- **Always resolve** completed tasks and fixed bugs
- **Always include reasoning** — "chose X because Y" is 10x more valuable than "did X"
- Search defaults to `status="active"` — use `status="all"` to include resolved memories
