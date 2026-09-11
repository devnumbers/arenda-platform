import { describe, expect, it } from 'vitest';

import { flattenUniqueById } from './feed-pages';

describe('flattenUniqueById — склейка порций ленты с дедупом (#597)', () => {
  it('склеивает порции подряд, порядок сохраняется', () => {
    expect(
      flattenUniqueById([
        [{ id: 'a' }, { id: 'b' }],
        [{ id: 'c' }],
      ]),
    ).toStrictEqual([{ id: 'a' }, { id: 'b' }, { id: 'c' }]);
  });

  it('повторная строка из догрузки поглощается один раз', () => {
    const pages = [
      [{ id: 'a' }, { id: 'b' }],
      [{ id: 'b' }, { id: 'c' }, { id: 'c' }],
    ];
    expect(flattenUniqueById(pages)).toStrictEqual([{ id: 'a' }, { id: 'b' }, { id: 'c' }]);
  });

  it('пустые и кратные порции не ломают склейку', () => {
    expect(flattenUniqueById([])).toStrictEqual([]);
    expect(flattenUniqueById([[], [{ id: 'a' }], []])).toStrictEqual([{ id: 'a' }]);
  });
});
