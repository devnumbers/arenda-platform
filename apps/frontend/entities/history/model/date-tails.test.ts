import { describe, expect, it } from 'vitest';

import { stripBareDateTails } from './date-tails';
import type { HistorySegment } from './types';

const text = (t: string): HistorySegment => ({ text: t });
const link = (t: string): HistorySegment => ({ text: t, link: { kind: 'task', id: 'x' } });

describe('stripBareDateTails (аудит #876: убрать голые подписи дат из строк)', () => {
  it('срезает хвост «(срок дата)» отдельным сегментом', () => {
    expect(stripBareDateTails([text('Задача выполнена: '), link('Помыть посуду'), text(' (срок 21.09.2026)')])).toEqual([
      text('Задача выполнена: '),
      link('Помыть посуду'),
    ]);
  });

  it('срезает хвост-период аренды «(дата – дата)»', () => {
    expect(stripBareDateTails([text('Добавлена аренда: '), link('Анна Смирнова'), text(' (01.10.2026 – 30.09.2027)')])).toEqual([
      text('Добавлена аренда: '),
      link('Анна Смирнова'),
    ]);
  });

  it('срезает одиночную дату бессрочной аренды «(дата)»', () => {
    expect(stripBareDateTails([text('Добавлена аренда: '), link('Анна'), text(' (01.10.2026)')])).toEqual([
      text('Добавлена аренда: '),
      link('Анна'),
    ]);
  });

  it('оставляет словесные даты и «(Вы)» — убираются только голые подписи', () => {
    const segments = [
      text('Досрочное удаление аренды со стороны «Моя квартира» '),
      text('(выселение с 02.10.2025)'),
      text('Иван Иванов (Вы)'),
    ];
    expect(stripBareDateTails(segments)).toEqual(segments);
  });

  it('не трогает сегменты со ссылкой и середину ленты', () => {
    expect(stripBareDateTails([text(' (срок 21.09.2026) '), link('срок 21.09.2026')])).toEqual([
      text(' (срок 21.09.2026) '),
      link('срок 21.09.2026'),
    ]);
  });

  it('срезает подряд идущие хвостовые даты, но не заходит дальше них', () => {
    expect(stripBareDateTails([text('Платёж изменён: '), link('Аренда'), text(' (01.10.2026 – 30.09.2027)'), text(' (срок 02.10.2026)')])).toEqual([
      text('Платёж изменён: '),
      link('Аренда'),
    ]);
  });
});
