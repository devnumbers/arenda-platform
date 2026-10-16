#!/usr/bin/env node
// Nolint gate (remediation grid #325, gate ticket #344): any `nolint` word in
// a Go comment under apps/backend is a finding — the target state is 0
// suppression directives (#323), so there is no whitelist: a genuine
// exclusion belongs in .golangci.yml with an ADR, not in a code comment.
// golangci-lint's nolintlint only validates the form of directives that
// exist; this gate enforces that none exist.
//
// Go-aware scanning, the same statement-aware-not-grep decision as the
// migration domain rules (tools/migration-lint/domain-rules.mjs): comments
// are scanned in both forms — line (where golangci-lint honors //nolint,
// including one trailing after prose) and block (golangci-lint itself only
// parses line comments; this gate is deliberately stricter, per the
// 0-mentions policy) — while occurrences inside string literals ("...", `...`)
// and runes ('…') are not directives. The word is matched case-insensitively
// with word boundaries, so prose mentions and links count too — with 0 as
// the target, a comment discussing nolint is equally stale.
//
// Contract: `node nolint-gate.mjs <file.go>...` → exit 0 (clean),
// 1 (findings printed to stdout), 2 (usage/IO error). A listed path that
// does not exist is skipped with a note — pre-commit passes staged
// deletions, and a deleted file cannot contain a directive.

import { readFileSync } from "node:fs";

const RULE = "nolint-gate";

const MESSAGE =
  '"nolint" in a comment is a suppressed finding — the target state is 0 nolint directives (issue #323); fix the code or record a config exclusion with an ADR instead';

const NO_LINT = /\bnolint\b/gi;

// Matches of the banned word inside one comment's text; startLine is the line
// where the comment opens, newlines inside the text (block comments) advance
// the reported line.
function commentFindings(text, startLine) {
  const findings = [];
  let m;
  NO_LINT.lastIndex = 0;
  while ((m = NO_LINT.exec(text)) !== null) {
    let line = startLine;
    for (let k = 0; k < m.index; k++) if (text[k] === "\n") line++;
    findings.push({ line });
  }
  return findings;
}

// Walks the source with a small Go lexer (comments, interpreted strings,
// raw strings, runes — backslash escapes respected) and collects findings
// from comment spans only.
function lintSource(src) {
  const findings = [];
  let i = 0;
  let line = 1;
  while (i < src.length) {
    const ch = src[i];
    if (ch === "\n") {
      line++;
      i++;
    } else if (ch === "/" && src[i + 1] === "/") {
      const start = i;
      i += 2;
      while (i < src.length && src[i] !== "\n") i++;
      findings.push(...commentFindings(src.slice(start, i), line));
    } else if (ch === "/" && src[i + 1] === "*") {
      const start = i;
      const startLine = line;
      i += 2;
      while (i < src.length && !(src[i] === "*" && src[i + 1] === "/")) {
        if (src[i] === "\n") line++;
        i++;
      }
      const end = i < src.length ? i + 2 : i;
      findings.push(...commentFindings(src.slice(start, end), startLine));
      i = end;
    } else if (ch === '"' || ch === "'") {
      const quote = ch;
      i++;
      while (i < src.length && src[i] !== quote) {
        if (src[i] === "\\") i++;
        if (src[i] === "\n") line++; // illegal in Go, harmless to count
        i++;
      }
      i++;
    } else if (ch === "`") {
      i++;
      while (i < src.length && src[i] !== "`") {
        if (src[i] === "\n") line++;
        i++;
      }
      i++;
    } else {
      i++;
    }
  }
  return findings;
}

function main() {
  const files = process.argv.slice(2);
  if (files.length === 0) {
    console.error("usage: node nolint-gate.mjs <file.go>...");
    process.exit(2);
  }

  let checked = 0;
  let skipped = 0;
  let total = 0;
  for (const file of files) {
    let src;
    try {
      src = readFileSync(file, "utf8");
    } catch (err) {
      if (err.code === "ENOENT") {
        console.log(`nolint-gate: skipped ${file} (no such file — staged deletion?)`);
        skipped++;
        continue;
      }
      console.error(`nolint-gate: cannot read ${file}: ${err.message}`);
      process.exit(2);
    }
    checked++;
    const findings = lintSource(src);
    total += findings.length;
    for (const f of findings) {
      console.log(`${file}:${f.line}: ${RULE}: ${MESSAGE}`);
    }
  }

  if (total > 0) {
    console.log(`nolint-gate: ${total} finding(s) in ${checked} checked file(s), ${skipped} skipped`);
    process.exit(1);
  }
  console.log(`nolint-gate: ok (${checked} checked, ${skipped} skipped)`);
}

main();
