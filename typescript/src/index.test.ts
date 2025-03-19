import { describe, test, expect, beforeAll, afterAll } from 'vitest';
import { spawn, ChildProcessWithoutNullStreams } from 'child_process';

function sendRequest(
  serverProc: ChildProcessWithoutNullStreams,
  requestObj: any
): Promise<any> {
  return new Promise((resolve, reject) => {
    const requestStr = JSON.stringify(requestObj) + "\n";
    let buffer = "";

    console.log(`send request: ${requestStr.trim()}`);

    const onData = (data: Buffer) => {
      const chunk = data.toString();
      console.log(`received response from server: ${chunk}`);
      buffer += chunk;

      // Assume that only the first line is received in this case
      // (If the server returns multiple lines at once, adjustments are needed)
      if (buffer.includes("\n")) {
        serverProc.stdout.off("data", onData);
        try {
          const lines = buffer.trim().split("\n");
          const response = JSON.parse(lines[0]);
          console.log(`parsed response: ${JSON.stringify(response)}`);
          resolve(response);
        } catch (err) {
          console.error(`parse error: ${err}`);
          reject(err);
        }
      }
    };

    serverProc.stdout.on("data", onData);
    serverProc.stdin.write(requestStr);
  });
}

describe("Simple MCP Server (TypeScript)", () => {
  let serverProc: ChildProcessWithoutNullStreams;

  // Start the server process before the tests
  beforeAll(() => {
    serverProc = spawn("node", ["./build/index.js"], {
      cwd: process.cwd(),
      stdio: ["pipe", "pipe", "pipe"],
    });

    // Add a listener to check the server's error output
    serverProc.stderr.on("data", (data) => {
      console.error(`server error: ${data}`);
    });

    // Detect the server process's exit
    serverProc.on("exit", (code) => {
      console.log(`server process exited. code: ${code}`);
    });
  });

  // End the server process after the tests
  afterAll(() => {
    serverProc.kill();
  });

  test("initialize (handshake)", async () => {
    const request = {
      jsonrpc: "2.0",
      id: 1,
      method: "initialize",
      params: {
        protocolVersion: "2025-03-08",
      },
    };
    const response = await sendRequest(serverProc, request);
    expect(response.jsonrpc).toBe("2.0");
    expect(response.id).toBe(1);
    expect(response.result.protocolVersion).toBe("2025-03-08");
    expect(response.result.serverInfo.name).toBe("simple-mcp-server");
  }, 10000);

  test("tools/list", async () => {
    const request = {
      jsonrpc: "2.0",
      id: 2,
      method: "tools/list",
      params: {},
    };
    const response = await sendRequest(serverProc, request);

    expect(response.jsonrpc).toBe("2.0");
    expect(response.id).toBe(2);
    expect(Array.isArray(response.result.tools)).toBe(true);
    // Check if the echo tool is included
    const echoToolInfo = response.result.tools.find((t: any) => t.name === "echo");
    expect(echoToolInfo).toBeDefined();
    expect(echoToolInfo.description).toBe("Returns the specified message as is");
  }, 10000);

  test("tools/call (echo)", async () => {
    const request = {
      jsonrpc: "2.0",
      id: 3,
      method: "tools/call",
      params: {
        name: "echo",
        arguments: { message: "Hello MCP!" }
      },
    };
    const response = await sendRequest(serverProc, request);

    expect(response.jsonrpc).toBe("2.0");
    expect(response.id).toBe(3);
    expect(response.result.content).toEqual([
      {
        type: "text",
        text: "Echo: Hello MCP!",
      },
    ]);
  }, 10000);

  test("invalid JSON -> Parse error", async () => {
    // Send an invalid JSON string
    serverProc.stdin.write("INVALID_JSON\n");

    // Wait for the reception (do not use the same mechanism as sendRequest)
    // In this case, assume that one line is returned
    const data = await new Promise<string>((resolve) => {
      serverProc.stdout.once("data", (buf) => {
        resolve(buf.toString().trim());
      });
    });

    const response = JSON.parse(data);
    expect(response.error.code).toBe(-32700);
    expect(response.error.message).toBe("Parse error");
  }, 10000);
});
