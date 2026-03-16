import { expect, test } from "bun:test";

import { buildAcpxCommand, extractAssistantText } from "../src/runtime/acpx.ts";

test("extractAssistantText keeps only agent message chunks", () => {
  const output = [
    '{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":1}}',
    '{"jsonrpc":"2.0","method":"session/update","params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"JSON"}}}}',
    '{"jsonrpc":"2.0","method":"session/update","params":{"update":{"sessionUpdate":"usage_update","used":10}}}',
    '{"jsonrpc":"2.0","method":"session/update","params":{"update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"_OK"}}}}',
    '{"jsonrpc":"2.0","id":3,"result":{"stopReason":"end_turn"}}',
  ].join("\n");

  expect(extractAssistantText(output)).toBe("JSON_OK");
});

test("buildAcpxCommand uses raw agent override when provided", () => {
  expect(
    buildAcpxCommand({
      acpxCommand: "/root/bin/acpx-local",
      agent: "claude",
      agentCommand: "/root/bin/claude-agent-acp-node22",
      cwd: "/tmp/repo",
      prompt: "hello",
      permissionMode: "approve-reads",
      timeoutSeconds: 45,
    }),
  ).toEqual([
    "/root/bin/acpx-local",
    "--cwd",
    "/tmp/repo",
    "--approve-reads",
    "--timeout",
    "45",
    "--format",
    "json",
    "--json-strict",
    "--agent",
    "/root/bin/claude-agent-acp-node22",
    "exec",
    "hello",
  ]);
});
