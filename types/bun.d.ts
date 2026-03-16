declare module "bun:sqlite" {
  export interface RunResult {
    changes: number;
    lastInsertRowid: number | bigint;
  }

  export interface Statement {
    all(...params: unknown[]): unknown[];
    get(...params: unknown[]): unknown;
    run(...params: unknown[]): RunResult;
  }

  export class Database {
    constructor(filename?: string, options?: { create?: boolean; strict?: boolean });
    close(): void;
    exec(sql: string): void;
    query(sql: string): Statement;
  }
}

declare interface BunProcess {
  stdout: ReadableStream<Uint8Array> | null;
  stderr: ReadableStream<Uint8Array> | null;
  exited: Promise<number>;
  kill(signal?: string): void;
}

declare interface BunSpawnOptions {
  cwd?: string;
  env?: Record<string, string | undefined>;
  stdout?: "inherit" | "pipe";
  stderr?: "inherit" | "pipe";
}

declare interface BunServer {
  port: number;
  stop(): void;
}

declare interface BunServeOptions {
  hostname?: string;
  port: number;
  fetch(request: Request): Response | Promise<Response>;
}

declare interface BunNamespace {
  argv: string[];
  env: Record<string, string | undefined>;
  serve(options: BunServeOptions): BunServer;
  spawn(command: string[], options?: BunSpawnOptions): BunProcess;
}

declare const Bun: BunNamespace;
