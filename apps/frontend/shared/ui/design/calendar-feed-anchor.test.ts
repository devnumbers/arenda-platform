import { describe, expect, it } from 'vitest';
import { compensatedScrollTop } from './calendar-feed-anchor';

describe('calendar-feed-anchor — позиция после дорисовки месяцев назад', () => {
  it('картинка не телепортируется: позиция от фактического места, не от снимка на триггере', () => {
    // Триггер снял ленту на высоте 3000; пока React рисовал 6 месяцев,
    // инерция доехала до стены (scrollTop 0). Возврат к снимку (17 + Δ)
    // прыгнул бы назад на 17px — путь инерции за задержку. Верный кадр —
    // текущий: стена остаётся у кромки, приписка уезжает выше невидимо.
    expect(compensatedScrollTop({ scrollTop: 0 }, 3000, 5014)).toBe(2014);
    // Инерция в полёте — то же самое от фактического места.
    expect(compensatedScrollTop({ scrollTop: 5 }, 3000, 5014)).toBe(2019);
  });

  it('лента не выросла: позиция не меняется вовсе', () => {
    expect(compensatedScrollTop({ scrollTop: 640 }, 9000, 9000)).toBe(640);
  });
});
