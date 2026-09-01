// Registry-scan contract (docs/agents/parallel-dev.md «Реестр слотов»): there
// is no central registry file — the registry IS the .env files of the
// checkouts. worktree-new scans AREND_SLOT in the root .env and
// .worktrees/*/.env, takes the first free N of 1..9, and rejects a corrupt
// registry (duplicate or malformed slot claims) instead of silently reusing
// a slot.
import { describe, expect, it } from "vitest";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { parseEnvFile, scanSlotRegistry, slotFromEnv } from "./lib.mjs";

function makeRoot({ rootEnv, worktreeEnvs }) {
  const root = mkdtempSync(path.join(tmpdir(), "dev-env-registry-"));
  if (rootEnv !== undefined) writeFileSync(path.join(root, ".env"), rootEnv);
  for (const [name, env] of Object.entries(worktreeEnvs ?? {})) {
    mkdirSync(path.join(root, ".worktrees", name), { recursive: true });
    if (env !== null) writeFileSync(path.join(root, ".worktrees", name, ".env"), env);
  }
  return root;
}

describe("parseEnvFile", () => {
  it("skips comments and blank lines, last occurrence wins, quotes stripped", () => {
    const env = parseEnvFile(
      [
        "# header comment",
        "",
        "POSTGRES_PORT=5433",
        'BACKEND_URL="http://localhost:8080"',
        "POSTGRES_PORT=5441",
        "#AREND_SLOT=9",
      ].join("\n"),
    );
    expect(env.POSTGRES_PORT).toBe("5441");
    expect(env.BACKEND_URL).toBe("http://localhost:8080");
    expect(env.AREND_SLOT).toBeUndefined();
  });

  it("keeps values containing = and trims CR from CRLF files", () => {
    const env = parseEnvFile('DATABASE_URL=postgres://arenda@localhost/db?sslmode=disable\r\nX="a=b"\r\n');
    expect(env.DATABASE_URL).toBe("postgres://arenda@localhost/db?sslmode=disable");
    expect(env.X).toBe("a=b");
  });
});

describe("slotFromEnv", () => {
  it("returns null when AREND_SLOT is absent and the slot when present", () => {
    expect(slotFromEnv({})).toBeNull();
    expect(slotFromEnv({ AREND_SLOT: "2" })).toBe(2);
    expect(slotFromEnv({ AREND_SLOT: "0" })).toBe(0);
  });

  it("throws on a malformed slot value", () => {
    expect(() => slotFromEnv({ AREND_SLOT: "abc" })).toThrow(/AREND_SLOT/);
    expect(() => slotFromEnv({ AREND_SLOT: "12" })).toThrow(/AREND_SLOT/);
  });
});

describe("scanSlotRegistry", () => {
  it("returns 1 free on an empty checkout", () => {
    const root = makeRoot({});
    expect(scanSlotRegistry(root)).toEqual({ taken: new Map(), free: 1 });
  });

  it("does not count a root .env without a slot (plain slot-0 checkout)", () => {
    const root = makeRoot({ rootEnv: "POSTGRES_PORT=5433\nDATABASE_URL=postgres://...\n" });
    expect(scanSlotRegistry(root).free).toBe(1);
  });

  it("ignores an explicit slot 0 in the root .env and worktree .env files that lack a slot", () => {
    const root = makeRoot({
      rootEnv: "AREND_SLOT=0\n",
      worktreeEnvs: { feat: "POSTGRES_PORT=5441\n" },
    });
    const res = scanSlotRegistry(root);
    expect(res.free).toBe(1);
    expect(res.taken.size).toBe(0);
  });

  it("finds the first free slot past the taken worktrees", () => {
    const root = makeRoot({
      worktreeEnvs: {
        b: "AREND_SLOT=3\n",
        a: "AREND_SLOT=1\n",
      },
    });
    const res = scanSlotRegistry(root);
    expect(res.free).toBe(2);
    expect([...res.taken.keys()].sort()).toEqual([1, 3]);
  });

  it("returns null when all nine worktree slots are taken", () => {
    const root = makeRoot({
      worktreeEnvs: Object.fromEntries(
        Array.from({ length: 9 }, (_, i) => [`wt${i + 1}`, `AREND_SLOT=${i + 1}\n`]),
      ),
    });
    expect(scanSlotRegistry(root).free).toBeNull();
  });

  it("rejects duplicate slot claims naming both files", () => {
    const root = makeRoot({
      worktreeEnvs: { a: "AREND_SLOT=2\n", b: "AREND_SLOT=2\n" },
    });
    expect(() => scanSlotRegistry(root)).toThrow(/AREND_SLOT=2/);
  });

  it("rejects a malformed slot value naming the file", () => {
    const root = makeRoot({ worktreeEnvs: { broken: "AREND_SLOT=oops\n" } });
    expect(() => scanSlotRegistry(root)).toThrow(/\.worktrees[/\\]broken[/\\]\.env/);
  });

  it("ignores worktree directories without an .env and non-directory entries", () => {
    const root = makeRoot({ worktreeEnvs: { empty: null } });
    writeFileSync(path.join(root, ".worktrees", "stray-file"), "not a dir");
    expect(scanSlotRegistry(root).free).toBe(1);
  });
});
