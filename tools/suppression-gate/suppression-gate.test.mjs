// Contract tests for the suppression gate: file paths as args → exit 0
// (clean) / 1 (findings printed) / 2 (usage error). The script is spawned as
// a subprocess exactly the way `make ts-suppressions` runs it; internal
// structure is not imported or tested.
//
// Fixtures are written to a temp dir per test: the contract is about file
// content → verdict, and inline TS keeps each case readable next to its
// assertion. Fixture paths deliberately mirror the generated/artifact shapes
// of both apps (shared/api/generated.ts, src/lib/generated/**, .next/types)
// so the exclusion rules are pinned to the real directory layout.
import { describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";

const scriptPath = path.join(import.meta.dirname, "suppression-gate.mjs");

function fixture(rel, content) {
  const dir = mkdtempSync(path.join(tmpdir(), "suppression-gate-"));
  const file = path.join(dir, rel);
  mkdirSync(path.dirname(file), { recursive: true });
  writeFileSync(file, content);
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

describe("suppression-gate: comment directives are findings", () => {
  const directives = [
    ["bare eslint-disable", "// eslint-disable"],
    ["next-line", "// eslint-disable-next-line react-hooks/exhaustive-deps"],
    ["with rule list", "// eslint-disable-next-line @next/next/no-img-element, no-console -- reason"],
    ["block comment", "/* eslint-disable react-hooks/set-state-in-effect */"],
    ["ts-ignore", "// @ts-ignore"],
    ["ts-expect-error", "// @ts-expect-error cannot type third-party"],
  ];

  for (const [label, directive] of directives) {
    it(`flags a ${label} directive`, () => {
      const file = fixture("bad.tsx", `export const x = 1;\n${directive}\nexport const y = 2;\n`);
      const res = runGate(file);
      expect(res.code).toBe(1);
      expect(res.stdout).toContain("suppression-gate");
      expect(res.stdout).toContain("bad.tsx:2:");
    });
  }

  it("flags a directive inside a JSX block comment", () => {
    const file = fixture(
      "grid.tsx",
      `export function Grid() {\n  return (\n    <div>\n      {/* eslint-disable-next-line @next/next/no-img-element */}\n      <img src="x" alt="" />\n    </div>\n  );\n}\n`,
    );
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("grid.tsx:4:");
  });

  it("flags a directive mentioned in prose of a comment (0-mentions policy)", () => {
    const file = fixture("prose.ts", `// legacy note: the old eslint-disable was removed, this one is new\nexport const x = 1;\n`);
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("prose.ts:1:");
  });
});

describe("suppression-gate: explicit any in code is a finding", () => {
  const cases = [
    ["variable annotation", "const x: any = f();"],
    ["optional property", "type T = { a?: any };"],
    ["record value", "let m: Record<string, any>;"],
    ["as-cast", "const v = input as any;"],
    ["return type", "function f(): any { return 1; }"],
    ["arrow return type", "const g = (x: string): any => x;"],
    ["generic argument", "const r = useRef<any>(null);"],
    ["array type", "let a: any[] = [];"],
    ["type alias", "type U = any;"],
    ["union member", "type V = string | any;"],
    ["readonly array", "declare const w: readonly any[];"],
    ["type predicate", "declare function isAny(x: unknown): x is any;"],
  ];

  for (const [label, line] of cases) {
    it(`flags ${label}`, () => {
      const file = fixture("bad.ts", `export const n = 1;\n${line}\n`);
      const res = runGate(file);
      expect(res.code).toBe(1);
      expect(res.stdout).toContain("explicit any");
      expect(res.stdout).toContain("bad.ts:2:");
    });
  }

  it("flags any after a line break between annotation and type", () => {
    const file = fixture("bad.ts", "declare let x:\n  any;\n");
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("bad.ts:2:");
  });
});

describe("suppression-gate: prose and literals are not findings", () => {
  it("ignores the English word any in a comment", () => {
    const file = fixture("ok.ts", "// true when the lease has any overdue rent operation\nexport const x = 1;\n");
    expect(runGate(file).code).toBe(0);
  });

  it("ignores the English word any in JSX prose", () => {
    const file = fixture(
      "ok.tsx",
      `export function W() {\n  return <p>Choose any tariff — Warning: any changes apply</p>;\n}\n`,
    );
    expect(runGate(file).code).toBe(0);
  });

  it("ignores any inside string literals", () => {
    const file = fixture("ok.ts", `const s = "cast: any, honestly";\nexport const x = s;\n`);
    expect(runGate(file).code).toBe(0);
  });

  it("ignores any inside template literal text (holes included, documented limit)", () => {
    const file = fixture("ok.ts", "const t = `value: any, ${1 + 1}`;\nexport const x = t;\n");
    expect(runGate(file).code).toBe(0);
  });

  it("keeps scanning after a string: a later annotation is still caught", () => {
    const file = fixture("bad.ts", `const s = ": any not here";\nconst x: any = s;\n`);
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("bad.ts:2:");
  });

  it("prose apostrophes do not break comment detection later in the file", () => {
    const file = fixture(
      "bad.tsx",
      `export function P() {\n  return <p>Don't lose any data — owners' keys stay</p>;\n}\n// @ts-ignore\nexport const y = 2;\n`,
    );
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("bad.tsx:4:");
  });

  it("a directive-looking string literal is not a directive", () => {
    const file = fixture("ok.ts", `const doc = "// eslint-disable prose in a string";\nexport const x = doc;\n`);
    expect(runGate(file).code).toBe(0);
  });

  it("anyOf and similar identifiers are not the any keyword", () => {
    const file = fixture("ok.ts", `export const t = { types: { anyOf: ["a"] } };\nexport const u = manyThings("x");\n`);
    expect(runGate(file).code).toBe(0);
  });
});

describe("suppression-gate: generated, artifacts and node_modules are excluded", () => {
  const dirty = `const x: any = 1; // eslint-disable\nexport default x;\n`;

  it("excludes a generated path segment (admin src/lib/generated)", () => {
    const file = fixture(path.join("src/lib/generated/api.ts"), dirty);
    const res = runGate(file);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("1 excluded");
  });

  it("excludes a generated basename (frontend shared/api/generated.ts)", () => {
    const file = fixture(path.join("shared/api/generated.ts"), dirty);
    expect(runGate(file).code).toBe(0);
  });

  it("excludes the .next build artifacts (.next/types)", () => {
    const file = fixture(path.join(".next/types/app/page.ts"), dirty);
    expect(runGate(file).code).toBe(0);
  });

  it("excludes node_modules and dist/out/build artifacts", () => {
    for (const rel of ["node_modules/pkg/index.js", "dist/bundle.js", "out/page.js", "build/lib.js"]) {
      const file = fixture(rel, dirty);
      const res = runGate(file);
      expect(res.code, rel).toBe(0);
      expect(res.stdout).toContain("1 excluded");
    }
  });

  it("excludes tool-managed env declarations", () => {
    for (const name of ["next-env.d.ts", "vite-env.d.ts"]) {
      expect(runGate(fixture(name, dirty)).code, name).toBe(0);
    }
  });

  it("counts a finding in an identical file under a manual path", () => {
    const file = fixture(path.join("src/lib/api.ts"), dirty);
    const res = runGate(file);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("src/lib/api.ts");
  });
});

describe("suppression-gate: scope and contract", () => {
  it("clean file passes", () => {
    const file = fixture("ok.ts", `import path from "node:path";\n\nexport const clean = path.join("a", "b");\n`);
    const res = runGate(file);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("ok");
  });

  it("reports per-file findings across a multi-file run", () => {
    const clean = fixture("a_ok.ts", "export const a = 1;\n");
    const dirty = fixture("b_bad.ts", "const b: any = 2;\nexport default b;\n");
    const res = runGate([clean, dirty]);
    expect(res.code).toBe(1);
    expect(res.stdout).toContain("b_bad.ts");
    expect(res.stdout).not.toContain("a_ok.ts:");
  });

  it("skips a listed path that does not exist (staged deletion)", () => {
    const file = fixture("ok.ts", "export const a = 1;\n");
    const res = runGate([file, path.join(path.dirname(file), "deleted.tsx")]);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("skipped");
  });

  it("skips non-source extensions", () => {
    const file = fixture("README.md", "contains : any and // eslint-disable in prose");
    const res = runGate(file);
    expect(res.code).toBe(0);
    expect(res.stdout).toContain("1 skipped");
  });

  it("exits 2 with usage when given no files", () => {
    const res = runGate([]);
    expect(res.code).toBe(2);
    expect(res.stderr).toContain("usage");
  });
});
