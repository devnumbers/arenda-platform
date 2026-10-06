// Contract tests for the migration domain-rules script: SQL file paths as
// args → exit 0 (clean) / 1 (findings printed) / 2 (usage error). The script
// is spawned as a subprocess exactly the way `make migrations-lint` runs it;
// internal structure is not imported or tested.
//
// Fixtures are written to a temp dir per test: the contract is about file
// content → verdict, and inline SQL keeps each case readable next to its
// assertion.
import { describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const scriptPath = path.join(import.meta.dirname, "domain-rules.mjs");

// The canonical timeout header every up migration >= 000108 must open with
// (convention #324). Fixtures named 0002xx_* fall under the rule, so every
// fixture that expects a clean verdict carries it — same as real migrations.
const TIMEOUTS_HEADER = "SET lock_timeout = '1s';\nSET statement_timeout = '5s';\n";

function fixture(name, sql) {
  const dir = mkdtempSync(path.join(tmpdir(), "migration-lint-"));
  const file = path.join(dir, name);
  writeFileSync(file, sql);
  return file;
}

function runRules(files) {
  const args = Array.isArray(files) ? files : [files];
  const res = spawnSync(process.execPath, [scriptPath, ...args], {
    encoding: "utf8",
    timeout: 15_000,
  });
  return { code: res.status, stdout: res.stdout, stderr: res.stderr };
}

describe("domain-rules: money must be BIGINT kopecks (money-float-type)", () => {
  const floatTypes = [
    "NUMERIC(14,2)",
    "numeric",
    "DECIMAL(10,2)",
    "real",
    "double precision",
    "float",
    "float4",
    "float8",
    "FLOAT(53)",
  ];

  for (const type of floatTypes) {
    it(`flags a column of type ${type}`, () => {
      const file = fixture("000200_bad.up.sql", `ALTER TABLE t ADD COLUMN amount ${type};\n`);
      const res = runRules(file);
      expect(res.code).toBe(1);
      expect(res.stdout).toContain("money-float-type");
      expect(res.stdout).toContain("000200_bad.up.sql:1:");
    });
  }

  it("flags the type line inside a multi-line CREATE TABLE statement", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "CREATE TABLE wallets (\n    id UUID PRIMARY KEY,\n    amount NUMERIC(14,2) NOT NULL\n);\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("money-float-type");
    expect(res.stdout).toContain(":3:");
  });

  it("flags a type changed to double precision in a later statement", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "ALTER TABLE t ADD COLUMN note TEXT;\n\nALTER TABLE t ALTER COLUMN amount TYPE double precision;\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("money-float-type");
    expect(res.stdout).toContain(":3:");
  });

  it("allows BIGINT kopecks columns", () => {
    const file = fixture(
      "000200_ok.up.sql",
      `${TIMEOUTS_HEADER}CREATE TABLE wallets (\n    id UUID PRIMARY KEY,\n    amount_kopecks BIGINT NOT NULL CHECK (amount_kopecks >= 0)\n);\n`,
    );
    expect(runRules(file).code).toBe(0);
  });
});

describe("domain-rules: no DEFAULT on id (id-column-default, ADR 0019)", () => {
  it("flags CREATE TABLE with DEFAULT on id", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "CREATE TABLE things (\n    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),\n    name TEXT NOT NULL\n);\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("id-column-default");
    expect(res.stdout).toContain(":2:");
  });

  it("flags DEFAULT written before PRIMARY KEY", () => {
    const file = fixture(
      "000200_bad.up.sql",
      'CREATE TABLE things (\n    "id" UUID DEFAULT uuidv7() PRIMARY KEY\n);\n',
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("id-column-default");
  });

  it("flags ALTER TABLE ADD COLUMN id ... DEFAULT", () => {
    const file = fixture("000200_bad.up.sql", "ALTER TABLE things ADD COLUMN id UUID DEFAULT uuidv7();\n");
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("id-column-default");
  });

  it("flags ALTER COLUMN id SET DEFAULT", () => {
    const file = fixture("000200_bad.up.sql", "ALTER TABLE things ALTER COLUMN id SET DEFAULT gen_random_uuid();\n");
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("id-column-default");
  });

  it("allows DEFAULT on non-id columns", () => {
    const file = fixture(
      "000200_ok.up.sql",
      `${TIMEOUTS_HEADER}CREATE TABLE things (\n    id UUID PRIMARY KEY,\n    created_at TIMESTAMPTZ NOT NULL DEFAULT now()\n);\n`,
    );
    expect(runRules(file).code).toBe(0);
  });

  it("allows a seed INSERT that sets id explicitly with uuidv7()", () => {
    const file = fixture("000200_ok.up.sql", `${TIMEOUTS_HEADER}INSERT INTO things (id, name) VALUES (uuidv7(), 'x');\n`);
    expect(runRules(file).code).toBe(0);
  });
});

describe("domain-rules: statement-aware parsing (comments, quoting)", () => {
  it("ignores -- line comments mentioning banned types", () => {
    const file = fixture(
      "000200_ok.up.sql",
      `${TIMEOUTS_HEADER}-- prices are numeric(14,2) no more, see 000022\nALTER TABLE t ADD COLUMN amount BIGINT;\n`,
    );
    expect(runRules(file).code).toBe(0);
  });

  it("ignores /* block comments */ mentioning banned types", () => {
    const file = fixture(
      "000200_ok.up.sql",
      `${TIMEOUTS_HEADER}/* decimal(10,2) was here\n   real and float too */\nALTER TABLE t ADD COLUMN amount BIGINT;\n`,
    );
    expect(runRules(file).code).toBe(0);
  });

  it("ignores banned-type words inside string literals", () => {
    const file = fixture(
      "000200_ok.up.sql",
      `${TIMEOUTS_HEADER}ALTER TABLE t ADD COLUMN kind TEXT CHECK (kind IN ('numeric', 'decimal'));\n`,
    );
    expect(runRules(file).code).toBe(0);
  });

  it("ignores banned-type words inside dollar-quoted bodies", () => {
    const file = fixture(
      "000200_ok.up.sql",
      `${TIMEOUTS_HEADER}CREATE FUNCTION f() RETURNS trigger AS $$\nBEGIN\n    -- hypothetical numeric(14,2) comment inside the body\n    RETURN NEW;\nEND;\n$$ LANGUAGE plpgsql;\n`,
    );
    expect(runRules(file).code).toBe(0);
  });

  it("is not confused by a semicolon inside a string literal", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "INSERT INTO t (note) VALUES ('a;b; c');\n\nALTER TABLE t ADD COLUMN amount NUMERIC(14,2);\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("money-float-type");
    expect(res.stdout).toContain(":3:");
  });
});

describe("domain-rules: scope", () => {
  it("skips down migrations as deliberate inverses of linted up files", () => {
    const file = fixture(
      "000200_rollback.down.sql",
      "CREATE TABLE things (\n    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),\n    amount NUMERIC(14,2)\n);\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("skipped");
  });

  it("skips historical pre-rule migrations listed as exemptions", () => {
    const file = fixture(
      "000001_init_schema.up.sql",
      "CREATE TABLE users (\n    id UUID PRIMARY KEY DEFAULT gen_random_uuid()\n);\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("exempt");
  });

  it("reports per-file findings across a multi-file run", () => {
    const clean = fixture("000200_ok.up.sql", `${TIMEOUTS_HEADER}ALTER TABLE t ADD COLUMN amount BIGINT;\n`);
    const dirty = fixture("000201_bad.up.sql", "ALTER TABLE t ADD COLUMN amount NUMERIC;\n");
    const res = runRules([clean, dirty]);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("000201_bad.up.sql");
    expect(res.stdout).not.toContain("000200_ok.up.sql:1");
  });

  it("exits 2 with usage when given no files", () => {
    const res = runRules([]);
    expect(res.code).toBe(2);
    expect(res.stderr).toContain("usage");
  });
});

describe("domain-rules: SET-timeout headers on up migrations (migration-timeouts, #324/#1146)", () => {
  it("flags an up migration >= 000108 without the SET header", () => {
    const file = fixture("000200_bad.up.sql", "ALTER TABLE t ADD COLUMN note TEXT;\n");
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("migration-timeouts");
    expect(res.stdout).toContain("000200_bad.up.sql:1:");
  });

  it("allows the canonical header", () => {
    const file = fixture("000200_ok.up.sql", `${TIMEOUTS_HEADER}ALTER TABLE t ADD COLUMN note TEXT;\n`);
    expect(runRules(file).code).toBe(0);
  });

  it("allows leading comments before the header", () => {
    const file = fixture(
      "000200_ok.up.sql",
      `-- ticket #1112: idempotency keys for creations\n${TIMEOUTS_HEADER}ALTER TABLE t ADD COLUMN note TEXT;\n`,
    );
    expect(runRules(file).code).toBe(0);
  });

  it("flags a lock_timeout value other than '1s'", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "SET lock_timeout = '30s';\nSET statement_timeout = '5s';\nALTER TABLE t ADD COLUMN note TEXT;\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("migration-timeouts");
    expect(res.stdout).toContain(":1:");
  });

  it("flags a missing statement_timeout when lock_timeout is present", () => {
    const file = fixture("000200_bad.up.sql", "SET lock_timeout = '1s';\nALTER TABLE t ADD COLUMN note TEXT;\n");
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("migration-timeouts");
    expect(res.stdout).toContain(":2:");
  });

  it("flags SETs that are not the first statements", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "ALTER TABLE t ADD COLUMN note TEXT;\nSET lock_timeout = '1s';\nSET statement_timeout = '5s';\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("migration-timeouts");
    expect(res.stdout).toContain(":1:");
  });

  it("flags a wrong-order header (statement_timeout first)", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "SET statement_timeout = '5s';\nSET lock_timeout = '1s';\nALTER TABLE t ADD COLUMN note TEXT;\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("migration-timeouts");
  });

  it("allows a statement_timeout value other than '5s' with a same-line justification comment", () => {
    const file = fixture(
      "000200_ok.up.sql",
      "SET lock_timeout = '1s';\nSET statement_timeout = '30s'; -- heavy backfill: full-table rewrite\nALTER TABLE t ADD COLUMN note TEXT;\n",
    );
    expect(runRules(file).code).toBe(0);
  });

  it("flags a statement_timeout value other than '5s' without a justification comment", () => {
    const file = fixture(
      "000200_bad.up.sql",
      "SET lock_timeout = '1s';\nSET statement_timeout = '30s';\nALTER TABLE t ADD COLUMN note TEXT;\n",
    );
    const res = runRules(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("migration-timeouts");
    expect(res.stdout).toContain(":2:");
  });

  it("does not apply to pre-cutoff versions (< 000108)", () => {
    const file = fixture("000107_legacy.up.sql", "ALTER TABLE t ADD COLUMN note TEXT;\n");
    expect(runRules(file).code).toBe(0);
  });
});
