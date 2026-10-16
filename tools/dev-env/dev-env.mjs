#!/usr/bin/env node
// Slot queries for the Makefile (docs/agents/parallel-dev.md). The Makefile
// evaluates these at parse time and inside recipes:
//   $(shell node tools/dev-env/dev-env.mjs compose-project)  → -p for COMPOSE_LOCAL
//   $(shell node tools/dev-env/dev-env.mjs frontend-port)    → next dev port
// Root defaults to the cwd — make always runs from the checkout root, so the
// queried .env is that checkout's own.
import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { parseEnvFile, slotConfig, slotFromEnv } from "./lib.mjs";

const USAGE = `usage: node dev-env.mjs <command> [root]

commands:
  compose-project [root]  compose project name for this checkout (arenda-local / arenda-wtN)
  frontend-port [root]    frontend dev-server port (3000 / 3020+N)
`;

function readRootEnv(root) {
  const envPath = path.join(root, ".env");
  if (!existsSync(envPath)) return {};
  return parseEnvFile(readFileSync(envPath, "utf8"));
}

// A malformed slot claim is a corrupt registry entry: name the file on stderr
// and exit 1 instead of falling back to main-checkout values (that would put
// the worktree stack into the arenda-local project — the exact collision the
// slots exist to prevent).
function slotOrDie(root) {
  try {
    return slotFromEnv(readRootEnv(root));
  } catch (err) {
    console.error(`${path.join(root, ".env")}: ${err.message}`);
    process.exit(1);
  }
}

const [command, root = process.cwd()] = process.argv.slice(2);

switch (command) {
  case "compose-project": {
    const slot = slotOrDie(root);
    console.log(slotConfig(slot ?? 0).composeProject);
    break;
  }
  case "frontend-port": {
    const slot = slotOrDie(root);
    console.log(slotConfig(slot ?? 0).devFrontendPort);
    break;
  }
  default:
    process.stderr.write(USAGE);
    process.exit(2);
}
