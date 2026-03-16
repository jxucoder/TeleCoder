import { runCommand } from "../process.ts";
import type { FailureKind, TaskRunInput, TaskRunResult } from "../types.ts";

export interface TaskRuntime {
  runTask(input: TaskRunInput): Promise<TaskRunResult>;
}

export class TaskRuntimeError extends Error {
  constructor(
    message: string,
    readonly failureKind: FailureKind,
    readonly exitCode?: number,
  ) {
    super(message);
    this.name = "TaskRuntimeError";
  }
}

function splitCommand(command: string): string[] {
  const parts: string[] = [];
  let current = "";
  let quote: "'" | '"' | null = null;

  for (let index = 0; index < command.length; index += 1) {
    const char = command[index]!;

    if (quote) {
      if (char === quote) {
        quote = null;
      } else {
        current += char;
      }
      continue;
    }

    if (char === "'" || char === '"') {
      quote = char;
      continue;
    }

    if (/\s/.test(char)) {
      if (current) {
        parts.push(current);
        current = "";
      }
      continue;
    }

    current += char;
  }

  if (current) {
    parts.push(current);
  }

  return parts;
}

function permissionFlag(mode: TaskRunInput["permissionMode"]): string {
  switch (mode) {
    case "approve-all":
      return "--approve-all";
    case "deny-all":
      return "--deny-all";
    case "approve-reads":
    default:
      return "--approve-reads";
  }
}

export function buildAcpxCommand(input: TaskRunInput): string[] {
  return [
    ...splitCommand(input.acpxCommand),
    "--cwd",
    input.cwd,
    permissionFlag(input.permissionMode),
    "--timeout",
    String(input.timeoutSeconds),
    "--format",
    "json",
    "--json-strict",
    ...(input.agentCommand ? ["--agent", input.agentCommand] : [input.agent]),
    "exec",
    input.prompt,
  ];
}

export function extractAssistantText(jsonl: string): string {
  const chunks: string[] = [];

  for (const line of jsonl.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }

    let parsed: any;
    try {
      parsed = JSON.parse(trimmed);
    } catch {
      continue;
    }

    const update = parsed?.params?.update;
    if (update?.sessionUpdate !== "agent_message_chunk") {
      continue;
    }

    const text = update?.content?.text;
    if (typeof text === "string" && text.length > 0) {
      chunks.push(text);
    }
  }

  return chunks.join("");
}

export class AcpxRuntime implements TaskRuntime {
  async runTask(input: TaskRunInput): Promise<TaskRunResult> {
    const command = buildAcpxCommand(input);

    const result = await runCommand(command, {
      cwd: input.cwd,
      timeoutMs: input.timeoutSeconds * 1000,
    });

    if (result.timedOut) {
      throw new TaskRuntimeError(
        `acpx exceeded the ${input.timeoutSeconds}s runtime limit`,
        "timeout",
        result.exitCode,
      );
    }

    if (result.exitCode !== 0) {
      const message = result.stderr || result.stdout || `acpx exited with code ${result.exitCode}`;
      const failureKind = /permission|denied|approval/i.test(message)
        ? "policy_denied"
        : "runtime_error";
      throw new TaskRuntimeError(message, failureKind, result.exitCode);
    }

    const assistantText = extractAssistantText(result.stdout);
    return {
      output: assistantText || result.stdout || result.stderr,
    };
  }
}
