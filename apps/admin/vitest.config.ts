import { defineConfig } from 'vitest/config';

// vitest config for apps/admin.
// Tests cover pure logic in src/lib (catalog, labels, formatting, isPropertyType).
// environment: node (no DOM). No path aliases are used by the module under test.
export default defineConfig({
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
});
