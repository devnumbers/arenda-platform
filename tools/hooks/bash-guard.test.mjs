// Contract tests for the PreToolUse(Bash) guard: hook JSON on stdin → exit
// code (0 = allow, 2 = block). The script is spawned as a subprocess exactly
// the way a harness runs it; internal structure is not imported or tested.
import { describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import path from "node:path";

const scriptPath = path.join(import.meta.dirname, "bash-guard.mjs");

function runGuard(command, payloadOverrides = {}) {
  const payload = {
    hook_event_name: "PreToolUse",
    session_id: "test-session",
    cwd: "/tmp/anywhere",
    tool_name: "Bash",
    tool_input: { command },
    ...payloadOverrides,
  };
  const res = spawnSync(process.execPath, [scriptPath], {
    input: JSON.stringify(payload),
    encoding: "utf8",
    timeout: 15_000,
  });
  return { code: res.status, stderr: res.stderr, stdout: res.stdout };
}

describe("bash-guard: destructive commands are blocked (exit 2)", () => {
  const blocked = [
    "rm -rf /tmp/x",
    "rm -fr .tmp",
    "rm -r build",
    "rm -Rf node_modules",
    "rm --recursive --force dist",
    "rm --recursive dist",
    "rm -f -r dist",
    "rm -rf /",
    "sudo rm -rf /tmp/x",
    "FOO=1 rm -rf /tmp/x",
    "env rm -rf /tmp/x",
    "/bin/rm -rf /tmp/x",
    "echo hi && rm -rf /tmp/x",
    "make test ; rm -rf /tmp/x",
    "make test || rm -rf /tmp/x",
    "cat list | xargs rm -rf",
    "xargs rm -r < list",
    "echo $(rm -rf /tmp/x)",
    "(cd /tmp && rm -rf x)",
    "git clean -fdx",
    "git clean -f",
    "git -C apps/backend clean -fd",
    "git clean --force -d -x",
    "git reset --hard",
    "git reset --hard HEAD~1",
    "git -C apps/backend reset --hard origin/main",
    "git push --force origin main",
    "git push -f origin main",
    "git push origin main --force",
    "docker system prune -af",
    "docker system prune",
    "docker volume prune",
    "docker compose down -v --remove-orphans",
    "docker compose down --volumes",
    "docker-compose down -v",
    "docker compose -p arenda-test down -v",
  ];

  for (const command of blocked) {
    it(`blocks: ${command}`, () => {
      const res = runGuard(command);
      expect(res.code).toBe(2);
      expect(res.stderr).toContain("bash-guard");
      expect(res.stderr.length).toBeGreaterThan("bash-guard: ".length);
    });
  }
});

describe("bash-guard: safe commands are allowed (exit 0)", () => {
  const allowed = [
    "rm build.log",
    "rm -f build.log",
    "rm -f a.txt b.txt",
    "rm -- build.log",
    "ls -la",
    "make test",
    "make local-infra-reset",
    "git clean -n",
    "git clean -nd",
    "git clean --dry-run -fdx",
    "git reset --soft HEAD~1",
    "git reset --keep HEAD~1",
    "git checkout main",
    "git push origin main",
    "git push --force-with-lease origin main",
    "git status --porcelain",
    "docker compose down",
    "docker compose -p arenda-test down --remove-orphans",
    "docker volume ls",
    "docker ps",
    // rm mentioned but not executed: guarded commands are matched as the
    // first command word of a segment, not as a substring.
    "echo 'rm -rf /'",
    "echo rm -rf /",
    "grep -r pattern apps/",
    "tar -xf archive.tar",
    "man rm",
  ];

  for (const command of allowed) {
    it(`allows: ${command}`, () => {
      const res = runGuard(command);
      expect(res.code).toBe(0);
    });
  }
});

describe("bash-guard: payload handling is fail-open", () => {
  function runRaw(stdin) {
    return spawnSync(process.execPath, [scriptPath], {
      input: stdin,
      encoding: "utf8",
      timeout: 15_000,
    });
  }

  it("allows on empty stdin", () => {
    expect(runRaw("").status).toBe(0);
  });

  it("allows on invalid JSON", () => {
    expect(runRaw("not json at all").status).toBe(0);
  });

  it("allows when tool_input.command is missing", () => {
    expect(runRaw(JSON.stringify({ hook_event_name: "PreToolUse", tool_input: {} })).status).toBe(0);
  });

  it("allows a non-string command", () => {
    expect(runRaw(JSON.stringify({ tool_input: { command: 42 } })).status).toBe(0);
  });

  it("reads the ZCode camelCase toolInput.command variant", () => {
    const res = runRaw(
      JSON.stringify({ hookEventName: "PreToolUse", toolName: "Bash", toolInput: { command: "rm -rf /tmp/x" } }),
    );
    expect(res.status).toBe(2);
  });
});
