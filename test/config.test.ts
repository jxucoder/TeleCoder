import { expect, test } from "bun:test";
import { join } from "node:path";

import { loadConfig } from "../src/config.ts";

test("loadConfig applies Bun and acpx defaults", async () => {
  const config = await loadConfig({
    HOME: "/tmp/home",
    TELECODER_DATA_DIR: "/tmp/telecoder",
    TELECODER_AGENT_CLAUDE_COMMAND: "node22 claude-agent-acp",
    TELECODER_AGENT_OPENCODE_COMMAND: "/usr/local/bin/opencode acp",
  });

  expect(config.dataDir).toBe("/tmp/telecoder");
  expect(config.dbPath).toBe(join("/tmp/telecoder", "telecoder.sqlite"));
  expect(config.workspaceDir).toBe(join("/tmp/telecoder", "workspaces"));
  expect(config.acpxCommand).toBe("acpx");
  expect(config.agentCommands).toEqual({
    claude: "node22 claude-agent-acp",
    opencode: "/usr/local/bin/opencode acp",
  });
  expect(config.defaultAgent).toBe("codex");
  expect(config.defaultPolicyMode).toBe("observe");
  expect(config.permissionMode).toBe("approve-reads");
  expect(config.workspaceWritePolicy).toBe("blocked");
  expect(config.policyMaxRuntimeSeconds).toBe(180);
  expect(config.promptTimeoutSeconds).toBe(300);
  expect(config.sessionHeartbeatSeconds).toBe(5);
  expect(config.sessionStaleSeconds).toBe(30);
});

test("loadConfig infers policy mode from legacy permission mode", async () => {
  const config = await loadConfig({
    HOME: "/tmp/home",
    TELECODER_DATA_DIR: "/tmp/telecoder",
    TELECODER_PERMISSION_MODE: "approve-all",
    TELECODER_PROMPT_TIMEOUT_SECONDS: "240",
  });

  expect(config.defaultPolicyMode).toBe("standard");
  expect(config.permissionMode).toBe("approve-all");
  expect(config.workspaceWritePolicy).toBe("contained");
  expect(config.policyMaxRuntimeSeconds).toBe(240);
});

test("loadConfig lets explicit policy mode override legacy permission mode", async () => {
  const config = await loadConfig({
    HOME: "/tmp/home",
    TELECODER_DATA_DIR: "/tmp/telecoder",
    TELECODER_POLICY_MODE: "locked",
    TELECODER_PERMISSION_MODE: "approve-all",
  });

  expect(config.defaultPolicyMode).toBe("locked");
  expect(config.permissionMode).toBe("deny-all");
  expect(config.workspaceWritePolicy).toBe("blocked");
  expect(config.policyMaxRuntimeSeconds).toBe(60);
});

test("loadConfig rejects stale timeout shorter than heartbeat interval", async () => {
  await expect(
    loadConfig({
      HOME: "/tmp/home",
      TELECODER_DATA_DIR: "/tmp/telecoder",
      TELECODER_SESSION_HEARTBEAT_SECONDS: "10",
      TELECODER_SESSION_STALE_SECONDS: "5",
    }),
  ).rejects.toThrow(
    "TELECODER_SESSION_STALE_SECONDS must be greater than TELECODER_SESSION_HEARTBEAT_SECONDS",
  );
});

test("loadConfig rejects invalid policy mode", async () => {
  await expect(
    loadConfig({
      HOME: "/tmp/home",
      TELECODER_DATA_DIR: "/tmp/telecoder",
      TELECODER_POLICY_MODE: "unsafe",
    }),
  ).rejects.toThrow("Invalid TELECODER_POLICY_MODE: unsafe");
});
