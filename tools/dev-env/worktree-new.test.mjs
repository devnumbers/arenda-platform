// Contract tests for make worktree-new's engine, run the way make runs it
// (spawned with cwd = the checkout). Each test builds a real throwaway git
// repo: the orchestration spans git (worktree add, ref checks) and the
// filesystem (two generated env files), so the spawn boundary is the seam.
// The generated-file expectations are the contract literals from
// docs/agents/parallel-dev.md «Per-worktree .env» and «make worktree-new».
import { describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const scriptPath = path.join(import.meta.dirname, "worktree-new.mjs");

function git(root, ...args) {
  const res = spawnSync("git", ["-c", "commit.gpgsign=false", ...args], {
    cwd: root,
    encoding: "utf8",
  });
  if (res.status !== 0) throw new Error(`git ${args.join(" ")} failed: ${res.stderr}`);
  return res.stdout;
}

function makeGitRepo({ rootEnv, envExample, registry = {} }) {
  const root = mkdtempSync(path.join(tmpdir(), "dev-env-wt-"));
  git(root, "init", "-q");
  git(root, "config", "user.email", "test@example.com");
  git(root, "config", "user.name", "test");
  writeFileSync(path.join(root, "README.md"), "repo\n");
  if (rootEnv !== undefined) writeFileSync(path.join(root, ".env"), rootEnv);
  if (envExample !== undefined) writeFileSync(path.join(root, ".env.example"), envExample);
  // Pre-seeded fake worktrees: the registry only reads .env files, so plain
  // directories exercise slot exhaustion and duplicate claims without paying
  // for nine real worktrees.
  for (const [name, env] of Object.entries(registry)) {
    mkdirSync(path.join(root, ".worktrees", name), { recursive: true });
    writeFileSync(path.join(root, ".worktrees", name, ".env"), env);
  }
  git(root, "add", ".");
  git(root, "commit", "-qm", "init");
  return root;
}

function runWorktreeNew(root, name) {
  const res = spawnSync(process.execPath, [scriptPath, name], {
    cwd: root,
    encoding: "utf8",
    timeout: 30_000,
  });
  return { code: res.status, stdout: res.stdout, stderr: res.stderr };
}

function worktreeBranches(root) {
  return git(root, "worktree", "list", "--porcelain");
}

describe("worktree-new: happy path", () => {
  const rootEnv = "BACKEND_URL=http://localhost:8080\nPOSTGRES_PORT=5433\nPOSTGRES_USER=arenda\nPOSTGRES_PASSWORD=arenda\nPOSTGRES_DB=arenda\nDADATA_API_KEY=real-key\n";

  it("creates the worktree on a new branch, generates both env files, prints the facts", () => {
    const root = makeGitRepo({ rootEnv });
    const res = runWorktreeNew(root, "feature-a");

    expect(res.code).toBe(0);
    const wtDir = path.join(root, ".worktrees", "feature-a");
    expect(existsSync(wtDir)).toBe(true);
    expect(worktreeBranches(root)).toContain(`branch refs/heads/feature-a`);

    // The worktree .env is the copy plus the appended slot block (last wins).
    const wtEnv = readFileSync(path.join(wtDir, ".env"), "utf8");
    expect(wtEnv).toContain("DADATA_API_KEY=real-key");
    expect(wtEnv).toContain("AREND_SLOT=1");
    expect(wtEnv).toContain("POSTGRES_PORT=5441");
    expect(wtEnv).toContain("DATABASE_URL=postgres://arenda:arenda@localhost:5441/arenda?sslmode=disable");
    expect(wtEnv).toContain("HTTP_ADDR=:8091");
    expect(wtEnv).toContain("COMPOSE_PROJECT_NAME=arenda-wt1");
    expect(wtEnv).toContain("E2E_PG_PORT=5446");
    expect(wtEnv).toContain("E2E_BACKEND_PORT=8096");
    expect(wtEnv).toContain("E2E_FRONTEND_PORT=3026");
    expect(wtEnv).toContain("E2E_COMPOSE_PROJECT=arenda-e2e-wt1");
    expect(wtEnv.indexOf("DADATA_API_KEY")).toBeLessThan(wtEnv.indexOf("AREND_SLOT=1"));

    // The frontend reads BACKEND_URL only from its own directory.
    const feEnv = readFileSync(path.join(wtDir, "apps", "frontend", ".env.development.local"), "utf8");
    expect(feEnv).toContain("BACKEND_URL=http://localhost:8091");

    // Connection facts: slot, both port roles, both projects, start commands.
    for (const fact of ["slot 1", "5441", "8091", "3021", "5446", "8096", "3026", "arenda-wt1", "arenda-e2e-wt1", "make local-infra-up", "make frontend-dev", wtDir]) {
      expect(res.stdout).toContain(fact);
    }
  });

  it("allocates the next free slot on the second worktree", () => {
    const root = makeGitRepo({ rootEnv });
    expect(runWorktreeNew(root, "feature-a").code).toBe(0);
    const res = runWorktreeNew(root, "feature-b");
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("slot 2");
    expect(readFileSync(path.join(root, ".worktrees", "feature-b", ".env"), "utf8")).toContain("AREND_SLOT=2");
  });

  it("prints the same derived DATABASE_URL it writes into the .env (custom credentials)", () => {
    const root = makeGitRepo({
      rootEnv: "POSTGRES_USER=ivan\nPOSTGRES_PASSWORD=s3cret\nPOSTGRES_DB=platform\n",
    });
    const res = runWorktreeNew(root, "feature-a");
    expect(res.code).toBe(0);
    const wtEnv = readFileSync(path.join(root, ".worktrees", "feature-a", ".env"), "utf8");
    const generatedUrl = wtEnv.split("\n").find((l) => l.startsWith("DATABASE_URL="));
    expect(res.stdout).toContain(generatedUrl);
    expect(generatedUrl).toContain("@localhost:5441/platform");
  });

  it("falls back to .env.example with a warning when the checkout has no root .env", () => {
    const root = makeGitRepo({ envExample: "ENCRYPTION_KEY=...\n" });
    const res = runWorktreeNew(root, "feature-a");
    expect(res.code).toBe(0);
    const wtEnv = readFileSync(path.join(root, ".worktrees", "feature-a", ".env"), "utf8");
    expect(wtEnv).toContain("ENCRYPTION_KEY=...");
    expect(wtEnv).toContain(".env.example");
    expect(res.stdout).toContain(".env.example");
  });
});

describe("worktree-new: refusals", () => {
  it("rejects names outside the slug contract", () => {
    const root = makeGitRepo({ rootEnv: "A=1\n" });
    for (const bad of ["Feature", "a b", "-lead", "..", "a/b"]) {
      const res = runWorktreeNew(root, bad);
      expect(res.code).toBe(2);
      expect(res.stderr).toContain("slug");
    }
    expect(existsSync(path.join(root, ".worktrees", "Feature"))).toBe(false);
  });

  it("rejects a missing name with usage", () => {
    const root = makeGitRepo({ rootEnv: "A=1\n" });
    // make expands an unset WT= to an empty argument.
    expect(runWorktreeNew(root, "")).toEqual(expect.objectContaining({ code: 2 }));
  });

  it("rejects an existing branch without creating anything", () => {
    const root = makeGitRepo({ rootEnv: "A=1\n" });
    git(root, "branch", "taken");
    const res = runWorktreeNew(root, "taken");
    expect(res.code).toBe(1);
    expect(res.stderr).toContain("taken");
    expect(existsSync(path.join(root, ".worktrees", "taken"))).toBe(false);
  });

  it("rejects an existing worktree directory", () => {
    const root = makeGitRepo({ rootEnv: "A=1\n" });
    expect(runWorktreeNew(root, "feature-a").code).toBe(0);
    const res = runWorktreeNew(root, "feature-a");
    expect(res.code).toBe(1);
    expect(res.stderr).toContain("exists");
  });

  it("refuses when no slot is free and leaves the tree untouched", () => {
    const registry = Object.fromEntries(
      Array.from({ length: 9 }, (_, i) => [`wt${i + 1}`, `AREND_SLOT=${i + 1}\n`]),
    );
    const root = makeGitRepo({ rootEnv: "A=1\n", registry });
    const res = runWorktreeNew(root, "feature-a");
    expect(res.code).toBe(1);
    expect(res.stderr).toContain("slot");
    expect(existsSync(path.join(root, ".worktrees", "feature-a"))).toBe(false);
  });

  it("refuses on a corrupt registry (duplicate slot claims)", () => {
    const root = makeGitRepo({
      rootEnv: "A=1\n",
      registry: { a: "AREND_SLOT=1\n", b: "AREND_SLOT=1\n" },
    });
    const res = runWorktreeNew(root, "feature-a");
    expect(res.code).toBe(1);
    expect(res.stderr).toContain("duplicate");
    expect(existsSync(path.join(root, ".worktrees", "feature-a"))).toBe(false);
  });
});
