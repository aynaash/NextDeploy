---
title: Model Context Protocol (MCP) Server
lede: Connecting NextDeploy directly to AI coding agents via the open Model Context Protocol over stdio JSON-RPC.
status: ready
sourceRef: shared/agenttrace/, cli/cmd/
related:
  - title: MCP Tool Schemas
    href: /docs/mcp/tools
  - title: Agent Rules & Prompts
    href: /docs/mcp/prompt-engineering
  - title: Agent-First Infrastructure
    href: /docs/agent-trace/agent-first-infra
---

## What is MCP?

The **Model Context Protocol (MCP)** is an open standard that enables AI models and autonomous coding agents (Claude Desktop, Cursor, Antigravity IDE, Windsurf, Roo Code, Goose) to securely connect to external tools, databases, and infrastructure.

Instead of requiring an LLM to guess CLI arguments or scrape terminal ANSI colors using raw bash subshells, an MCP server provides **strongly-typed, schema-validated tool definitions**.

NextDeploy runs natively as an MCP server via:
```bash
nextdeploy mcp
```

### Auto-Scaffolding MCP Client Configs
NextDeploy can automatically generate the correct MCP JSON configuration files for your editor:

```bash
# Configure Cursor (.cursor/mcp.json)
nextdeploy mcp init --client cursor

# Configure Google Antigravity IDE (.antigravity/mcp_servers.json)
nextdeploy mcp init --client antigravity

# Configure all supported environments
nextdeploy mcp init --client all
```

---

## Architecture: Agent to NextDeploy over MCP

```mermaid
flowchart LR
    subgraph ClientEnv ["AI Client Environment"]
        Agent["Coding Agent (Cursor / Claude / Antigravity)"]
    end

    subgraph TransportLayer ["MCP Transport Layer"]
        Agent <-->|"stdio JSON-RPC 2.0"| Server["NextDeploy MCP Server"]
    end

    subgraph NativeEngine ["NextDeploy Native Engine"]
        Server --> Tool_Inspect["Tool: nextdeploy_inspect"]
        Server --> Tool_Plan["Tool: nextdeploy_plan"]
        Server --> Tool_Ship["Tool: nextdeploy_ship"]
        Server --> Tool_Trace["Tool: nextdeploy_trace"]
        Server --> Tool_Takeover["Tool: nextdeploy_takeover"]
    end

    subgraph CloudEdge ["Cloud & Edge"]
        Tool_Ship --> VPS["Linux VPS (nextdeployd)"]
        Tool_Ship --> CF["Cloudflare Workers (nextcompile)"]
        Tool_Ship --> AWS["AWS Lambda + CloudFront"]
    end
```

---

## Client Configuration

### 1. Cursor IDE (`.cursor/mcp.json`)
In your project root, create `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "nextdeploy": {
      "command": "nextdeploy",
      "args": ["mcp"],
      "env": {
        "PATH": "/usr/local/bin:/opt/homebrew/bin:~/.local/bin:$PATH"
      }
    }
  }
}
```

### 2. Claude Desktop (`claude_desktop_config.json`)
On macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "nextdeploy": {
      "command": "nextdeploy",
      "args": ["mcp"],
      "env": {
        "CLOUDFLARE_API_TOKEN": "your-token-here",
        "CLOUDFLARE_ACCOUNT_ID": "your-account-id-here"
      }
    }
  }
}
```

### 3. Google Antigravity IDE
In `.antigravity/mcp_servers.json`:

```json
{
  "mcpServers": {
    "nextdeploy": {
      "command": "nextdeploy",
      "args": ["mcp"]
    }
  }
}
```

---

## Why MCP Outperforms Raw Bash Execution

| Capability | Raw Bash Subshell (`run_command`) | NextDeploy Native MCP |
| :--- | :--- | :--- |
| **Parameter Validation** | None (prone to syntax errors, shell escaping bugs) | Strongly-typed JSON schema validation |
| **Interactive Prompts** | Hangs headless agent processes indefinitely | Fully non-interactive by protocol design |
| **Error Handling** | Unstructured regex scraping of ANSI terminal output | Structured JSON error envelopes with `culprit_file` and `fix_hint` |
| **Security** | Arbitrary shell execution risk | Scoped, permission-gated infrastructure operations |
| **Context Consumption** | 2,000 lines of terminal logs consuming tokens | Concise structured telemetry responses |
