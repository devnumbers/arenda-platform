import { defineConfig } from 'vitest/config';
import { fileURLToPath } from 'node:url';
import { dirname } from 'node:path';

// vitest config for apps/frontend.
// Mirrors the tsconfig `paths` mapping: "@/*" -> "./*" (relative to apps/frontend root).
// We test pure logic (validate/catalog/format), so environment is node, no DOM.
// Uses .mts because apps/frontend/package.json has no "type": "module".
const root = dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  resolve: {
    alias: {
      '@': root,
    },
  },
  test: {
    environment: 'node',
    include: ['**/*.test.ts'],
  },
});
