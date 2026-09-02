#!/usr/bin/env node
// make worktree-new WT=<name> engine (docs/agents/parallel-dev.md): create a
// git worktree under .worktrees/<name> on a new branch, allocate the next
// free slot from the registry (.env files) whose ports no listener holds
// (lsof preflight), generate the worktree's root .env (copy + slot block)
// and apps/frontend/.env.development.local, and print the connection facts.
// Deliberately does not install dependencies, run baseline tests, start
// infrastructure, or push — that is the using-git-worktrees skill's side of
// the recipe.
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { envOverrideBlock, parseEnvFile, scanSlotRegistry, selectSlotByPorts, slotConfig, slotDatabaseUrl } from "./lib.mjs";

const USAGE = `usage: node worktree-new.mjs <name> [root]

<name> names both the branch and the .worktrees/<name> directory:
lowercase slug [a-z0-9._-], no leading dash or dot.
`;

function die(message, code = 1) {
  console.error(`worktree-new: ${message}`);
  process.exit(code);
}

function git(root, args) {
  const res = spawnSync("git", args, { cwd: root, encoding: "utf8" });
  if (res.error) die(`git is not available: ${res.error.message}`);
  return res;
}

const [name, root = process.cwd()] = process.argv.slice(2);
if (!name) {
  process.stderr.write(USAGE);
  process.exit(2);
}
if (!/^[a-z0-9][a-z0-9._-]*$/.test(name)) {
  die(`"${name}" is not a valid name — the slug contract is [a-z0-9._-] with no leading dash or dot`, 2);
}

const wtDir = path.join(root, ".worktrees", name);
if (git(root, ["rev-parse", "--verify", "-q", `refs/heads/${name}`]).status === 0) {
  die(`branch ${name} already exists`);
}
if (existsSync(wtDir)) {
  die(`${wtDir} already exists`);
}

let registry;
try {
  registry = scanSlotRegistry(root);
} catch (err) {
  die(err.message);
}
if (registry.freeSlots.length === 0) {
  die("no free worktree slot — all of 1..9 are claimed; remove a worktree to release its slot");
}

// Port preflight on top of the registry (#489 live check): the registry only
// knows .env claims, not who actually listens. A slot handed out over a
// foreign listener would let the e2e runner's free_port() kill that process,
// so held slots are skipped with a warning naming the holding pids. An
// unavailable lsof means no preflight: warn and fall back to the registry's
// first free slot.
function listeningPids(port) {
  const res = spawnSync("lsof", ["-ti", `tcp:${port}`, "-sTCP:LISTEN"], { encoding: "utf8" });
  if (res.error) return null;
  return res.stdout
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

const preflight = selectSlotByPorts(registry.freeSlots, listeningPids);
for (const { slot: held, busyPorts } of preflight.blocked) {
  const detail = busyPorts.map(({ port, pids }) => `port ${port} (pid ${pids.join(", ")})`).join(", ");
  console.error(
    `worktree-new: WARNING: slot ${held} skipped — ${detail} already listening; ` +
      "the ports are held by another stack or a stray process (docs/agents/parallel-dev.md, «Разборка»)",
  );
}
let { slot, cfg } = preflight;
if (preflight.unavailable) {
  console.error(
    "worktree-new: WARNING: lsof is not available — the port preflight was skipped; " +
      "check the slot's ports by hand before running e2e (docs/agents/parallel-dev.md, «Разборка»)",
  );
  slot = registry.freeSlots[0];
  cfg = slotConfig(slot);
}
if (slot === null) {
  die("every free slot 1..9 has listening ports — clear the holders first (docs/agents/parallel-dev.md, «Разборка»)");
}

const worktree = git(root, ["worktree", "add", path.relative(root, wtDir), "-b", name]);
if (worktree.status !== 0) {
  die(`git worktree add failed:\n${worktree.stderr}`);
}

// Source env: the real keys of the main checkout when they exist, otherwise
// the placeholder example plus a warning that keys must be filled by hand.
let baseFile = path.join(root, ".env");
let fallbackExample = false;
if (!existsSync(baseFile)) {
  baseFile = path.join(root, ".env.example");
  fallbackExample = true;
  if (!existsSync(baseFile)) {
    die(`neither ${path.join(root, ".env")} nor ${baseFile} exists — nothing to copy keys from`);
  }
}
const baseContent = readFileSync(baseFile, "utf8");
const baseEnv = parseEnvFile(baseContent);

const header = fallbackExample
  ? [
      "# WARNING: the main checkout had no root .env — this file was copied from",
      "# .env.example. Fill in the real placeholder keys by hand (DADATA_API_KEY,",
      "# ENCRYPTION_KEY, SMTP_*, T_KASSA_* as needed) before running the backend.",
      "",
    ].join("\n")
  : "";

const wtEnv =
  header +
  (baseContent.endsWith("\n") || baseContent === "" ? baseContent : `${baseContent}\n`) +
  "\n" +
  envOverrideBlock(cfg, baseEnv);
writeFileSync(path.join(wtDir, ".env"), wtEnv);

const frontendDir = path.join(wtDir, "apps", "frontend");
mkdirSync(frontendDir, { recursive: true });
writeFileSync(
  path.join(frontendDir, ".env.development.local"),
  `# Generated by make worktree-new (slot ${slot}) — the frontend reads BACKEND_URL\n# only from its own directory (proxy.ts, app/api route), never from the root .env.\nBACKEND_URL=http://localhost:${cfg.devBackendPort}\n`,
);

console.log(`Worktree ready: ${wtDir} (branch ${name}, slot ${slot})`);
if (fallbackExample) {
  console.log("WARNING: no root .env in this checkout — the worktree .env was copied from .env.example; fill in the real keys.");
}
console.log("");
console.log("  role   postgres  backend  frontend  compose project");
console.log(`  dev    ${cfg.devPgPort}      ${cfg.devBackendPort}     ${cfg.devFrontendPort}      ${cfg.composeProject}`);
console.log(`  e2e    ${cfg.e2ePgPort}      ${cfg.e2eBackendPort}     ${cfg.e2eFrontendPort}      ${cfg.e2eComposeProject}`);
console.log("");
console.log(`  DATABASE_URL=${slotDatabaseUrl(cfg, baseEnv)}`);
console.log(`  env: ${path.join(wtDir, ".env")}`);
console.log("");
console.log("Start (inside the worktree):");
console.log(`  cd ${wtDir}`);
console.log("  make local-infra-up && make migrate-up");
console.log(`  make frontend-dev      # http://localhost:${cfg.devFrontendPort}`);
console.log("  make frontend-e2e      # disposable e2e stack on this slot's ports");
