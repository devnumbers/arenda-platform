// Contract tests for the dev-env CLI: the Makefile evaluates it at parse time
// ($(shell node tools/dev-env/dev-env.mjs compose-project)) and inside
// recipes, so the contract is command + optional root arg → one stdout line,
// exit 0; a corrupt registry entry exits 1 with the file named on stderr;
// unknown usage exits 2.
import { describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const scriptPath = path.join(import.meta.dirname, "dev-env.mjs");

function makeRoot(env) {
  const root = mkdtempSync(path.join(tmpdir(), "dev-env-cli-"));
  if (env !== undefined) writeFileSync(path.join(root, ".env"), env);
  return root;
}

function runCli(...args) {
  const res = spawnSync(process.execPath, [scriptPath, ...args], {
    encoding: "utf8",
    timeout: 15_000,
  });
  return { code: res.status, stdout: res.stdout, stderr: res.stderr };
}

describe("dev-env.mjs compose-project", () => {
  it("prints arenda-local without a .env", () => {
    const res = runCli("compose-project", makeRoot());
    expect(res.code).toBe(0);
    expect(res.stdout.trim()).toBe("arenda-local");
  });

  it("prints arenda-local for slot 0 and arenda-wtN for a worktree slot", () => {
    expect(runCli("compose-project", makeRoot("AREND_SLOT=0\n")).stdout.trim()).toBe("arenda-local");
    expect(runCli("compose-project", makeRoot("AREND_SLOT=2\n")).stdout.trim()).toBe("arenda-wt2");
  });

  it("fails with the file named when the .env carries a malformed slot", () => {
    const root = makeRoot("AREND_SLOT=oops\n");
    const res = runCli("compose-project", root);
    expect(res.code).toBe(1);
    expect(res.stderr).toContain(".env");
    expect(res.stderr).toContain("AREND_SLOT");
  });
});

describe("dev-env.mjs frontend-port", () => {
  it("prints 3000 without a .env and 3020+N for a worktree slot", () => {
    expect(runCli("frontend-port", makeRoot()).stdout.trim()).toBe("3000");
    expect(runCli("frontend-port", makeRoot("AREND_SLOT=3\n")).stdout.trim()).toBe("3023");
  });
});

describe("dev-env.mjs usage", () => {
  it("exits 2 with usage on stderr for unknown or missing commands", () => {
    expect(runCli().code).toBe(2);
    expect(runCli().stderr).toContain("usage");
    expect(runCli("bogus").code).toBe(2);
  });
});
