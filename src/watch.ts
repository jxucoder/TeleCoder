import { runCommand } from "./process.ts";
import type {
  CiWatchEvent,
  PrWatchEvent,
  SessionRecord,
  TeleCoderPolicyMode,
  WatchKind,
  WatchRecord,
  WatchStatus,
  WorkspaceWritePolicy,
} from "./types.ts";

interface ReturnSections {
  changed?: string;
  next?: string;
  uncertain?: string;
  verified?: string;
}

interface ReturnSummarySession {
  agent: string;
  error: string;
  policyMode: TeleCoderPolicyMode;
  resultText: string;
  runtimeCommand: string;
  status: SessionRecord["status"];
  workspaceWritePolicy: WorkspaceWritePolicy;
}

const DIFF_CONTEXT_LIMIT = 6000;

export function parseWatchKind(raw: string | undefined): WatchKind | undefined {
  switch (raw) {
    case undefined:
    case "":
      return undefined;
    case "ci_failure":
    case "pr_review":
      return raw;
    default:
      throw new Error(`Invalid watch kind: ${raw}`);
  }
}

export function parseWatchStatus(raw: string | undefined): WatchStatus | undefined {
  switch (raw) {
    case undefined:
    case "":
      return undefined;
    case "active":
    case "paused":
      return raw;
    default:
      throw new Error(`Invalid watch status: ${raw}`);
  }
}

export function isCiFailureEvent(event: CiWatchEvent): boolean {
  const status = (event.status ?? "completed").toLowerCase();
  const conclusion = (event.conclusion ?? "failure").toLowerCase();

  return status === "completed" && conclusion === "failure";
}

export function isPrReviewEvent(event: PrWatchEvent): boolean {
  const action = (event.action ?? "synchronize").toLowerCase();
  return action === "opened" || action === "reopened" || action === "synchronize";
}

export function matchesCiWatch(watch: WatchRecord, event: CiWatchEvent): boolean {
  if (watch.kind !== "ci_failure" || watch.status !== "active") {
    return false;
  }

  if (watch.repo !== event.repo) {
    return false;
  }

  if (watch.workflowName && watch.workflowName !== event.workflowName) {
    return false;
  }

  if (watch.branchName && watch.branchName !== event.branchName) {
    return false;
  }

  return true;
}

export function matchesPrWatch(watch: WatchRecord, event: PrWatchEvent): boolean {
  if (watch.kind !== "pr_review" || watch.status !== "active") {
    return false;
  }

  if (watch.repo !== event.repo) {
    return false;
  }

  if (watch.baseBranch && watch.baseBranch !== event.baseBranch) {
    return false;
  }

  if (watch.headBranch && watch.headBranch !== event.headBranch) {
    return false;
  }

  return true;
}

export function defaultCiWatchPolicy(): TeleCoderPolicyMode {
  return "observe";
}

export function defaultPrWatchPolicy(): TeleCoderPolicyMode {
  return "observe";
}

export function buildCiEventKey(event: CiWatchEvent): string {
  return event.runId;
}

export function buildPrEventKey(event: PrWatchEvent): string {
  return `${event.prNumber}:${event.headSha ?? event.headBranch}`;
}

export function buildCiTriggerSummary(watch: WatchRecord, event: CiWatchEvent): string {
  const scope = [event.workflowName, event.branchName, event.sha ? event.sha.slice(0, 12) : ""]
    .filter(Boolean)
    .join(" on ");
  const source = event.runUrl ? ` (${event.runUrl})` : "";

  return `CI watch ${watch.id} fired: ${scope}${source}`;
}

export function buildPrTriggerSummary(watch: WatchRecord, event: PrWatchEvent): string {
  const source = event.prUrl ? ` (${event.prUrl})` : "";
  return `PR watch ${watch.id} fired: PR #${event.prNumber} ${event.title}${source}`;
}

function sharedReturnInstructions(): string[] {
  return [
    "Boundaries:",
    "- Do not push changes.",
    "- Do not create, update, or merge pull requests.",
    "- If code changes seem necessary, describe them in Next instead of performing them.",
    "",
    "Return exactly these sections:",
    "Changed:",
    "Verified:",
    "Uncertain:",
    "Next:",
  ];
}

export function buildCiWatchPrompt(watch: WatchRecord, event: CiWatchEvent): string {
  const details = [
    `CI watch fired for repository: ${event.repo}`,
    `Workflow: ${event.workflowName}`,
    `Branch: ${event.branchName}`,
    `Run ID: ${event.runId}`,
    event.sha ? `Commit: ${event.sha}` : "",
    event.runUrl ? `Run URL: ${event.runUrl}` : "",
    event.summary ? `Reported summary: ${event.summary}` : "",
    "",
    "Instructions:",
    watch.instructions,
    "",
    ...sharedReturnInstructions(),
  ]
    .filter(Boolean)
    .join("\n");

  return details;
}

export async function loadPrDiffContext(event: PrWatchEvent): Promise<string> {
  if (event.diffText && event.diffText.trim()) {
    return clipDiffContext(event.diffText.trim());
  }

  const repoCheck = await runCommand(["git", "-C", event.repo, "rev-parse", "--git-dir"], {
    timeoutMs: 10_000,
  });
  if (repoCheck.exitCode !== 0) {
    return "Diff context unavailable: repo is not a readable local git checkout.";
  }

  const statResult = await runCommand(
    ["git", "-C", event.repo, "diff", "--stat", "--no-color", `${event.baseBranch}...${event.headBranch}`],
    { timeoutMs: 10_000 },
  );
  const diffResult = await runCommand(
    [
      "git",
      "-C",
      event.repo,
      "diff",
      "--no-color",
      "--unified=0",
      `${event.baseBranch}...${event.headBranch}`,
    ],
    { timeoutMs: 10_000 },
  );

  if (statResult.exitCode !== 0 || diffResult.exitCode !== 0) {
    return "Diff context unavailable: git diff could not be loaded for the requested base/head refs.";
  }

  const parts = [
    statResult.stdout ? `Diff stat:\n${statResult.stdout}` : "",
    diffResult.stdout ? `\nDiff excerpt:\n${clipDiffContext(diffResult.stdout)}` : "",
  ]
    .filter(Boolean)
    .join("\n");

  return parts || "Diff context unavailable: no diff output was produced.";
}

function clipDiffContext(diffText: string): string {
  if (diffText.length <= DIFF_CONTEXT_LIMIT) {
    return diffText;
  }
  return `${diffText.slice(0, DIFF_CONTEXT_LIMIT)}\n[diff truncated]`;
}

export function buildPrWatchPrompt(
  watch: WatchRecord,
  event: PrWatchEvent,
  diffContext: string,
): string {
  const details = [
    `PR review watch fired for repository: ${event.repo}`,
    `PR: #${event.prNumber} ${event.title}`,
    `Base branch: ${event.baseBranch}`,
    `Head branch: ${event.headBranch}`,
    event.action ? `Action: ${event.action}` : "",
    event.headSha ? `Head SHA: ${event.headSha}` : "",
    event.prUrl ? `PR URL: ${event.prUrl}` : "",
    event.body ? `PR body:\n${event.body}` : "",
    "",
    diffContext ? `Loaded diff context:\n${diffContext}` : "Loaded diff context unavailable.",
    "",
    "Instructions:",
    watch.instructions,
    "",
    ...sharedReturnInstructions(),
  ]
    .filter(Boolean)
    .join("\n");

  return details;
}

export function summarizeWatchOutcome(session: Pick<SessionRecord, "error" | "resultText" | "status">): string {
  const sections = parseReturnSections(session.resultText);

  if (session.status === "complete") {
    const changed = normalizeSectionValue(sections.changed);
    if (changed) {
      return changed.slice(0, 280);
    }

    const text = session.resultText.trim();
    if (!text) {
      return "Completed without agent output.";
    }
    return text.split("\n")[0]!.slice(0, 280);
  }

  if (session.status === "error") {
    return `Failed: ${session.error}`.slice(0, 280);
  }

  return `Session is ${session.status}.`;
}

export function buildWatchReturnSummary(
  triggerSummary: string,
  session: ReturnSummarySession,
): string {
  const sections = parseReturnSections(session.resultText);

  const changed = normalizeSectionValue(sections.changed) ?? fallbackChanged(session);
  const verified = normalizeSectionValue(sections.verified) ?? "Not explicitly verified.";
  const uncertain =
    session.status === "error"
      ? session.error || "Session failed without an explicit error."
      : normalizeSectionValue(sections.uncertain) ?? "No additional uncertainty reported.";
  const next =
    normalizeSectionValue(sections.next) ??
    (session.status === "complete"
      ? "Review the result and decide whether to act on it."
      : "Inspect the error and rerun or intervene manually.");

  return [
    `Trigger: ${triggerSummary}`,
    `Runtime: acpx -> ${session.runtimeCommand} as ${session.agent}; policy=${session.policyMode}; workspace-writes=${session.workspaceWritePolicy}`,
    `Changed: ${changed}`,
    `Verified: ${verified}`,
    `Uncertain: ${uncertain}`,
    `Next: ${next}`,
  ].join("\n");
}

function fallbackChanged(session: Pick<ReturnSummarySession, "resultText" | "status">): string {
  if (session.status !== "complete") {
    return "No completed change summary is available.";
  }

  const text = session.resultText.trim();
  if (!text) {
    return "No change summary reported.";
  }

  return text.split("\n")[0]!;
}

function parseReturnSections(text: string): ReturnSections {
  const sections: ReturnSections = {};
  let current: keyof ReturnSections | null = null;
  const buffers: Partial<Record<keyof ReturnSections, string[]>> = {};

  for (const rawLine of text.split("\n")) {
    const line = rawLine.trim();
    const heading = parseSectionHeading(line);
    if (heading) {
      current = heading.key;
      buffers[current] = [heading.value];
      continue;
    }

    if (!current) {
      continue;
    }

    if (!buffers[current]) {
      buffers[current] = [];
    }
    buffers[current]!.push(rawLine.trim());
  }

  for (const key of Object.keys(buffers) as Array<keyof ReturnSections>) {
    const value = normalizeSectionValue(buffers[key]!.join("\n"));
    if (value) {
      sections[key] = value;
    }
  }

  return sections;
}

function parseSectionHeading(
  line: string,
): { key: keyof ReturnSections; value: string } | null {
  const match = line.match(/^(Changed|Verified|Uncertain|Next):\s*(.*)$/i);
  if (!match) {
    return null;
  }

  const label = match[1]!.toLowerCase();
  const value = match[2] ?? "";

  switch (label) {
    case "changed":
      return { key: "changed", value };
    case "verified":
      return { key: "verified", value };
    case "uncertain":
      return { key: "uncertain", value };
    case "next":
      return { key: "next", value };
    default:
      return null;
  }
}

function normalizeSectionValue(raw: string | undefined): string | undefined {
  if (!raw) {
    return undefined;
  }

  const normalized = raw
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .join(" ");

  return normalized || undefined;
}
