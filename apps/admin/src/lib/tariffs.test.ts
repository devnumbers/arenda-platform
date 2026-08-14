import { describe, expect, it } from 'vitest';
import { formatPropertyLimit, tariffName, tariffNameChoices } from './tariffs';

// Значения enum TariffName из apps/backend/api/openapi/openapi.yaml.
const openApiTariffNames = ['basic', 'pro', 'business'] as const;

describe('tariffNameChoices', () => {
  it('covers every OpenAPI TariffName value exactly once', () => {
    const ids = tariffNameChoices.map((choice) => choice.id);
    for (const name of openApiTariffNames) {
      expect(ids.filter((id) => id === name)).toHaveLength(1);
    }
    expect(ids).toHaveLength(openApiTariffNames.length);
  });

  it('every choice has a non-empty Russian label', () => {
    for (const choice of tariffNameChoices) {
      expect(choice.name.trim()).not.toBe('');
    }
  });
});

describe('tariffName', () => {
  it.each([
    ['basic', 'Базовый'],
    ['pro', 'Про'],
    ['business', 'Бизнес'],
  ] as const)('maps %s to %s', (name, label) => {
    expect(tariffName(name)).toBe(label);
  });

  it('passes unknown names through unchanged', () => {
    expect(tariffName('unlimited')).toBe('unlimited');
  });
});

describe('formatPropertyLimit', () => {
  it('renders the unlimited limit (-1) as «Безлимит»', () => {
    expect(formatPropertyLimit(-1)).toBe('Безлимит');
  });

  it('renders finite limits as plain numbers', () => {
    expect(formatPropertyLimit(0)).toBe('0');
    expect(formatPropertyLimit(1)).toBe('1');
    expect(formatPropertyLimit(5)).toBe('5');
    expect(formatPropertyLimit(100)).toBe('100');
  });
});
