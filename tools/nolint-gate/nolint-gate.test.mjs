// Contract tests for the nolint gate: .go file paths as args → exit 0
// (clean) / 1 (findings printed) / 2 (usage error). The script is spawned as
// a subprocess exactly the way `make backend-nolint` runs it; internal
// structure is not imported or tested.
//
// Fixtures are written to a temp dir per test: the contract is about file
// content → verdict, and inline Go keeps each case readable next to its
// assertion.
import { describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const scriptPath = path.join(import.meta.dirname, "nolint-gate.mjs");

function fixture(name, go) {
  const dir = mkdtempSync(path.join(tmpdir(), "nolint-gate-"));
  const file = path.join(dir, name);
  writeFileSync(file, go);
  return file;
}

function runGate(files) {
  const args = Array.isArray(files) ? files : [files];
  const res = spawnSync(process.execPath, [scriptPath, ...args], {
    encoding: "utf8",
    timeout: 15_000,
  });
  return { code: res.status, stdout: res.stdout, stderr: res.stderr };
}

describe("nolint-gate: directives in comments are findings", () => {
  const spellings = [
    ["bare", "//nolint"],
    ["spaced", "// nolint"],
    ["linter-specific", "//nolint:gosec"],
    ["multi-linter", "//nolint:gosec,errcheck"],
    ["with explanation", "//nolint:gosec // third-party sha"],
    ["after prose", "// legacy quirk, keep //nolint:gosec"],
    ["uppercase", "//NoLint"],
  ];

  for (const [label, directive] of spellings) {
    it(`flags a ${label} directive`, () => {
      const file = fixture("bad.go", `package x\n\nfunc f() { _ = 1 }\n${directive}\n`);
      const res = runGate(file);
      expect(res.code).toBe(1);
      expect(res.stdout).toContain("nolint-gate");
      expect(res.stdout).toContain("bad.go:4:");
    });
  }

  it("flags a directive inside a block comment", () => {
    const file = fixture("bad.go", "package x\n\n/* nolint:gosec */\nfunc f() { _ = 1 }\n");
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("bad.go:3:");
  });

  it("flags a multi-line block comment on the line of the match", () => {
    const file = fixture(
      "bad.go",
      "package x\n\n/*\nlegacy block\n//nolint:gosec\n*/\nfunc f() { _ = 1 }\n",
    );
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("bad.go:5:");
  });

  it("does not flag the word nolinter (word boundary)", () => {
    const file = fixture("ok.go", "package x\n\n// nolinter of last resort\nfunc f() { _ = 1 }\n");
    expect(runGate(file).code).toBe(0);
  });
});

describe("nolint-gate: string-aware scanning (occurrences outside comments are not directives)", () => {
  it("ignores //nolint inside an interpreted string literal", () => {
    const file = fixture("ok.go", 'package x\n\nfunc f() { s := "//nolint:gosec"; _ = s }\n');
    expect(runGate(file).code).toBe(0);
  });

  it("ignores //nolint inside a raw string literal", () => {
    const file = fixture("ok.go", "package x\n\nfunc f() { s := `raw //nolint`; _ = s }\n");
    expect(runGate(file).code).toBe(0);
  });

  it("is not confused by an escaped quote or a quote inside a rune", () => {
    const file = fixture(
      "ok.go",
      'package x\n\nfunc f() { q := \'"\' ; s := "\\\\"; _, _ = q, s }\n',
    );
    expect(runGate(file).code).toBe(0);
  });

  it("keeps scanning after a string: a later directive is still caught", () => {
    const file = fixture(
      "bad.go",
      'package x\n\nfunc f() { s := "//not a directive"; _ = s }\n//nolint:gosec\n',
    );
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("bad.go:4:");
  });

  it("a URL containing nolint in a comment is still flagged (target is 0 mentions)", () => {
    const file = fixture("bad.go", "package x\n\n// see https://golangci-lint.run/docs/nolint/\nfunc f() { _ = 1 }\n");
    const res = runGate(file);
    expect(res.code).toBe(1);
  });
});

describe("nolint-gate: scope and contract", () => {
  it("clean Go file passes", () => {
    const file = fixture(
      "ok.go",
      "package x\n\n// honest comment with a / slash and \"quotes\" in prose\nfunc f() { s := \"/\"; _ = s }\n",
    );
    const res = runGate(file);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("ok");
  });

  it("reports per-file findings across a multi-file run", () => {
    const clean = fixture("a_ok.go", "package a\n");
    const dirty = fixture("b_bad.go", "package b\n\n//nolint\n");
    const res = runGate([clean, dirty]);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("b_bad.go");
    expect(res.stdout).not.toContain("a_ok.go:1");
  });

  it("skips a listed path that does not exist (staged deletion)", () => {
    const file = fixture("ok.go", "package x\n");
    const res = runGate([file, path.join(path.dirname(file), "deleted.go")]);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("skipped");
  });

  it("exits 2 with usage when given no files", () => {
    const res = runGate([]);
    expect(res.code).toBe(2);
    expect(res.stderr).toContain("usage");
  });
});
