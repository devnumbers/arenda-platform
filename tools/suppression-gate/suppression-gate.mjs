#!/usr/bin/env node
// Suppression gate (quality mode #378, gate ticket #399): a suppression
// directive or an explicit `any` in the manual TS/JS code of apps/frontend,
// apps/admin and apps/landing is a finding — the target state is 0
// suppressions, so there is no
// whitelist: a genuine exclusion belongs in the ESLint config (or an ADR-backed
// tool exclusion), not in a code comment. The ESLint rules already in force
// validate what exists; this gate enforces that nothing exists to validate —
// the same stance as the backend nolint gate (tools/nolint-gate, #344), whose
// package layout and exit contract this tool mirrors.
//
// Counted, per the ticket:
//   - `eslint-disable` in a comment (any spelling: bare, -line, -next-line,
//     rule lists; prose mentions count too — with 0 as the target, a comment
//     discussing a directive is equally stale);
//   - `@ts-ignore` / `@ts-expect-error` / `@ts-nocheck` in a comment;
//   - explicit `any` as a type in code.
//
// Comment/string-aware scanning, not grep (the nolint-gate/migration-lint
// decision): directives only exist in comments, and `any` is an English word —
// prose occurrences ("…has any overdue rent operation…", JSX copy "Choose any
// tariff") must not count, so strings, template literals and comments are
// skipped by a small scanner before matching. Known limits, accepted because a
// real parse needs the TypeScript compiler (a dependency the gate refuses to
// take): regex literals stay in code spans, template holes are skipped with
// their literal text, and a quote directly preceded by a word character or a
// closing bracket is treated as prose (don't, owners') rather than a string
// opener — legal code always spaces the quote token away from the previous one.
//
// `any` is matched in code spans only when both sides look like type syntax:
// the nearest significant code character before it is `: < , | & ; = [` (or
// `=>`), or the nearest preceding word is a type keyword (as/extends/keyof/
// infer/readonly/satisfies/is), AND the first significant character after it
// is `; , ) ] } > | & = ( { [` or nothing until end of line / end of file.
// Both sides are required so JSX copy like "Warning: any changes apply" (word
// after) or "Choose any tariff" (word before) stays clean; the deliberately
// strict residue — e.g. prose ": any," — is a rephrase-the-copy finding, the
// same strictness the nolint gate applies to prose mentions.
//
// Manual-code scope by path, so the pre-commit staged list and the CI full
// pass behave identically: node_modules, build artifacts (.next, dist, out,
// build, coverage), generated code (a `generated` path segment or file stem —
// apps/frontend/shared/api/generated.ts, apps/admin/src/lib/generated/**) and
// tool-managed env declarations (next-env.d.ts, vite-env.d.ts) are excluded —
// the counter looks only at hand-written code (.next/types alone carries 20
// `any` and 102 `@ts-ignore`).
//
// Contract: `node suppression-gate.mjs <file>...` → exit 0 (clean),
// 1 (findings printed to stdout), 2 (usage/IO error). A listed path that does
// not exist is skipped with a note — pre-commit passes staged deletions, and a
// deleted file cannot contain a suppression.

import { readFileSync } from "node:fs";

const RULE = "suppression-gate";

const MESSAGE =
  "a suppression in manual TS/JS code — the target state is 0 suppressions (issue #399); fix the code or record a config exclusion with an ADR instead";

// Directives live in comments by definition; matched case-insensitively.
const DIRECTIVE = /eslint-disable|@ts-(?:ignore|expect-error|nocheck)/gi;

const EXCLUDED_SEGMENTS = new Set([
  "node_modules",
  ".next",
  "dist",
  "out",
  "build",
  "coverage",
  "generated",
]);
const EXCLUDED_BASENAMES = new Set(["next-env.d.ts", "vite-env.d.ts"]);
const SOURCE_EXT = /\.(?:ts|tsx|js|jsx|mjs|cjs|mts|cts)$/;

// Type keywords that may legitimately precede `any` with only space between.
const TYPE_KEYWORDS = new Set([
  "as",
  "extends",
  "keyof",
  "infer",
  "readonly",
  "satisfies",
  "is",
]);

const LEFT_CONTEXT_CHARS = new Set([":", "<", ",", "|", "&", ";", "=", "["]);
const RIGHT_CONTEXT_CHARS = new Set([
  ";",
  ",",
  ")",
  "]",
  "}",
  ">",
  "|",
  "&",
  "=",
  "(",
  "{",
  "[",
]);

function isWordChar(ch) {
  return ch !== undefined && /[\w$]/.test(ch);
}

function isExcludedPath(file) {
  const segments = file.split(/[/\\]/);
  const base = segments[segments.length - 1];
  if (EXCLUDED_BASENAMES.has(base)) return true;
  if (segments.some((segment) => EXCLUDED_SEGMENTS.has(segment))) return true;
  return base.replace(/\.[^.]+$/, "") === "generated";
}

// A quote starts a string unless the immediately preceding character is a word
// character or a closing bracket/quote — the shapes prose apostrophes take
// (don't, owners') while legal code always spaces the quote token.
function opensString(src, i) {
  if (i === 0) return true;
  return !/[\w$)\]"']/.test(src[i - 1]);
}

// Splits the source into code and comment spans. Strings and template
// literals (holes included — see the header for the accepted limit) are
// consumed silently; JSX {/* … */} comments arrive as ordinary block comments.
function scanSpans(src) {
  const spans = [];
  let i = 0;
  let codeStart = 0;
  const flushCode = (end) => {
    if (end > codeStart) spans.push({ kind: "code", start: codeStart, end });
  };

  while (i < src.length) {
    const ch = src[i];
    if (ch === "/" && src[i + 1] === "/") {
      flushCode(i);
      const start = i;
      i += 2;
      while (i < src.length && src[i] !== "\n") i++;
      spans.push({ kind: "comment", start, end: i });
      codeStart = i;
    } else if (ch === "/" && src[i + 1] === "*") {
      flushCode(i);
      const start = i;
      i += 2;
      while (i < src.length && !(src[i] === "*" && src[i + 1] === "/")) i++;
      i = i < src.length ? i + 2 : i;
      spans.push({ kind: "comment", start, end: i });
      codeStart = i;
    } else if ((ch === '"' || ch === "'") && opensString(src, i)) {
      flushCode(i);
      i++;
      while (i < src.length && src[i] !== ch) {
        if (src[i] === "\\") i++;
        i++;
      }
      i++;
      codeStart = i;
    } else if (ch === "`") {
      flushCode(i);
      i++;
      while (i < src.length && src[i] !== "`") {
        if (src[i] === "\\") i++;
        i++;
      }
      i++;
      codeStart = i;
    } else {
      i++;
    }
  }
  flushCode(src.length);
  return spans;
}

// All non-whitespace characters of code spans in index order — the significant
// stream both `any`-context sides are resolved against (comments and strings
// between code contribute nothing, exactly like a parser's token stream).
function significantChars(src, spans) {
  const chars = [];
  for (const span of spans) {
    if (span.kind !== "code") continue;
    for (let i = span.start; i < span.end; i++) {
      const ch = src[i];
      if (!/\s/.test(ch)) chars.push({ ch, index: i });
    }
  }
  return chars;
}

// True when the `any` at [start, end) of src sits in type syntax on both sides
// (header has the rationale). sigChars is the significant stream of the file.
function isTypeAny(src, sigChars, start, end) {
  // Left side: nearest significant char before the match (and the identifier
  // word ending there, if any).
  let pos = 0;
  let count = 0;
  while (pos < sigChars.length && sigChars[pos].index < start) {
    pos++;
    count++;
  }
  if (count === 0) return false;
  const left = sigChars[count - 1];
  let leftOk = LEFT_CONTEXT_CHARS.has(left.ch);
  if (!leftOk && left.ch === ">" && count >= 2 && sigChars[count - 2].ch === "=") {
    leftOk = true; // => return type
  }
  if (!leftOk) {
    // The word is contiguous word characters in the raw source (whitespace
    // separates tokens; the significant stream has it stripped).
    let word = "";
    for (let j = left.index; j >= 0 && isWordChar(src[j]); j--) word = src[j] + word;
    leftOk = TYPE_KEYWORDS.has(word);
  }
  if (!leftOk) return false;

  // Right side: first significant char after the match — crossing intervening
  // comments/strings — must be type syntax, unless nothing significant is left
  // on the line (a declaration ending in `: any` at end of line) or at all
  // (end of file). Skip the match's own letters first.
  while (pos < sigChars.length && sigChars[pos].index < end) pos++;
  const next = pos < sigChars.length ? sigChars[pos] : null;
  if (next === null) return true;
  if (RIGHT_CONTEXT_CHARS.has(next.ch)) return true;
  for (let i = end; i < src.length && src[i] !== "\n"; i++) {
    if (!/\s/.test(src[i])) return false;
  }
  return true;
}

function lintSource(src) {
  const findings = [];
  const spans = scanSpans(src);
  const sigChars = significantChars(src, spans);

  for (const span of spans) {
    const text = src.slice(span.start, span.end);
    if (span.kind === "comment") {
      DIRECTIVE.lastIndex = 0;
      let m;
      while ((m = DIRECTIVE.exec(text)) !== null) {
        findings.push({ index: span.start + m.index, kind: m[0].toLowerCase() });
      }
      continue;
    }
    for (let i = span.start; i < span.end - 2; i++) {
      if (src.slice(i, i + 3) !== "any") continue;
      if (isWordChar(src[i - 1]) || isWordChar(src[i + 3])) continue;
      if (isTypeAny(src, sigChars, i, i + 3)) {
        findings.push({ index: i, kind: "explicit any" });
      }
      i += 2;
    }
  }
  return findings;
}

function lineOf(src, index) {
  let line = 1;
  for (let i = 0; i < index; i++) if (src[i] === "\n") line++;
  return line;
}

function main() {
  const files = process.argv.slice(2);
  if (files.length === 0) {
    console.error("usage: node suppression-gate.mjs <file>...");
    process.exit(2);
  }

  let checked = 0;
  let excluded = 0;
  let skipped = 0;
  let total = 0;
  for (const file of files) {
    if (!SOURCE_EXT.test(file)) {
      skipped++;
      continue;
    }
    if (isExcludedPath(file)) {
      excluded++;
      continue;
    }

    let src;
    try {
      src = readFileSync(file, "utf8");
    } catch (err) {
      if (err.code === "ENOENT") {
        console.log(`suppression-gate: skipped ${file} (no such file — staged deletion?)`);
        skipped++;
        continue;
      }
      console.error(`suppression-gate: cannot read ${file}: ${err.message}`);
      process.exit(2);
    }
    checked++;
    for (const finding of lintSource(src)) {
      console.log(`${file}:${lineOf(src, finding.index)}: ${RULE}: ${finding.kind}: ${MESSAGE}`);
      total++;
    }
  }

  if (total > 0) {
    console.log(
      `suppression-gate: ${total} finding(s) in ${checked} checked file(s), ${excluded} excluded, ${skipped} skipped`,
    );
    process.exit(1);
  }
  console.log(`suppression-gate: ok (${checked} checked, ${excluded} excluded, ${skipped} skipped)`);
}

main();
