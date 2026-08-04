// Optional gofmt pass for the generated Go file. If the `gofmt` binary is on
// PATH, the generated source is piped through it so the emitted file is
// canonical Go (aligned struct/map literals, no stray blank lines). If gofmt is
// not available (e.g. running on a machine without the Go toolchain), the
// unformatted source is returned unchanged and a one-line warning is printed.
//
// We intentionally do NOT make gofmt a hard dependency: this generator also
// emits TypeScript, and TS developers should be able to regenerate the catalog
// without installing Go. The Go-side CI/Makefile can always run `gofmt -w` on
// the artifact as a verification step.

import { spawnSync } from 'node:child_process';

export function gofmt(src) {
  const res = spawnSync('gofmt', { input: src, encoding: 'utf8' });
  if (res.error || res.status !== 0) {
    if (res.error && res.error.code === 'ENOENT') {
      console.warn(
        'gofmt: binary not found on PATH — Go artifact emitted unformatted.\n' +
          '  Run `gofmt -w apps/backend/internal/properties/domain/zz_catalog.gen.go` to normalise.',
      );
      return src;
    }
    const detail = (res.stderr || res.stdout || '').trim();
    throw new Error(`gofmt failed (status ${res.status}): ${detail}`);
  }
  return res.stdout;
}
