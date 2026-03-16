import { mkdir } from "node:fs/promises";
import { existsSync } from "node:fs";
import { join, resolve } from "node:path";

import { runCommand } from "./process.ts";
import type { WorkspaceInfo } from "./types.ts";

function cloneSource(repo: string): string {
  if (existsSync(repo)) {
    return resolve(repo);
  }
  return repo;
}

export async function prepareWorkspace(
  workspaceRoot: string,
  sessionId: string,
  repo: string,
): Promise<WorkspaceInfo> {
  await mkdir(workspaceRoot, { recursive: true });

  const workDir = join(workspaceRoot, sessionId);
  const branch = `telecoder/${sessionId}`;

  const cloneResult = await runCommand(["git", "clone", cloneSource(repo), workDir]);
  if (cloneResult.exitCode !== 0) {
    throw new Error(cloneResult.stderr || cloneResult.stdout || "git clone failed");
  }

  const checkoutResult = await runCommand(["git", "checkout", "-b", branch], { cwd: workDir });
  if (checkoutResult.exitCode !== 0) {
    throw new Error(checkoutResult.stderr || checkoutResult.stdout || "git checkout failed");
  }

  return { branch, workDir };
}
