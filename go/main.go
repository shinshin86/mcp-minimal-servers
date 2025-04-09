package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// ToolContent represents the content returned by an MCP tool.
type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// MCPTool defines the interface that a tool must implement.
type MCPTool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(args map[string]interface{}) ([]ToolContent, error)
}

// echoTool is equivalent to the "echo" tool in the TypeScript sample.
type echoTool struct{}

// Name returns the name of the echo tool.
func (e *echoTool) Name() string {
	return "echo"
}

// Description returns a brief description of the echo tool.
func (e *echoTool) Description() string {
	return "Returns the specified message as is"
}

// InputSchema returns the JSON schema for the echo tool's input parameters.
func (e *echoTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"message": map[string]interface{}{
				"type":        "string",
				"description": "The string to echo",
			},
		},
		"required": []string{"message"},
	}
}

// Execute performs the actual echo operation based on the given arguments.
func (e *echoTool) Execute(args map[string]interface{}) ([]ToolContent, error) {
	msg, ok := args["message"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid type for 'message'")
	}
	content := ToolContent{
		Type: "text",
		Text: fmt.Sprintf("Echo: %s", msg),
	}
	return []ToolContent{content}, nil
}

// tools is a list of available tools.
var tools = []MCPTool{
	&echoTool{},
}

// JSONRPCRequest represents a generic JSON-RPC request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id,omitempty"`
}

// JSONRPCError represents the "error" field of a JSON-RPC response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSONRPCErrorResponse represents a JSON-RPC error response object.
type JSONRPCErrorResponse struct {
	JSONRPC string       `json:"jsonrpc"`
	ID      interface{}  `json:"id"`
	Error   JSONRPCError `json:"error"`
}

// sendResponse writes a JSON-RPC result response to the given writer.
func sendResponse(w io.Writer, response interface{}) {
	bytes, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(w, "Failed to marshal response: %v\n", err)
		return
	}
	fmt.Fprintf(w, "%s\n", string(bytes))
}

// sendError writes a JSON-RPC error response to the given writer.
func sendError(w io.Writer, id interface{}, code int, message string) {
	errResp := JSONRPCErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	sendResponse(w, errResp)
}

// toolsCallParams holds the parameters expected by "tools/call".
type toolsCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// runMCPServer reads JSON-RPC requests from r and writes responses to w.
func runMCPServer(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			// Parse error: -32700
			sendError(w, nil, -32700, "Parse error")
			continue
		}

		if req.JSONRPC != "2.0" || req.Method == "" {
			sendError(w, req.ID, -32600, "Invalid Request")
			continue
		}

		method := req.Method
		id := req.ID
		isNotification := (id == nil)

		switch method {
		case "initialize":
			// Example: parse protocolVersion and respond with initialization info
			var params map[string]interface{}
			_ = json.Unmarshal(req.Params, &params)
			clientProtocol, _ := params["protocolVersion"].(string)
			protocolVersion := clientProtocol
			if protocolVersion == "" {
				protocolVersion = "2025-03-08"
			}

			initResponse := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"protocolVersion": protocolVersion,
					"serverInfo": map[string]string{
						"name":    "simple-mcp-server",
						"version": "0.1.0",
					},
					"capabilities": map[string]interface{}{
						"tools": map[string]interface{}{},
					},
				},
			}
			sendResponse(w, initResponse)

		case "initialized", "notifications/initialized":
			// No response
			continue

		case "cancelled":
			// No specific handling
			continue

		case "tools/list":
			// Return the list of tools
			toolList := make([]map[string]interface{}, 0, len(tools))
			for _, t := range tools {
				toolList = append(toolList, map[string]interface{}{
					"name":        t.Name(),
					"description": t.Description(),
					"inputSchema": t.InputSchema(),
				})
			}
			listResp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"tools": toolList,
				},
			}
			sendResponse(w, listResp)

		case "resources/list":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"resources": []interface{}{},
				},
			}
			sendResponse(w, resp)

		case "prompts/list":
			resp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"prompts": []interface{}{},
				},
			}
			sendResponse(w, resp)

		case "tools/call":
			var params toolsCallParams
			if err := json.Unmarshal(req.Params, &params); err != nil {
				sendError(w, id, -32602, "Invalid parameters")
				continue
			}
			if params.Name == "" || params.Arguments == nil {
				sendError(w, id, -32602, "Invalid parameters: missing tool name or arguments")
				continue
			}

			// Search for the tool
			var foundTool MCPTool
			for _, t := range tools {
				if t.Name() == params.Name {
					foundTool = t
					break
				}
			}
			if foundTool == nil {
				sendError(w, id, -32601, fmt.Sprintf("Method not found: tool '%s' is not available", params.Name))
				continue
			}

			// Validate required fields
			schema := foundTool.InputSchema()
			required, _ := schema["required"].([]string)
			missingParam := false
			for _, field := range required {
				if _, ok := params.Arguments[field]; !ok {
					sendError(w, id, -32602, fmt.Sprintf("Missing required parameter: '%s'", field))
					missingParam = true
					break
				}
			}
			if missingParam {
				// Stop processing this request
				continue
			}

			// Execute the tool
			resultContent, err := foundTool.Execute(params.Arguments)
			if err != nil {
				sendError(w, id, -32603, "Internal error during tool execution")
				continue
			}

			// Return success response
			callResp := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      id,
				"result": map[string]interface{}{
					"content": resultContent,
				},
			}
			sendResponse(w, callResp)

		default:
			if !isNotification {
				sendError(w, id, -32601, fmt.Sprintf("Method not found: %s", method))
			}
		}
	}

	return scanner.Err()
}

// main uses standard input/output for the MCP server.
func main() {
	if err := runMCPServer(os.Stdin, os.Stdout); err != nil {
		os.Exit(1)
	}
}
