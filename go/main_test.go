package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// runTestInput is a helper function that feeds the input string to runMCPServer
// and returns all lines of output.
func runTestInput(t *testing.T, input string) []string {
	t.Helper()
	var out bytes.Buffer
	err := runMCPServer(strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("runMCPServer error: %v", err)
	}
	// Split the output by newlines
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	return lines
}

// jsonRPCBase represents the basic JSON-RPC response fields for quick checks.
type jsonRPCBase struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
}

// 1) Test the "initialize" method
func TestInitialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2023-10-10"},"id":1}`
	lines := runTestInput(t, input)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line output, got %d lines", len(lines))
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %v", resp["jsonrpc"])
	}
	if resp["id"] != float64(1) { // JSON numeric values become float64 by default
		t.Errorf("expected id=1, got %v", resp["id"])
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'result' to be an object, got %T", resp["result"])
	}
	if result["protocolVersion"] != "2023-10-10" {
		t.Errorf("expected protocolVersion=2023-10-10, got %v", result["protocolVersion"])
	}
}

// 2) Test the "tools/list" method
func TestToolsList(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"tools/list","id":2}`
	lines := runTestInput(t, input)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line output, got %d lines", len(lines))
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %v", resp["jsonrpc"])
	}
	if resp["id"] != float64(2) {
		t.Errorf("expected id=2, got %v", resp["id"])
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'result' to be an object, got %T", resp["result"])
	}

	toolsVal, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected 'tools' to be an array, got %T", result["tools"])
	}
	if len(toolsVal) != 1 {
		t.Errorf("expected 1 tool (echo), got %d", len(toolsVal))
	}
	toolObj, ok := toolsVal[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected tools[0] to be an object, got %T", toolsVal[0])
	}
	if toolObj["name"] != "echo" {
		t.Errorf("expected tool name=echo, got %v", toolObj["name"])
	}
}

// 3) Test calling the "echo" tool
func TestToolsCallEcho(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"echo","arguments":{"message":"Hello!"}},"id":3}`
	lines := runTestInput(t, input)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line output, got %d lines", len(lines))
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["id"] != float64(3) {
		t.Errorf("expected id=3, got %v", resp["id"])
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'result' to be an object, got %T", resp["result"])
	}
	content, ok := result["content"].([]interface{})
	if !ok {
		t.Fatalf("expected 'content' to be an array, got %T", result["content"])
	}
	if len(content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(content))
	}

	item, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected content[0] to be an object, got %T", content[0])
	}
	if item["type"] != "text" {
		t.Errorf("expected content[0].type = text, got %v", item["type"])
	}
	if item["text"] != "Echo: Hello!" {
		t.Errorf("expected content[0].text = 'Echo: Hello!', got %v", item["text"])
	}
}

// 4) Test error case: missing required argument "message"
func TestToolsCallEcho_MissingArgument(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"echo","arguments":{}},"id":4}`
	lines := runTestInput(t, input)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line output, got %d lines", len(lines))
	}

	var errResp JSONRPCErrorResponse
	if err := json.Unmarshal([]byte(lines[0]), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if errResp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc=2.0, got %v", errResp.JSONRPC)
	}
	if errResp.ID != float64(4) {
		t.Errorf("expected id=4, got %v", errResp.ID)
	}
	if errResp.Error.Code != -32602 {
		t.Errorf("expected code=-32602, got %d", errResp.Error.Code)
	}
	if !strings.Contains(errResp.Error.Message, "Missing required parameter") {
		t.Errorf("expected error message about missing parameter, got %v", errResp.Error.Message)
	}
}

// 5) Test error case: calling a tool that does not exist
func TestToolsCallUnknownTool(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"unknownTool","arguments":{"foo":"bar"}},"id":5}`
	lines := runTestInput(t, input)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line output, got %d lines", len(lines))
	}

	var errResp JSONRPCErrorResponse
	if err := json.Unmarshal([]byte(lines[0]), &errResp); err != nil {
		t.Fatalf("failed to unmarshal error response: %v", err)
	}

	if errResp.ID != float64(5) {
		t.Errorf("expected id=5, got %v", errResp.ID)
	}
	if errResp.Error.Code != -32601 {
		t.Errorf("expected code=-32601, got %d", errResp.Error.Code)
	}
}
