import { describe, expect, it } from 'vitest';
import { readIsoRangeParam } from './parse-iso-range-param';

const TODAY = '2026-09-23';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

describe('readIsoRangeParam', () => {
  it('читает диапазон из пары параметров', () => {
    expect(readIsoRangeParam(paramsOf({ from: '2026-08-01', to: '2026-08-20' }), 'from', 'to', TODAY)).toEqual({
      from: '2026-08-01',
      to: '2026-08-20',
    });
  });

  it('неполная пара, битые даты, перевёрнутый порядок — null', () => {
    expect(readIsoRangeParam(paramsOf({ from: '2026-08-01' }), 'from', 'to', TODAY)).toBeNull();
    expect(readIsoRangeParam(paramsOf({ from: '2026-13-40', to: '2026-08-20' }), 'from', 'to', TODAY)).toBeNull();
    expect(readIsoRangeParam(paramsOf({ from: 'не дата', to: '2026-08-20' }), 'from', 'to', TODAY)).toBeNull();
    expect(readIsoRangeParam(paramsOf({ from: '2026-08-20', to: '2026-08-01' }), 'from', 'to', TODAY)).toBeNull();
  });

  it('будущий хвост обрезается «сегодня» (канон #477: будущего в скоупе не бывает)', () => {
    expect(readIsoRangeParam(paramsOf({ from: '2026-08-01', to: '2030-01-01' }), 'from', 'to', TODAY)).toEqual({
      from: '2026-08-01',
      to: TODAY,
    });
    // Целиком будущий период — после обрезки to<from — отбрасывается.
    expect(readIsoRangeParam(paramsOf({ from: '2030-01-01', to: '2030-02-01' }), 'from', 'to', TODAY)).toBeNull();
  });
});
