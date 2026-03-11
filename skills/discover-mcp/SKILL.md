---
name: discover-mcp
description: Use when the user asks to discover available MCP tools/resources/servers, wants a recommendation for which MCP tool to use, or mentions an unfamiliar MCP tool/server that must be verified.
---

# Discover MCP

## Overview

Systematically enumerate MCP servers/resources in this environment and map them to the user's task before recommending a tool or next action.

## Workflow

### 1) Clarify discovery scope

- Determine whether the request targets:
  - Codex/MCP runtime tools (MCP resources available to this agent), or
  - One-MCP app services (configured services in this repo/runtime).
- If ambiguous, ask one concise question before enumerating.

### 2) Enumerate resources

- Call `list_mcp_resources` with no server filter to see all servers.
- For each relevant server, call `list_mcp_resources` with `server` to get full listings.
- Call `list_mcp_resource_templates` to surface parameterized resources.

Example calls:

```text
list_mcp_resources {}
list_mcp_resources {"server":"memorix"}
list_mcp_resource_templates {"server":"memorix"}
```

### 3) Inspect candidates

- Use `read_mcp_resource` on promising URIs to capture schema, usage, or examples.

Example call:

```text
read_mcp_resource {"server":"memorix","uri":"<resource-uri>"}
```

### 4) Recommend and confirm

- List 2–5 relevant tools/resources with a one-line purpose each.
- Propose a first call and ask for any missing inputs.
- If no tools are found, say so and suggest how to connect/install.

## Output Checklist

- "What exists" (servers + resources/templates)
- "What fits" (short mapping to the task)
- "Next call" (a concrete tool-call suggestion)

## Guardrails

- Do not guess tool availability; if none found, report it plainly.
- Prefer MCP introspection over web search; use web.run only when the user asks or external info is required.
- If the request targets One-MCP app services (not agent MCP tools), inspect project config/data rather than assuming it matches the runtime tool list.
- Follow any project-level MCP/skill sync rules in `AGENTS.md` before suggesting sync actions.
