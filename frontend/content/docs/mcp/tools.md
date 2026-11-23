---
title: MCP Tool Schemas & Parameter Contracts
lede: Formal JSON Schema definitions and parameter contracts for all 8 NextDeploy MCP tools.
status: ready
sourceRef: shared/agenttrace/, cli/cmd/
related:
  - title: MCP Server Overview
    href: /docs/mcp/server
  - title: Agent Rules & Prompts
    href: /docs/mcp/prompt-engineering
  - title: CLI Reference
    href: /docs/cli/reference
---

## Tool Directory

The NextDeploy MCP server exposes 8 primary tools to connected LLMs and agents:

1. `nextdeploy_inspect`: Static analysis of routes, dependencies, and integrations.
2. `nextdeploy_plan`: Dry-run infrastructure drift and validation calculation.
3. `nextdeploy_ship`: Build, compile, deploy, and verify live endpoints.
4. `nextdeploy_status`: Live deployment health, active release ID, and probe checks.
5. `nextdeploy_logs`: Tail recent serverless or VPS daemon logs.
6. `nextdeploy_secrets`: Encrypted secret read, write, and key rotation.
7. `nextdeploy_trace`: Checkpoint agent mental models and update takeover briefs.
8. `nextdeploy_rollback`: Atomic sub-second traffic reversal.

---

## Tool Schemas

### 1. `nextdeploy_inspect`
Extracts Next.js version, output mode, route classification, and external integrations from compiled manifests.

```json
{
  "name": "nextdeploy_inspect",
  "description": "Statically inspects Next.js compilation manifests to extract classified routes, Server Actions, middleware matchers, and external integrations (Stripe, Prisma, Supabase).",
  "inputSchema": {
    "type": "object",
    "properties": {
      "project_dir": {
        "type": "string",
        "description": "Path to the Next.js project directory (defaults to current working directory)."
      }
    }
  }
}
```

---

### 2. `nextdeploy_plan`
Calculates an execution diff showing what resources would be created, updated, or deleted without applying changes.

```json
{
  "name": "nextdeploy_plan",
  "description": "Calculates a Terraform-style dry-run execution plan showing infrastructure changes and drift.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "refresh": {
        "type": "boolean",
        "description": "Force re-fetching remote cloud state before calculating drift."
      },
      "only": {
        "type": "string",
        "description": "Limit plan evaluation to specific modules (e.g. 'd1,r2,dns')."
      }
    }
  }
}
```

---

### 3. `nextdeploy_ship`
Executes end-to-end compilation, static asset sync, compute provisioning, health probing, and atomic cutover.

```json
{
  "name": "nextdeploy_ship",
  "description": "Builds and deploys the Next.js application to the configured target (VPS, Cloudflare Workers, or AWS Lambda).",
  "inputSchema": {
    "type": "object",
    "properties": {
      "environment": {
        "type": "string",
        "description": "Deploy to an isolated named environment (e.g. 'staging', 'pr-42')."
      },
      "verify": {
        "type": "boolean",
        "description": "Enforce strict post-deploy HTTP smoke checks; rolls back automatically on failure (defaults to true for agents)."
      },
      "force": {
        "type": "boolean",
        "description": "Rebuild and deploy even if git working tree is dirty or cache matches."
      }
    }
  }
}
```

---

### 4. `nextdeploy_trace`
Snapshots the agent's mental model, intent, and git diffs into `.nextdeploy/trace/checkpoint.json` and updates `takeover.md`.

```json
{
  "name": "nextdeploy_trace",
  "description": "Records an agent engineering checkpoint (prompts, reasoning steps, modified files, and remaining unfinished work) into the local continuity layer.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "objective": {
        "type": "string",
        "description": "Current engineering intent or goal of the session."
      },
      "agent": {
        "type": "string",
        "description": "Name of the AI coding agent (e.g. 'cursor', 'claude-code', 'antigravity')."
      },
      "model": {
        "type": "string",
        "description": "Active model identifier (e.g. 'claude-3-7-sonnet', 'gemini-3.8-flash')."
      },
      "unfinished_work": {
        "type": "string",
        "description": "Summary of remaining tasks or blocking issues for human takeover."
      }
    },
    "required": ["objective"]
  }
}
```

---

### 5. `nextdeploy_rollback`
Atomically reverts production traffic to the previous active release in under 1 second.

```json
{
  "name": "nextdeploy_rollback",
  "description": "Instantly reverts production traffic to a previous release without rebuilding.",
  "inputSchema": {
    "type": "object",
    "properties": {
      "steps": {
        "type": "integer",
        "description": "Number of releases to roll back (defaults to 1)."
      },
      "release_id": {
        "type": "string",
        "description": "Specific timestamp release identifier to restore."
      }
    }
  }
}
```
