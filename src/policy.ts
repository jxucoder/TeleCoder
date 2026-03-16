import type {
  PermissionMode,
  TeleCoderConfig,
  TeleCoderPolicyMode,
  WorkspaceWritePolicy,
} from "./types.ts";

export interface ResolvedPolicy {
  effectivePermissionMode: PermissionMode;
  maxRuntimeSeconds: number;
  policyMode: TeleCoderPolicyMode;
  workspaceWritePolicy: WorkspaceWritePolicy;
}

interface PolicyDefaults {
  maxRuntimeSeconds: number;
  permissionMode: PermissionMode;
  workspaceWritePolicy: WorkspaceWritePolicy;
}

const POLICY_DEFAULTS: Record<TeleCoderPolicyMode, PolicyDefaults> = {
  locked: {
    permissionMode: "deny-all",
    workspaceWritePolicy: "blocked",
    maxRuntimeSeconds: 60,
  },
  observe: {
    permissionMode: "approve-reads",
    workspaceWritePolicy: "blocked",
    maxRuntimeSeconds: 180,
  },
  standard: {
    permissionMode: "approve-all",
    workspaceWritePolicy: "contained",
    maxRuntimeSeconds: 300,
  },
};

export function parsePolicyMode(raw: string | undefined): TeleCoderPolicyMode | undefined {
  switch (raw) {
    case undefined:
    case "":
      return undefined;
    case "locked":
    case "observe":
    case "standard":
      return raw;
    default:
      throw new Error(`Invalid TELECODER_POLICY_MODE: ${raw}`);
  }
}

export function inferPolicyModeFromPermission(permissionMode: PermissionMode): TeleCoderPolicyMode {
  switch (permissionMode) {
    case "deny-all":
      return "locked";
    case "approve-all":
      return "standard";
    case "approve-reads":
    default:
      return "observe";
  }
}

export function resolvePolicy(
  config: Pick<TeleCoderConfig, "defaultPolicyMode" | "promptTimeoutSeconds">,
  requestedMode?: TeleCoderPolicyMode,
): ResolvedPolicy {
  const policyMode = requestedMode ?? config.defaultPolicyMode;
  const defaults = POLICY_DEFAULTS[policyMode];

  return {
    policyMode,
    effectivePermissionMode: defaults.permissionMode,
    workspaceWritePolicy: defaults.workspaceWritePolicy,
    maxRuntimeSeconds: Math.min(defaults.maxRuntimeSeconds, config.promptTimeoutSeconds),
  };
}
