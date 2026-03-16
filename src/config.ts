import { mkdir } from "node:fs/promises";
import { homedir } from "node:os";
import { join, resolve } from "node:path";

import type { PermissionMode, TeleCoderConfig } from "./types.ts";
import { inferPolicyModeFromPermission, parsePolicyMode, resolvePolicy } from "./policy.ts";

function parsePort(raw: string | undefined, fallback: number): number {
  if (!raw) {
    return fallback;
  }
  const port = Number.parseInt(raw, 10);
  if (!Number.isFinite(port) || port <= 0) {
    throw new Error(`Invalid TELECODER_LISTEN_PORT: ${raw}`);
  }
  return port;
}

function parsePositiveInt(name: string, raw: string | undefined, fallback: number): number {
  if (!raw) {
    return fallback;
  }
  const value = Number.parseInt(raw, 10);
  if (!Number.isFinite(value) || value <= 0) {
    throw new Error(`Invalid ${name}: ${raw}`);
  }
  return value;
}

function parsePermissionMode(raw: string | undefined): PermissionMode {
  switch (raw) {
    case undefined:
    case "":
      return "approve-reads";
    case "approve-all":
    case "approve-reads":
    case "deny-all":
      return raw;
    default:
      throw new Error(`Invalid TELECODER_PERMISSION_MODE: ${raw}`);
  }
}

function parseAgentCommands(
  env: Record<string, string | undefined>,
): Record<string, string> {
  const commands: Record<string, string> = {};

  for (const [key, value] of Object.entries(env)) {
    const match = key.match(/^TELECODER_AGENT_([A-Z0-9_]+)_COMMAND$/);
    if (!match || !value) {
      continue;
    }

    const agent = match[1]!.toLowerCase().replaceAll("_", "-");
    const command = value.trim();
    if (!command) {
      continue;
    }
    commands[agent] = command;
  }

  return commands;
}

export async function loadConfig(
  env: Record<string, string | undefined> = process.env,
): Promise<TeleCoderConfig> {
  const dataDir = resolve(env.TELECODER_DATA_DIR ?? join(homedir(), ".telecoder"));
  const workspaceDir = join(dataDir, "workspaces");
  const promptTimeoutSeconds = parsePositiveInt(
    "TELECODER_PROMPT_TIMEOUT_SECONDS",
    env.TELECODER_PROMPT_TIMEOUT_SECONDS,
    300,
  );
  const sessionHeartbeatSeconds = parsePositiveInt(
    "TELECODER_SESSION_HEARTBEAT_SECONDS",
    env.TELECODER_SESSION_HEARTBEAT_SECONDS,
    5,
  );
  const sessionStaleSeconds = parsePositiveInt(
    "TELECODER_SESSION_STALE_SECONDS",
    env.TELECODER_SESSION_STALE_SECONDS,
    30,
  );

  if (sessionStaleSeconds <= sessionHeartbeatSeconds) {
    throw new Error(
      "TELECODER_SESSION_STALE_SECONDS must be greater than TELECODER_SESSION_HEARTBEAT_SECONDS",
    );
  }

  const explicitPolicyMode = parsePolicyMode(env.TELECODER_POLICY_MODE);
  const legacyPermissionMode = parsePermissionMode(env.TELECODER_PERMISSION_MODE);
  const defaultPolicyMode =
    explicitPolicyMode ?? inferPolicyModeFromPermission(legacyPermissionMode);
  const defaultPolicy = resolvePolicy(
    {
      defaultPolicyMode,
      promptTimeoutSeconds,
    },
    defaultPolicyMode,
  );

  await mkdir(workspaceDir, { recursive: true });

  return {
    dataDir,
    dbPath: join(dataDir, "telecoder.sqlite"),
    workspaceDir,
    listenHost: env.TELECODER_LISTEN_HOST ?? "127.0.0.1",
    listenPort: parsePort(env.TELECODER_LISTEN_PORT, 7080),
    acpxCommand: env.TELECODER_ACPX_COMMAND ?? "acpx",
    agentCommands: parseAgentCommands(env),
    defaultAgent: env.TELECODER_DEFAULT_AGENT ?? "codex",
    defaultPolicyMode,
    permissionMode: defaultPolicy.effectivePermissionMode,
    workspaceWritePolicy: defaultPolicy.workspaceWritePolicy,
    policyMaxRuntimeSeconds: defaultPolicy.maxRuntimeSeconds,
    promptTimeoutSeconds,
    sessionHeartbeatSeconds,
    sessionStaleSeconds,
  };
}
