import type { PropertyAttributes } from './types';

/**
 * Coerces an untyped (e.g. from JSON / sessionStorage / OpenAPI `unknown`)
 * value into a PropertyAttributes record, keeping only entries whose value is
 * a string or number. Null and other non-scalar values are dropped. Returns an
 * empty record for null/non-object/undefined inputs.
 */
export function coerceAttributes(raw: unknown): PropertyAttributes {
  if (raw === null || typeof raw !== 'object') return {};
  const result: Record<string, string | number> = {};
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    if (typeof value === 'string' || typeof value === 'number') {
      result[key] = value;
    }
  }
  return result;
}
