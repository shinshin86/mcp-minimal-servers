/**
 * Simple MCP Server (No external libraries version)
 *
 * This server communicates with MCP clients (e.g., Cursor) via standard input/output (STDIN/STDOUT)
 * in JSON-RPC format, providing the minimum functionality of MCP.
 * 
 * This sample implements only one tool, "echo tool", which returns the string sent by the user.
 */

import * as readline from 'readline';

/** MCP tool output content type definition */
interface ToolContent {
  type: string;
  text?: string;
}

/** MCP tool basic structure */
interface MCPTool {
  name: string;             // Tool name
  description: string;      // Brief description of the tool
  inputSchema: object;      // JSON Schema for input parameters
  execute: (args: any) => ToolContent | ToolContent[];
}

/** Example: Echo tool - Returns the input string as is */
const echoTool: MCPTool = {
  name: "echo",
  description: "Returns the specified message as is",
  inputSchema: {
    type: "object",
    properties: {
      message: { type: "string", description: "The string to echo" }
    },
    required: ["message"]
  },
  execute: (args) => {
    const msg = args.message;
    return { type: "text", text: `Echo: ${msg}` };
  }
};

/** List of tools provided */
const tools: MCPTool[] = [ echoTool ];

/** Utility function to send JSON-RPC responses to standard output */
function sendResponse(obj: any) {
  process.stdout.write(JSON.stringify(obj) + "\n");
}

/** Utility function to send JSON-RPC error responses */
function sendError(id: any, code: number, message: string) {
  const errorResponse = {
    jsonrpc: "2.0",
    id: id,
    error: { code, message }
  };
  sendResponse(errorResponse);
}

/** Configuration for reading standard input (STDIN) line by line */
const rl = readline.createInterface({ input: process.stdin });

/**
 * Parses the JSON-RPC request received from standard input (STDIN)
 * and processes it according to the MCP protocol.
 */
rl.on('line', (data: string) => {
  if (!data.trim()) return;  // Ignore empty lines

  let request;
  try {
    request = JSON.parse(data);
  } catch (err) {
    // If JSON parsing fails, return -32700 (Parse error)
    sendError(null, -32700, "Parse error");
    return;
  }

  // Check if the JSON-RPC 2.0 format is met
  if (request.jsonrpc !== "2.0" || !request.method) {
    sendError(request.id ?? null, -32600, "Invalid Request");
    return;
  }

  const method = request.method;
  const id = request.id;
  const isNotification = (id === null || id === undefined);

  // 1. Initialize handshake (initialize → initialized)
  if (method === "initialize") {
    // Receive the protocol version from the client and return server information
    const clientProtocol = request.params?.protocolVersion;
    const protocolVersion = (typeof clientProtocol === "string") 
      ? clientProtocol 
      : "2025-03-08";  // Default or appropriate date and time

    const initResponse = {
      jsonrpc: "2.0",
      id: id,
      result: {
        protocolVersion: protocolVersion,
        serverInfo: {
          name: "simple-mcp-server",
          version: "0.1.0"
        },
        // Overview of the features provided by this server (supports tools)
        capabilities: {
          tools: {}
        }
      }
    };
    sendResponse(initResponse);
    return;
  }

  if (method === "initialized" || method === "notifications/initialized") {
    // The client receives the server's response and notifies that initialization is complete
    // In this case, no response is returned
    return;
  }

  // 2. (Optional) Cancel notification, etc.
  if (method === "cancelled") {
    // If the client cancels the tool call midway, for example
    // In this sample, nothing is done
    return;
  }

  // 3. Get tool list
  if (method === "tools/list") {
    // Return the list of tools currently registered on the server
    const toolList = tools.map(t => ({
      name: t.name,
      description: t.description,
      inputSchema: t.inputSchema
    }));
    const listResponse = {
      jsonrpc: "2.0",
      id: id,
      result: {
        tools: toolList
      }
    };
    sendResponse(listResponse);
    return;
  }

  // Resources and prompts are returned as empty lists
  if (method === "resources/list") {
    sendResponse({ jsonrpc: "2.0", id: id, result: { resources: [] } });
    return;
  }
  if (method === "prompts/list") {
    sendResponse({ jsonrpc: "2.0", id: id, result: { prompts: [] } });
    return;
  }

  // 4. Tool call request (tools/call)
  if (method === "tools/call") {
    const params = request.params;
    if (!params || typeof params !== "object") {
      sendError(id, -32602, "Invalid parameters");
      return;
    }
    const toolName = params.name;
    const args = params.arguments;
    if (typeof toolName !== "string" || args === undefined) {
      sendError(id, -32602, "Invalid parameters: missing tool name or arguments");
      return;
    }

    // Find the tool corresponding to the tool name
    const tool = tools.find(t => t.name === toolName);
    if (!tool) {
      sendError(id, -32601, `Method not found: tool '${toolName}' is not available`);
      return;
    }

    // Check if the tool's inputSchema has required fields
    const schema: any = tool.inputSchema;
    if (schema.required && Array.isArray(schema.required)) {
      for (const field of schema.required) {
        if (!(field in args)) {
          sendError(id, -32602, `Missing required parameter: '${field}'`);
          return;
        }
      }
    }

    // Call the tool's execute
    try {
      const resultContent = tool.execute(args);
      // On the MCP protocol, content must be returned as an array
      const contentArray: ToolContent[] = Array.isArray(resultContent) ? resultContent : [ resultContent ];
      // Return the result in JSON-RPC format
      const callResponse = {
        jsonrpc: "2.0",
        id: id,
        result: {
          content: contentArray
        }
      };
      sendResponse(callResponse);
    } catch (error) {
      // If an unexpected error occurs during tool execution
      sendError(id, -32603, "Internal error during tool execution");
    }
    return;
  }

  // If an unexpected method is called
  if (!isNotification) {
    sendError(id, -32601, `Method not found: ${method}`);
  }
});

/** If the MCP client disconnects, exit the process */
rl.on('close', () => {
  process.exit(0);
});