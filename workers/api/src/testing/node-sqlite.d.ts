// node:sqlite 的最小類型（Node 22+ 內建；項目未安裝 @types/node）
declare module 'node:sqlite' {
  type Value = null | number | bigint | string | Uint8Array;
  export class StatementSync {
    get(...params: Value[]): Record<string, unknown> | undefined;
    all(...params: Value[]): Record<string, unknown>[];
    run(...params: Value[]): { changes: number | bigint; lastInsertRowid: number | bigint };
  }
  export class DatabaseSync {
    constructor(path: string);
    exec(sql: string): void;
    prepare(sql: string): StatementSync;
  }
}

declare module 'node:fs' {
  export function readFileSync(path: string | URL, encoding: 'utf8'): string;
  export function readdirSync(path: string | URL): string[];
}
