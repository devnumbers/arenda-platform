import { describe, expect, it } from 'vitest';

import { operationsSectionTitle } from './operations-section';

describe('operationsSectionTitle', () => {
  it('заголовок секции с текущим месяцем в предложном падеже (макет 1185:40820)', () => {
    expect(operationsSectionTitle('2026-09-11')).toBe('Операции в сентябре');
    expect(operationsSectionTitle('2026-08-31')).toBe('Операции в августе');
    expect(operationsSectionTitle('2026-03-01')).toBe('Операции в марте');
  });

  it('все двенадцать месяцев', () => {
    const expected = [
      'январе', 'феврале', 'марте', 'апреле', 'мае', 'июне',
      'июле', 'августе', 'сентябре', 'октябре', 'ноябре', 'декабре',
    ];
    expected.forEach((month, index) => {
      const iso = `2026-${String(index + 1).padStart(2, '0')}-15`;
      expect(operationsSectionTitle(iso)).toBe(`Операции в ${month}`);
    });
  });
});
