// Registry-scan contract (docs/agents/parallel-dev.md «Реестр слотов»): there
// is no central registry file — the registry IS the .env files of the
// checkouts. worktree-new scans AREND_SLOT in the root .env and
// .worktrees/*/.env, takes the first free N of 1..9, and rejects a corrupt
// registry (duplicate or malformed slot claims) instead of silently reusing
// a slot. On top of the registry sits the port preflight: the .env files say
// nothing about who actually listens, and a slot handed out over a foreign
// listener would get its ports killed by the e2e runner's free_port()
// (#489 live check).
import { describe, expect, it } from "vitest";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { parseEnvFile, scanSlotRegistry, selectSlotByPorts, slotConfig, slotFromEnv, slotPorts } from "./lib.mjs";

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
  const allSlots = [1, 2, 3, 4, 5, 6, 7, 8, 9];

  it("returns every slot free on an empty checkout", () => {
    const root = makeRoot({});
    expect(scanSlotRegistry(root)).toEqual({ taken: new Map(), freeSlots: allSlots });
  });

  it("does not count a root .env without a slot (plain slot-0 checkout)", () => {
    const root = makeRoot({ rootEnv: "POSTGRES_PORT=5433\nDATABASE_URL=postgres://...\n" });
    expect(scanSlotRegistry(root).freeSlots[0]).toBe(1);
  });

  it("ignores an explicit slot 0 in the root .env and worktree .env files that lack a slot", () => {
    const root = makeRoot({
      rootEnv: "AREND_SLOT=0\n",
      worktreeEnvs: { feat: "POSTGRES_PORT=5441\n" },
    });
    const res = scanSlotRegistry(root);
    expect(res.freeSlots[0]).toBe(1);
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
    expect(res.freeSlots[0]).toBe(2);
    expect([...res.taken.keys()].sort()).toEqual([1, 3]);
  });

  it("returns no free slots when all nine worktree slots are taken", () => {
    const root = makeRoot({
      worktreeEnvs: Object.fromEntries(
        Array.from({ length: 9 }, (_, i) => [`wt${i + 1}`, `AREND_SLOT=${i + 1}\n`]),
      ),
    });
    expect(scanSlotRegistry(root).freeSlots).toEqual([]);
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
    expect(scanSlotRegistry(root).freeSlots[0]).toBe(1);
  });
});

describe("slotPorts", () => {
  it("lists a slot's six host ports: dev role (pg, backend, frontend) then e2e", () => {
    expect(slotPorts(slotConfig(1))).toEqual([5441, 8091, 3021, 5446, 8096, 3026]);
  });

  it("keeps slot 0 on its historical ports", () => {
    expect(slotPorts(slotConfig(0))).toEqual([5433, 8080, 3000, 5436, 8081, 3010]);
  });
});

describe("selectSlotByPorts", () => {
  // probe(port) → pid array, empty = free — the seam worktree-new wires to lsof.
  it("takes the first candidate when all six ports are free", () => {
    const res = selectSlotByPorts([2, 5], () => []);
    expect(res.slot).toBe(2);
    expect(res.blocked).toEqual([]);
  });

  it("skips a candidate whose port already has a listener and reports the holder", () => {
    const probe = (port) => (port === 8096 ? ["4242"] : []);
    const res = selectSlotByPorts([1, 2], probe);
    expect(res.slot).toBe(2);
    expect(res.blocked).toEqual([{ slot: 1, busyPorts: [{ port: 8096, pids: ["4242"] }] }]);
  });

  it("a busy dev-role port blocks the slot just like an e2e one", () => {
    const res = selectSlotByPorts([1, 3], (port) => (port === 3021 ? ["77"] : []));
    expect(res.slot).toBe(3);
    expect(res.blocked).toEqual([{ slot: 1, busyPorts: [{ port: 3021, pids: ["77"] }] }]);
  });

  it("keeps probing in order and returns every blocked candidate when nothing is free", () => {
    const probe = (port) => (port === 8091 || port === 8092 ? ["9", "10"] : []);
    const res = selectSlotByPorts([1, 2], probe);
    expect(res.slot).toBeNull();
    expect(res.blocked.map((b) => b.slot)).toEqual([1, 2]);
    expect(res.blocked[0].busyPorts).toEqual([{ port: 8091, pids: ["9", "10"] }]);
    expect(res.blocked[1].busyPorts).toEqual([{ port: 8092, pids: ["9", "10"] }]);
  });

  it("a null probe answer (the probe can't run) aborts the preflight as unavailable", () => {
    const res = selectSlotByPorts([1, 2], (port) => (port === 5441 ? null : []));
    expect(res.unavailable).toBe(true);
    expect(res.slot).toBeNull();
  });
});
