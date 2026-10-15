'use client';

import { useEffect } from 'react';

/** Лок прокрутки страницы на всё время монтирования поверхностного
 * диалога. Полноэкранные поверхности (календарные пикеры, шит фильтров
 * истории, пикеры визардов, поиск адреса) — fixed-оверлеи без модальной
 * обёртки: страница под ними остаётся скроллируемой, и жест у края
 * внутренней ленты уезжает в цепочку скролла страницы; под непрозрачной
 * поверхностью движение невидимо и «уводит» ленту к закрытию (решение
 * владельца 2026-10-13; прецедент — ручной overflow:hidden в more-sheet;
 * конвенция слоёв — карта #1416, тикет #1417). Восстановление — по
 * очистке эффекта. Локи вкладываются LIFO (шит фильтров открывает пикер
 * периода поверх себя): React очищает эффекты детей раньше родительских,
 * restore по захваченному previous корректен. С локом Radix-шитов
 * (react-remove-scroll) хук не конфликтует — стек «календарный пикер
 * (этот хук) → шит месяца (Radix)» уже живёт. На Windows
 * появление/исчезновение скроллбара сдвигает фоновую страницу на ширину
 * гюттера (html держит scrollbar-gutter: stable — сдвига колонки нет). */
export function useBodyScrollLock(): void {
  useEffect(() => {
    const previousHtml = document.documentElement.style.overflow;
    const previousBody = document.body.style.overflow;
    // Лок несёт html — документный скроллёр: overflow-x: clip на корне
    // (#709) блокирует перенос overflow с body на viewport (перенос
    // работает, только если у html оба значения visible), и лок одного
    // body в Chromium вьюпорт не останавливал — жест уезжал в страницу
    // (карта #1416, тикет #1417). clip, а не hidden: scroll-контейнера
    // не создаёт, sticky фона жив (канон #709); на viewport clip
    // интерпретируется как hidden. body — прежний канон 13.10.
    document.documentElement.style.overflow = 'clip';
    document.body.style.overflow = 'hidden';
    return () => {
      document.documentElement.style.overflow = previousHtml;
      document.body.style.overflow = previousBody;
    };
  }, []);
}
