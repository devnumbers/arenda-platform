import { execSync } from 'child_process';

export interface PgConfig {
  container?: string;
  user?: string;
  password?: string;
  database?: string;
}

export function queryValue(query: string, config: PgConfig = {}): string | null {
  const {
    container = 'arenda-local-postgres-1',
    user = process.env.POSTGRES_USER || 'arenda',
    password = process.env.POSTGRES_PASSWORD || 'arenda',
    database = process.env.POSTGRES_DB || 'arenda',
  } = config;

  try {
    const result = execSync(
      `docker exec -e PGPASSWORD=${password} -i ${container} psql -U ${user} -d ${database} -tA -c "${query.replace(/"/g, '\\"')}"`,
      { encoding: 'utf-8', timeout: 10_000 }
    );
    const trimmed = result.trim();
    return trimmed === '' ? null : trimmed;
  } catch (error) {
    console.error('DB query failed:', query, error);
    return null;
  }
}

export function queryRows<T extends Record<string, unknown>>(query: string, config: PgConfig = {}): T[] {
  const {
    container = 'arenda-local-postgres-1',
    user = process.env.POSTGRES_USER || 'arenda',
    password = process.env.POSTGRES_PASSWORD || 'arenda',
    database = process.env.POSTGRES_DB || 'arenda',
  } = config;

  try {
    const result = execSync(
      `docker exec -e PGPASSWORD=${password} -i ${container} psql -U ${user} -d ${database} -tA -F '\\x1f' -R '\\x1e' -c "${query.replace(/"/g, '\\"')}"`,
      { encoding: 'utf-8', timeout: 10_000 }
    );
    const lines = result.split('\x1e').map((line) => line.trim()).filter(Boolean);
    const columns = query
      .replace(/select\s+/i, '')
      .split(' from ')[0]
      .split(',')
      .map((c) => c.trim().split(/\s+/).pop()?.replace(/"/g, '') || c.trim());
    return lines.map((line) => {
      const values = line.split('\x1f');
      const row = {} as T;
      columns.forEach((col, idx) => {
        (row as Record<string, unknown>)[col] = values[idx] ?? null;
      });
      return row;
    });
  } catch (error) {
    console.error('DB query failed:', query, error);
    return [];
  }
}

export async function assertDbState(
  query: string,
  expected: string | ((value: string | null) => boolean),
  message: string
): Promise<void> {
  const actual = queryValue(query);
  if (typeof expected === 'function') {
    if (!expected(actual)) {
      throw new Error(`${message}: predicate failed, got ${actual}`);
    }
    return;
  }
  if (actual !== expected) {
    throw new Error(`${message}: expected ${expected}, got ${actual}`);
  }
}

export async function assertDbRows<T extends Record<string, unknown>>(
  query: string,
  predicate: (rows: T[]) => boolean,
  message: string
): Promise<void> {
  const rows = queryRows<T>(query);
  if (!predicate(rows)) {
    throw new Error(`${message}: predicate failed, got ${JSON.stringify(rows)}`);
  }
}
