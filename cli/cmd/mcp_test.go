package cmd

import (
	"encoding/json"
	"testing"
)

func TestMCPToolsList(t *testing.T) {
	tools := getMCPTools()
	if len(tools) < 5 {
		t.Fatalf("expected at least 5 MCP tools, got %d", len(tools))
	}

	toolMap := make(map[string]bool)
	for _, tool := range tools {
		toolMap[tool.Name] = true
	}

	expected := []string{
		"nextdeploy_inspect",
		"nextdeploy_plan",
		"nextdeploy_ship",
		"nextdeploy_status",
		"nextdeploy_trace",
		"nextdeploy_takeover",
		"nextdeploy_rollback",
	}

	for _, name := range expected {
		if !toolMap[name] {
			t.Errorf("missing expected MCP tool: %s", name)
		}
	}
}

func TestMCPInitializeRequest(t *testing.T) {
	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	var req rpcRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("failed to unmarshal JSON-RPC request: %v", err)
	}

	if req.Method != "initialize" {
		t.Errorf("expected method initialize, got %s", req.Method)
	}
}

func TestMCPToolCallInspect(t *testing.T) {
	reqJSON := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"nextdeploy_status","arguments":{}}}`
	var req rpcRequest
	if err := json.Unmarshal([]byte(reqJSON), &req); err != nil {
		t.Fatalf("failed to unmarshal JSON-RPC tool call: %v", err)
	}

	if req.Method != "tools/call" {
		t.Errorf("expected method tools/call, got %s", req.Method)
	}
}
