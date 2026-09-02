// Contract tests for the Stop gate: hook JSON on stdin → exit code (0 =
// allow, 2 = block). The script under test is a verbatim copy inside a
// throwaway git repository whose gates (make targets / npm script) are fakes
// that log their invocation — the copy makes the script's "same repository"
// self-scoping resolve to the fixture, so the real repo is never touched.
import { describe, expect, it, afterEach } from "vitest";
import { spawnSync } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const sourceScript = path.join(import.meta.dirname, "stop-gate.mjs");

// git exports its own environment to hook processes, and in a linked
// worktree GIT_DIR is absolute — a pre-push run from a worktree poisons
// every fixture git call below with the real checkout's git dir (hooks
// resolve from it too). The fixtures own their git dirs: strip the hook
// environment wholesale at the spawn boundary.
function sanitizedEnv() {
  return Object.fromEntries(
    Object.entries(process.env).filter(([key]) => !key.startsWith("GIT_")),
  );
}

function exec(dir, args, opts = {}) {
  const res = spawnSync("git", ["-C", dir, ...args], { encoding: "utf8", ...opts, env: sanitizedEnv() });
  if (res.status !== 0) throw new Error(`git ${args.join(" ")} failed: ${res.stderr}`);
  return res.stdout;
}

const MAKEFILE = `backend-lint:
\t@node mark-gate.mjs backend

backend-nolint:
\t@node mark-gate.mjs backend-nolint

ts-suppressions:
\t@node mark-gate.mjs ts-suppressions

admin-typecheck:
\t@node mark-gate.mjs admin

migrations-lint:
\t@node mark-gate.mjs migrations
`;

const MARK_GATE = `import { appendFileSync, existsSync } from "node:fs";
import { fileURLToPath } from "node:url";
import path from "node:path";
const root = path.dirname(fileURLToPath(import.meta.url));
const name = process.argv[2];
appendFileSync(path.join(root, "gates.log"), name + "\\n");
if (existsSync(path.join(root, "fail-" + name))) process.exit(1);
`;

// npm --prefix apps/<app> runs the script with cwd = apps/<app>.
const FRONTEND_PACKAGE_JSON = JSON.stringify(
  { name: "fixture-frontend", private: true, scripts: { lint: "node ../../mark-gate.mjs frontend" } },
  null,
  2,
);

const ADMIN_PACKAGE_JSON = JSON.stringify(
  { name: "fixture-admin", private: true, scripts: { lint: "node ../../mark-gate.mjs admin-lint" } },
  null,
  2,
);

function makeFixture(name) {
  const dir = path.join(tmpdir(), `stop-gate-${name}-${process.pid}-${Date.now()}`);
  mkdirSync(path.join(dir, "apps/backend/db/migrations"), { recursive: true });
  mkdirSync(path.join(dir, "apps/frontend"), { recursive: true });
  mkdirSync(path.join(dir, "apps/admin"), { recursive: true });
  mkdirSync(path.join(dir, "docs"), { recursive: true });
  writeFileSync(path.join(dir, "Makefile"), MAKEFILE);
  writeFileSync(path.join(dir, "mark-gate.mjs"), MARK_GATE);
  writeFileSync(path.join(dir, "apps/frontend/package.json"), FRONTEND_PACKAGE_JSON);
  writeFileSync(path.join(dir, "apps/admin/package.json"), ADMIN_PACKAGE_JSON);
  // Tracked baseline so a new migration file shows as its full path in
  // `git status --porcelain` (an untracked directory would collapse to
  // "?? apps/backend/", exactly like in the real repo).
  writeFileSync(path.join(dir, "apps/backend/db/migrations/000000_baseline.up.sql"), "-- baseline\n");
  copyFileSync(sourceScript, path.join(dir, "stop-gate.mjs"));
  exec(dir, ["init", "-q", "-b", "main"]);
  exec(dir, ["config", "user.email", "test@test"]);
  exec(dir, ["config", "user.name", "test"]);
  exec(dir, ["add", "-A"]);
  exec(dir, ["commit", "-q", "-m", "baseline"]);
  return dir;
}

function runStopGate(script, dir, payloadOverrides = {}, stdin = undefined) {
  const payload = {
    hook_event_name: "Stop",
    session_id: "test-session",
    cwd: dir,
    ...payloadOverrides,
  };
  const res = spawnSync(process.execPath, [script], {
    input: stdin ?? JSON.stringify(payload),
    encoding: "utf8",
    cwd: dir,
    timeout: 60_000,
    env: sanitizedEnv(),
  });
  return { code: res.status, stderr: res.stderr, stdout: res.stdout };
}

function gatesRun(dir) {
  const log = path.join(dir, "gates.log");
  return existsSync(log) ? readFileSync(log, "utf8").trim().split("\n").filter(Boolean).sort() : [];
}

const allFixtures = [];
afterEach(() => {
  while (allFixtures.length > 0) {
    const dir = allFixtures.pop();
    rmSync(dir, { recursive: true, force: true });
  }
});

describe("stop-gate: gates by touched package", () => {
  const fixture = (name) => {
    const dir = makeFixture(name);
    allFixtures.push(dir);
    return dir;
  };

  it("clean tree runs nothing", () => {
    const dir = fixture("clean");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual([]);
  });

  it("uncommitted backend change runs only the backend gates", () => {
    const dir = fixture("backend");
    writeFileSync(path.join(dir, "apps/backend/main.go"), "package main\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["backend", "backend-nolint"]);
  });

  it("uncommitted frontend change runs only the frontend gate", () => {
    const dir = fixture("frontend");
    writeFileSync(path.join(dir, "apps/frontend/page.tsx"), "export {};\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["frontend", "ts-suppressions"]);
  });

  it("uncommitted admin change runs only the admin gates", () => {
    const dir = fixture("admin");
    writeFileSync(path.join(dir, "apps/admin/App.tsx"), "export {};\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["admin", "admin-lint", "ts-suppressions"]);
  });

  it("changes across all three packages run all the gates", () => {
    const dir = fixture("mixed");
    writeFileSync(path.join(dir, "apps/backend/main.go"), "package main\n");
    writeFileSync(path.join(dir, "apps/frontend/page.tsx"), "export {};\n");
    writeFileSync(path.join(dir, "apps/admin/App.tsx"), "export {};\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["admin", "admin-lint", "backend", "backend-nolint", "frontend", "ts-suppressions"]);
  });

  it("untracked files count as touched", () => {
    const dir = fixture("untracked");
    writeFileSync(path.join(dir, "apps/backend/new_file.go"), "package main\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["backend", "backend-nolint"]);
  });

  it("staged files count as touched", () => {
    const dir = fixture("staged");
    writeFileSync(path.join(dir, "apps/admin/App.tsx"), "export {};\n");
    exec(dir, ["add", "apps/admin/App.tsx"]);
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["admin", "admin-lint", "ts-suppressions"]);
  });

  it("uncommitted migration change runs the migrations gate (and the backend gates)", () => {
    const dir = fixture("migrations");
    writeFileSync(path.join(dir, "apps/backend/db/migrations/000200_x.up.sql"), "ALTER TABLE t ADD COLUMN c BIGINT;\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["backend", "backend-nolint", "migrations"]);
  });

  it("changes outside the three packages run nothing", () => {
    const dir = fixture("docs");
    writeFileSync(path.join(dir, "docs/README.md"), "text\n");
    writeFileSync(path.join(dir, "Makefile"), `${MAKEFILE}# comment\n`);
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual([]);
  });
});

describe("stop-gate: blocking and fail-open behavior", () => {
  const fixture = (name) => {
    const dir = makeFixture(name);
    allFixtures.push(dir);
    return dir;
  };

  it("a failing gate blocks the turn (exit 2) and names the gate", () => {
    const dir = fixture("failing");
    writeFileSync(path.join(dir, "apps/backend/main.go"), "package main\n");
    writeFileSync(path.join(dir, "fail-backend"), "");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(2);
    expect(res.stderr).toContain("backend-lint");
    expect(res.stderr).toContain("stop-gate");
  });

  it("a failing admin lint gate blocks the turn (exit 2)", () => {
    const dir = fixture("failing-admin-lint");
    writeFileSync(path.join(dir, "apps/admin/App.tsx"), "export {};\n");
    writeFileSync(path.join(dir, "fail-admin-lint"), "");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
    expect(res.code).toBe(2);
    expect(res.stderr).toContain("npm --prefix apps/admin run lint");
    expect(gatesRun(dir)).toEqual(["admin", "admin-lint", "ts-suppressions"]);
  });

  it("invalid JSON on stdin fails open (exit 0)", () => {
    const dir = fixture("badjson");
    writeFileSync(path.join(dir, "apps/backend/main.go"), "package main\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir, {}, "not json");
    expect(res.code).toBe(0);
  });

  it("stays silent in a repository the script does not belong to", () => {
    const dir = fixture("foreign");
    writeFileSync(path.join(dir, "apps/backend/main.go"), "package main\n");
    // The real script lives in this monorepo, the session repo is the fixture:
    // different repositories → allow without running any gates.
    const res = runStopGate(sourceScript, dir);
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual([]);
  });

  it("honors the payload cwd instead of the process cwd", () => {
    const dir = fixture("payloadcwd");
    writeFileSync(path.join(dir, "apps/backend/main.go"), "package main\n");
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), tmpdir(), { cwd: dir });
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual(["backend", "backend-nolint"]);
  });

  it("allows when payload cwd is outside any git repository", () => {
    const dir = fixture("nogit");
    const outside = path.join(dir, "plain-dir");
    mkdirSync(outside);
    const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir, { cwd: outside });
    expect(res.code).toBe(0);
    expect(gatesRun(dir)).toEqual([]);
  });

  it("fixtures are isolated from a hook's GIT_* environment", () => {
    // A pre-push hook run from a linked worktree exports GIT_DIR pointing
    // into the real checkout; the fixture git ops must not see it.
    const poisoned = { GIT_DIR: path.join(tmpdir(), "no-such-git-dir"), GIT_WORK_TREE: tmpdir() };
    const restore = Object.entries(poisoned).map(([key, value]) => {
      const prev = process.env[key];
      process.env[key] = value;
      return [key, prev];
    });
    try {
      const dir = fixture("poisoned-env");
      writeFileSync(path.join(dir, "apps/backend/main.go"), "package main\n");
      const res = runStopGate(path.join(dir, "stop-gate.mjs"), dir);
      expect(res.code).toBe(0);
      expect(gatesRun(dir)).toEqual(["backend", "backend-nolint"]);
    } finally {
      for (const [key, prev] of restore) {
        if (prev === undefined) delete process.env[key];
        else process.env[key] = prev;
      }
    }
  });
});
