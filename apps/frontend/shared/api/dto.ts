/**
 * Единственная точка импорта OpenAPI-генерированных DTO за пределами shared/api.
 * Клиент перегенерируется командой `npm run generate:api`; сам файл generated.ts
 * импортируется только внутри shared/api (enforced by no-restricted-imports в
 * eslint.config.mjs).
 */
export type { components, operations } from './generated';
