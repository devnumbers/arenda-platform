#!/usr/bin/env node
// Stop gate: run the quality gates of the packages with uncommitted changes
// before the agent's turn ends, so "claiming done without checks" is
// mechanically hard.
//
// Harness-neutral contract: hook JSON on stdin → exit 0 (allow) / 2 (block,
// stderr carries the reason the model sees). Internal errors fail open
// (exit 0), mirroring the fail-open harness hook mechanism.
//
// Scope: uncommitted working-tree changes only (staged + unstaged +
// untracked). Committed changes are already gated at commit time by the
// lefthook pre-commit hook (lefthook.yml), so the two layers compose without
// double-gating the same diff; `git commit --no-verify` remains the one
// documented deliberate bypass of both.
//
// Self-scoping: the gate commands (make/npm targets) exist only in this
// repository, so the hook silently allows unless the session repo (payload
// cwd) shares a git common dir with the checkout the script itself lives in
// (worktrees included). The user-level harness config fires hooks in every
// session; this check keeps every other project quiet.

import { spawn, spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";

// Mirrors the pre-commit gates in lefthook.yml; keep the two in sync. A gate
// lists every path prefix that trips it (the TS suppression gate spans both
// frontend and admin — one full-tree scan either way).
const PACKAGE_GATES = [
  { name: "backend", prefixes: ["apps/backend/"], command: ["make", "backend-lint"] },
  // Same prefix as the backend gate — both run in parallel on any backend
  // change (nolint gate #344: zero //nolint directives, full pass).
  { name: "backend nolint", prefixes: ["apps/backend/"], command: ["make", "backend-nolint"] },
  { name: "frontend", prefixes: ["apps/frontend/"], command: ["npm", "--prefix", "apps/frontend", "run", "lint"] },
  // Suppression gate #399: zero eslint-disable/@ts-*/explicit-any in the
  // manual code of both apps; a change in either app runs the full pass.
  { name: "ts suppressions", prefixes: ["apps/frontend/", "apps/admin/"], command: ["make", "ts-suppressions"] },
  { name: "admin typecheck", prefixes: ["apps/admin/"], command: ["make", "admin-typecheck"] },
  { name: "admin lint", prefixes: ["apps/admin/"], command: ["npm", "--prefix", "apps/admin", "run", "lint"] },
  // Coarser than lefthook's *.sql glob by design (prefix matching): a
  // migration-only change also trips the backend prefix above — accepted.
  { name: "migrations lint", prefixes: ["apps/backend/db/migrations/"], command: ["make", "migrations-lint"] },
];

const OUTPUT_TAIL_LINES = 40;

function readStdinJson() {
  try {
    return JSON.parse(readFileSync(0, "utf8"));
  } catch {
    return null; // no or invalid payload → fail open
  }
}

function git(dir, args) {
  const res = spawnSync("git", ["-C", dir, ...args], { encoding: "utf8" });
  if (res.status !== 0) return null;
  return res.stdout.trim();
}

function changedPaths(root) {
  const status = git(root, ["status", "--porcelain"]);
  if (status === null) return [];
  const paths = [];
  for (const line of status.split("\n")) {
    if (line === "") continue;
    const entry = line.slice(3).replace(/^"|"$/g, "");
    // Renames list both sides; either location counts as touched.
    for (const p of entry.split(" -> ")) {
      const trimmed = p.trim().replace(/^"|"$/g, "");
      if (trimmed !== "") paths.push(trimmed);
    }
  }
  return paths;
}

function runGate(gate, cwd) {
  return new Promise((resolve) => {
    const child = spawn(gate.command[0], gate.command.slice(1), { cwd });
    let output = "";
    const collect = (chunk) => {
      output += chunk;
    };
    child.stdout.on("data", collect);
    child.stderr.on("data", collect);
    child.on("error", (err) => resolve({ gate, spawnError: err })); // e.g. binary missing: environment, not quality
    child.on("close", (code) => resolve({ gate, code, output }));
  });
}

function outputTail(output) {
  const lines = output.trimEnd().split("\n");
  return lines.slice(-OUTPUT_TAIL_LINES).join("\n");
}

async function main() {
  const payload = readStdinJson();
  if (payload === null) return;

  const sessionDir = typeof payload?.cwd === "string" && payload.cwd !== "" ? payload.cwd : process.cwd();
  const sessionRepo = git(sessionDir, ["rev-parse", "--path-format=absolute", "--git-common-dir"]);
  const scriptRepo = git(import.meta.dirname, ["rev-parse", "--path-format=absolute", "--git-common-dir"]);
  if (!sessionRepo || !scriptRepo || sessionRepo !== scriptRepo) return;

  const root = git(sessionDir, ["rev-parse", "--show-toplevel"]);
  if (!root) return;

  const paths = changedPaths(root);
  const gates = PACKAGE_GATES.filter((gate) =>
    paths.some((p) => gate.prefixes.some((prefix) => p.startsWith(prefix))),
  );
  if (gates.length === 0) return;

  const results = await Promise.all(gates.map((gate) => runGate(gate, root)));

  for (const res of results) {
    if (res.spawnError) {
      console.error(`stop-gate: could not start the ${res.gate.name} gate (${res.spawnError.message}) — skipped`);
    }
  }

  const failed = results.filter((res) => res.code !== null && res.code !== undefined && res.code !== 0);
  if (failed.length > 0) {
    for (const res of failed) {
      console.error(`stop-gate: ${res.gate.command.join(" ")} failed (exit ${res.code}). Fix it before ending the turn.`);
      if (res.output.trim() !== "") {
        console.error(`--- ${res.gate.name} gate output (last ${OUTPUT_TAIL_LINES} lines) ---`);
        console.error(outputTail(res.output));
        console.error(`--- end of ${res.gate.name} gate output ---`);
      }
    }
    process.exit(2);
  }

  console.error(`stop-gate: ok (${gates.map((g) => g.name).join(", ")})`);
}

await main();
