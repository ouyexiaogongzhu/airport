// 測試用 D1：node:sqlite 內存庫 + 按序執行 migrations/，讓 SQL 在真實 SQLite 上驗證。
// 只實現代碼用到的子集：prepare/bind/first/all/run、batch（單事務）。
import { DatabaseSync } from 'node:sqlite';
import { readdirSync, readFileSync } from 'node:fs';

type Value = null | number | bigint | string | Uint8Array;

const MIGRATIONS = new URL('../../migrations/', import.meta.url);

function toValue(v: unknown): Value {
  if (v === undefined || v === null) return null;
  if (typeof v === 'boolean') return v ? 1 : 0;
  return v as Value;
}

export function createTestD1(): { db: D1Database; raw: DatabaseSync } {
  const raw = new DatabaseSync(':memory:');
  for (const f of readdirSync(MIGRATIONS).filter((n) => n.endsWith('.sql')).sort()) {
    raw.exec(readFileSync(new URL(f, MIGRATIONS), 'utf8'));
  }

  const makeStmt = (sql: string, params: Value[] = []) => {
    const stmt = {
      sql,
      params,
      bind: (...args: unknown[]) => makeStmt(sql, args.map(toValue)),
      first: async <T>(col?: string): Promise<T | null> => {
        const row = raw.prepare(sql).get(...params);
        if (!row) return null;
        return (col ? row[col] : row) as T;
      },
      all: async <T>() => ({ results: raw.prepare(sql).all(...params) as T[], success: true, meta: {} }),
      run: async () => {
        const r = raw.prepare(sql).run(...params);
        return { success: true, meta: { changes: Number(r.changes), last_row_id: Number(r.lastInsertRowid) } };
      },
    };
    return stmt;
  };

  const db = {
    prepare: (sql: string) => makeStmt(sql),
    batch: async (stmts: ReturnType<typeof makeStmt>[]) => {
      raw.exec('BEGIN');
      try {
        const out = [];
        for (const s of stmts) out.push(await s.run());
        raw.exec('COMMIT');
        return out;
      } catch (e) {
        raw.exec('ROLLBACK');
        throw e;
      }
    },
  };
  return { db: db as unknown as D1Database, raw };
}
