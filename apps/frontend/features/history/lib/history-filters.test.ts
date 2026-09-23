import { describe, expect, it } from 'vitest';
import {
  DEFAULT_HISTORY_FILTERS,
  historyFeedScope,
  historyFiltersParams,
  historyPeriodChipLabel,
  isDefaultHistoryFilters,
  readHistoryFilters,
  toggleHistoryFilterOption,
} from './history-filters';

const TODAY = '2026-09-23';

const paramsOf = (record: Record<string, string>) => ({
  get: (name: string) => record[name] ?? null,
});

const UUID_A = '0b6e3333-aaaa-4bbb-8ccc-ddddddddddd1';
const UUID_B = '0b6e3333-aaaa-4bbb-8ccc-ddddddddddd2';
const NOT_UUID = 'собака';

describe('readHistoryFilters', () => {
  it('пустой URL — дефолт: без периода, все группы «все» (null)', () => {
    expect(readHistoryFilters(paramsOf({}), TODAY)).toEqual(DEFAULT_HISTORY_FILTERS);
    expect(DEFAULT_HISTORY_FILTERS).toEqual({
      period: null,
      actions: null,
      kinds: null,
      actorIds: null,
      propertyIds: null,
    });
  });

  it('период читается; битые даты, перевёрнутый и будущий хвост отбрасываются (канон #477)', () => {
    expect(readHistoryFilters(paramsOf({ from: '2026-08-01', to: '2026-08-20' }), TODAY).period).toEqual({
      from: '2026-08-01',
      to: '2026-08-20',
    });
    // Будущий хвост обрезается «сегодня».
    expect(readHistoryFilters(paramsOf({ from: '2026-08-01', to: '2030-01-01' }), TODAY).period).toEqual({
      from: '2026-08-01',
      to: TODAY,
    });
    // Перевёрнутый период — отброшен.
    expect(readHistoryFilters(paramsOf({ from: '2026-08-20', to: '2026-08-01' }), TODAY).period).toBeNull();
    // Битые даты — отброшены.
    expect(readHistoryFilters(paramsOf({ from: '2026-13-40', to: '2026-08-20' }), TODAY).period).toBeNull();
    expect(readHistoryFilters(paramsOf({ from: '2026-08-01' }), TODAY).period).toBeNull();
  });

  it('группа: параметр отсутствует — «все» (null); пустое значение — «ни один» ([]) ', () => {
    expect(readHistoryFilters(paramsOf({}), TODAY).actions).toBeNull();
    expect(readHistoryFilters(paramsOf({ actions: 'added,deleted' }), TODAY).actions).toEqual(['added', 'deleted']);
    expect(readHistoryFilters(paramsOf({ actions: '' }), TODAY).actions).toEqual([]);
    expect(readHistoryFilters(paramsOf({ kinds: 'payment' }), TODAY).kinds).toEqual(['payment']);
    expect(readHistoryFilters(paramsOf({ kinds: '' }), TODAY).kinds).toEqual([]);
  });

  it('неизвестные слаги действий/видов отбрасываются; известные сохраняют порядок', () => {
    expect(readHistoryFilters(paramsOf({ actions: 'added,чужое,deleted' }), TODAY).actions).toEqual(['added', 'deleted']);
    expect(readHistoryFilters(paramsOf({ kinds: 'payment,чужое' }), TODAY).kinds).toEqual(['payment']);
    expect(readHistoryFilters(paramsOf({ actions: 'чужое' }), TODAY).actions).toEqual([]);
  });

  it('id — только uuid без дублей, порядок сохранён; пустое значение — «ни один»', () => {
    expect(readHistoryFilters(paramsOf({ actors: `${UUID_B},${UUID_A},${UUID_B}` }), TODAY).actorIds).toEqual([
      UUID_B,
      UUID_A,
    ]);
    expect(readHistoryFilters(paramsOf({ actors: NOT_UUID }), TODAY).actorIds).toEqual([]);
    expect(readHistoryFilters(paramsOf({ objects: `${UUID_A}` }), TODAY).propertyIds).toEqual([UUID_A]);
    expect(readHistoryFilters(paramsOf({ objects: '' }), TODAY).propertyIds).toEqual([]);
  });
});

describe('historyFiltersParams', () => {
  it('дефолт — пустой patch (канон: дефолтные значения не пишутся)', () => {
    expect(historyFiltersParams(DEFAULT_HISTORY_FILTERS)).toEqual({});
  });

  it('«все» (null) — ключ отсутствует; «ни один» — пустое значение; выбор — CSV', () => {
    expect(
      historyFiltersParams({
        period: null,
        actions: null,
        kinds: ['payment', 'task'],
        actorIds: [],
        propertyIds: [UUID_A],
      }),
    ).toEqual({ kinds: 'payment,task', actors: '', objects: UUID_A });
  });

  it('период пишется обеими границами', () => {
    expect(
      historyFiltersParams({ ...DEFAULT_HISTORY_FILTERS, period: { from: '2026-08-01', to: '2026-08-20' } }),
    ).toEqual({ from: '2026-08-01', to: '2026-08-20' });
  });

  it('чтение после записи — то же состояние (round-trip)', () => {
    const filters = {
      period: { from: '2026-08-01', to: TODAY },
      actions: ['added'] as const,
      kinds: null,
      actorIds: [] as const,
      propertyIds: [UUID_A, UUID_B] as const,
    };
    const read = readHistoryFilters(paramsOf(historyFiltersParams(filters)), TODAY);
    expect(read).toEqual(filters);
  });
});

describe('isDefaultHistoryFilters', () => {
  it('дефолт — все группы «все» и без периода', () => {
    expect(isDefaultHistoryFilters(DEFAULT_HISTORY_FILTERS)).toBe(true);
    expect(isDefaultHistoryFilters({ ...DEFAULT_HISTORY_FILTERS, kinds: [] })).toBe(false);
    expect(isDefaultHistoryFilters({ ...DEFAULT_HISTORY_FILTERS, actions: ['added'] })).toBe(false);
    expect(isDefaultHistoryFilters({ ...DEFAULT_HISTORY_FILTERS, period: { from: TODAY, to: TODAY } })).toBe(false);
  });
});

describe('historyFeedScope', () => {
  it('дефолт — пустой скоуп', () => {
    expect(historyFeedScope(DEFAULT_HISTORY_FILTERS)).toEqual({});
  });

  it('период мапится в dateFrom/dateTo, группы — в скоуп контракта #708', () => {
    expect(
      historyFeedScope({
        period: { from: '2026-08-01', to: '2026-08-20' },
        actions: ['added', 'changed'],
        kinds: null,
        actorIds: [UUID_A],
        propertyIds: null,
      }),
    ).toEqual({
      dateFrom: '2026-08-01',
      dateTo: '2026-08-20',
      actions: ['added', 'changed'],
      actorIds: [UUID_A],
    });
  });

  it('группа «ни один» — результат пуст: скоуп null, запрос не делается', () => {
    expect(historyFeedScope({ ...DEFAULT_HISTORY_FILTERS, actorIds: [] })).toBeNull();
    expect(historyFeedScope({ ...DEFAULT_HISTORY_FILTERS, kinds: [] })).toBeNull();
    // «Все» (null) — не пустой результат.
    expect(historyFeedScope({ ...DEFAULT_HISTORY_FILTERS, actorIds: null })).not.toBeNull();
  });
});

describe('historyPeriodChipLabel', () => {
  it('без периода — «Выбрать период» (макет 2177-60527)', () => {
    expect(historyPeriodChipLabel(null)).toBe('Выбрать период');
  });

  it('с периодом — формат чипа канона: «1 окт — 20 дек» (макет 2067-162950)', () => {
    expect(historyPeriodChipLabel({ from: '2026-10-01', to: '2026-12-20' })).toBe('1 окт — 20 дек');
    expect(historyPeriodChipLabel({ from: '2026-09-23', to: '2026-09-23' })).toBe('23 сен');
  });
});

describe('toggleHistoryFilterOption', () => {
  const ALL = ['added', 'changed', 'completed', 'deleted'];

  it('«все» (null) — явный список «все, кроме переключённой»', () => {
    expect(toggleHistoryFilterOption(ALL, null, 'changed')).toEqual(['added', 'completed', 'deleted']);
  });

  it('список — добавить в конец и убрать', () => {
    expect(toggleHistoryFilterOption(ALL, ['added'], 'deleted')).toEqual(['added', 'deleted']);
    expect(toggleHistoryFilterOption(ALL, ['added', 'deleted'], 'deleted')).toEqual(['added']);
  });

  it('уборка последней опции даёт «ни один» ([]), не «все»', () => {
    expect(toggleHistoryFilterOption(ALL, ['changed'], 'changed')).toEqual([]);
  });
});
