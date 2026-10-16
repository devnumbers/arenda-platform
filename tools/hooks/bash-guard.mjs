#!/usr/bin/env node
// PreToolUse(Bash) guard: blocks destructive shell commands before they run.
//
// Harness-neutral contract: hook JSON on stdin → exit 0 (allow) / 2 (block,
// stderr carries the reason the model sees). Any payload or internal error
// fails open (exit 0). Like the harness hook mechanism itself, this is a
// safety net against common agent mistakes, not a security barrier: command
// segmentation is best-effort (quotes, substitutions and command chains are
// not parsed by a real shell).
//
// Deliberately repo-agnostic: the user-level harness config fires this hook
// in every session, so the guard protects any project on the machine, not
// just this monorepo.

import { readFileSync } from "node:fs";

// `$(` is treated as a segment boundary so command substitutions
// (`echo $(rm -rf x)`) are checked too; quoted literals are not executed,
// but distinguishing them needs a real shell parser — accepted over-blocking.
const SEGMENT_SPLIT = /&&|\|\||[;|&\n]|\$\(/;
const ENV_ASSIGNMENT = /^[A-Za-z_][A-Za-z0-9_]*=/;
// Prefixes that wrap another command; flags seen before the command word
// (env -i, xargs -0, sudo -n) belong to the wrapper, not the payload.
const PASS_THROUGH = new Set(["sudo", "env", "nice", "nohup", "time", "xargs"]);
const GIT_GLOBAL_FLAGS_WITH_VALUE = new Set([
  "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path", "--super-prefix",
]);
const DOCKER_GLOBAL_FLAGS_WITH_VALUE = new Set(["-H", "--host", "--context", "--config", "--log-level"]);
const COMPOSE_FLAGS_WITH_VALUE = new Set([
  "-f", "--file", "-p", "--project-name", "--project-directory", "--env-file",
]);

function rule(id, hint) {
  return { rule: id, hint };
}

// Resolves the first command word of a segment: strips subshell/brace
// punctuation, env assignments, sudo/env/xargs-style wrappers and their flags.
function resolveCommand(tokens) {
  for (let i = 0; i < tokens.length; i++) {
    const tok = tokens[i].replace(/^[$({`]+/, "");
    if (tok === "" || ENV_ASSIGNMENT.test(tok) || PASS_THROUGH.has(tok) || tok.startsWith("-")) continue;
    const parts = tok.split("/");
    return { word: parts[parts.length - 1] || tok, rest: tokens.slice(i + 1) };
  }
  return null;
}

// Finds the subcommand after the binary's global flags (`git -C apps/backend
// clean`); flags that take a separate value skip that value too.
function resolveSubcommand(tokens, flagsWithValue) {
  for (let i = 0; i < tokens.length; i++) {
    const tok = tokens[i];
    if (!tok.startsWith("-")) return { sub: tok, rest: tokens.slice(i + 1) };
    if (!tok.includes("=") && flagsWithValue.has(tok)) i++;
  }
  return { sub: null, rest: [] };
}

function hasShortFlag(tokens, letter) {
  return tokens.some((tok) => new RegExp(`^-[a-zA-Z]*${letter}`).test(tok));
}

function downWithVolumes(args) {
  return args.includes("-v") || args.includes("--volumes");
}

function checkComposeDownVolumes(subcommand, args) {
  if (subcommand !== "down" || !downWithVolumes(args)) return null;
  return rule(
    "docker compose down -v",
    "docker compose down -v deletes the project's named volumes (local databases); drop -v or ask the user",
  );
}

function hasRecursiveRmFlag(tokens) {
  for (const tok of tokens) {
    if (tok === "--") break;
    if (tok === "--recursive" || /^-[a-zA-Z]*[rR]/.test(tok)) return true;
  }
  return false;
}

function checkCommand(word, rest, depth) {
  switch (word) {
    case "rm":
      if (hasRecursiveRmFlag(rest)) {
        return rule(
          "recursive rm",
          "rm with -r/-R/--recursive is blocked; delete individual files, or ask the user for recursive deletion",
        );
      }
      break;
    case "git": {
      const { sub, rest: args } = resolveSubcommand(rest, GIT_GLOBAL_FLAGS_WITH_VALUE);
      if (sub === "clean" && !hasShortFlag(args, "n") && !args.includes("--dry-run")) {
        return rule("git clean", "git clean without -n/--dry-run wipes untracked files; preview with -n first");
      }
      if (sub === "reset" && args.includes("--hard")) {
        return rule("git reset --hard", "git reset --hard discards uncommitted work; use --soft/--keep or ask the user");
      }
      if (sub === "push" && (args.includes("--force") || args.includes("-f"))) {
        return rule(
          "git push --force",
          "git push --force rewrites remote history; use --force-with-lease or ask the user",
        );
      }
      break;
    }
    case "docker": {
      const { sub, rest: args } = resolveSubcommand(rest, DOCKER_GLOBAL_FLAGS_WITH_VALUE);
      if (sub === "system" || sub === "volume") {
        const inner = resolveSubcommand(args, new Set()).sub;
        if (inner === "prune") {
          return rule(
            "docker prune",
            "docker system/volume prune deletes data of unrelated projects; remove specific objects instead",
          );
        }
      }
      if (sub === "compose") {
        const { sub: inner, rest: innerArgs } = resolveSubcommand(args, COMPOSE_FLAGS_WITH_VALUE);
        const violation = checkComposeDownVolumes(inner, innerArgs);
        if (violation) return violation;
      }
      break;
    }
    case "docker-compose": {
      const { sub, rest: args } = resolveSubcommand(rest, COMPOSE_FLAGS_WITH_VALUE);
      const violation = checkComposeDownVolumes(sub, args);
      if (violation) return violation;
      break;
    }
    // `bash -c "rm -rf x"` hides a whole command in one string argument.
    case "bash":
    case "sh":
    case "zsh": {
      if (depth === 0) {
        const cIndex = rest.indexOf("-c");
        if (cIndex !== -1 && rest.length > cIndex + 1) {
          const inner = rest
            .slice(cIndex + 1)
            .join(" ")
            .replace(/^['"]/, "")
            .replace(/['"]$/, "");
          const violation = findViolation(inner, depth + 1);
          if (violation) return violation;
        }
      }
      break;
    }
  }
  return null;
}

function findViolation(command, depth = 0) {
  for (const segment of command.split(SEGMENT_SPLIT)) {
    const tokens = segment.trim().split(/\s+/).filter(Boolean);
    const resolved = resolveCommand(tokens);
    if (!resolved) continue;
    const violation = checkCommand(resolved.word, resolved.rest, depth);
    if (violation) return { ...violation, segment: segment.trim() };
  }
  return null;
}

function main() {
  let payload;
  try {
    payload = JSON.parse(readFileSync(0, "utf8"));
  } catch {
    return; // no or invalid payload → allow (fail-open)
  }
  // Kimi Code sends tool_input, ZCode duplicates the fields in camelCase too.
  const command = payload?.tool_input?.command ?? payload?.toolInput?.command;
  if (typeof command !== "string" || command === "") return;

  const violation = findViolation(command);
  if (!violation) return;

  const segment = violation.segment.length > 200 ? `${violation.segment.slice(0, 200)}…` : violation.segment;
  console.error(`bash-guard: blocked [${violation.rule}] in: ${segment}`);
  console.error(`bash-guard: ${violation.hint}`);
  process.exit(2);
}

main();
