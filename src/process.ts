export interface CommandResult {
  exitCode: number;
  stderr: string;
  timedOut: boolean;
  stdout: string;
}

export interface RunCommandOptions {
  cwd?: string;
  env?: Record<string, string | undefined>;
  timeoutMs?: number;
}

function readText(stream: ReadableStream<Uint8Array> | null): Promise<string> {
  if (!stream) {
    return Promise.resolve("");
  }
  return new Response(stream).text();
}

export async function runCommand(
  command: string[],
  options: RunCommandOptions = {},
): Promise<CommandResult> {
  let timedOut = false;
  const proc = Bun.spawn(command, {
    cwd: options.cwd,
    env: { ...process.env, ...options.env },
    stderr: "pipe",
    stdout: "pipe",
  });

  const timer =
    options.timeoutMs && options.timeoutMs > 0
      ? setTimeout(() => {
          timedOut = true;
          proc.kill("SIGKILL");
        }, options.timeoutMs)
      : null;

  try {
    const [stdout, stderr, exitCode] = await Promise.all([
      readText(proc.stdout),
      readText(proc.stderr),
      proc.exited,
    ]);

    return {
      exitCode,
      stderr: stderr.trim(),
      timedOut,
      stdout: stdout.trim(),
    };
  } finally {
    if (timer) {
      clearTimeout(timer);
    }
  }
}
