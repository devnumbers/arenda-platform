'use client';

import { useEffect } from 'react';

/** Лок прокрутки страницы на всё время монтирования поверхностного
 * диалога. Полноэкранные пикеры (CalendarDatePicker/CalendarRangePicker)
 * — fixed-поверхности без модальной обёртки: страница под ними остаётся
 * скроллируемой, и жест у ограниченного края ленты уезжал в цепочку
 * скролла страницы (решение владельца 2026-10-13; прецедент — ручной
 * overflow:hidden в more-sheet). Восстановление — по очистке эффекта.
 * Вложенные локи не поддерживаются: полноэкранные поверхности не
 * вкладываются друг в друга; на Windows появление/исчезновение скроллбара
 * сдвигает фоновую страницу на ширину гюттера (html держит
 * scrollbar-gutter: stable — сдвига колонки нет). */
export function useBodyScrollLock(): void {
  useEffect(() => {
    const previous = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => {
      document.body.style.overflow = previous;
    };
  }, []);
}
