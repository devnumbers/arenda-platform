'use client';

import { useEffect, useLayoutEffect } from 'react';

/**
 * Лок прокрутки страницы на всё время монтирования поверхностного
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
 * гюттера (html держит scrollbar-gutter: stable — сдвига колонки нет).
 *
 * Позиция чтения сохраняется (карта #1437, план 041, feel-check
 * владельца 16.10): по оси Y лок ставит hidden, а не clip — hidden-бокс
 * остаётся скролл-контейнером и хранит оффсет (clip сбрасывал
 * viewport-скролл: страница «отходила назад» при каждом открытии и не
 * возвращалась после закрытия); по оси X остаётся clip —
 * скролл-контейнер по X не создаётся, sticky жив (канон #709).
 *
 * Страж позиции живёт на уровне модуля и вооружён до любого
 * монтирования: прыжок движка при вставке оверлея случается синхронно с
 * мутацией DOM, раньше всех эффектов React (замер 16.10: MutationObserver
 * видит документ уже сдвинутым; доверенный мгновенный скролл без
 * единого вызова scroll-API — не focus, не scrollIntoView, не anchoring,
 * не snap), поэтому страж, ставившийся эффектом хука, опаздывал в двух
 * местах сразу — захватывал уже прыгнутый оффсет и регистрировался после
 * события. Цель клампа — до-прыжковый трекер: скролл-события обычного
 * чтения приходят после layout-фазы, трекер хранит позицию чтения.
 * Коррекция уходит из диспетчера события макротаской: присвоение
 * scrollTop тому же скроллеру внутри его же scroll-события Chromium
 * молча отбрасывает (страж в диспетчере: ~110 срабатываний без
 * эффекта; отложенный — вернул позицию). setTimeout, а не rAF:
 * headless-браузер e2e не обязан тикать кадры.
 */

/** Последняя стабильная позиция чтения (до-прыжковая). */
let lastStableScrollY = 0;

/** Число активных локов: страж клампит, пока оно больше нуля. */
let lockCount = 0;

let pendingClamp = false;

if (typeof window !== 'undefined') {
  window.addEventListener(
    'scroll',
    () => {
      // Трекер заморожен на время лока: прыжок движка приходит тем же
      // scroll-событием, на котором клампится, и слушатели диспатчат по
      // порядку регистрации — незамороженный трекер успел бы запомнить
      // прыгнутое значение раньше клампа (замер 16.10: в протоколе одно
      // событие, кламп видел «смещение равно трекеру» и не срабатывал).
      if (lockCount === 0) {
        lastStableScrollY = window.scrollY;
      }
    },
    { passive: true },
  );
  window.addEventListener('scroll', () => {
    if (lockCount === 0 || pendingClamp) {
      return;
    }
    const scroller = document.scrollingElement;
    if (scroller === null || scroller.scrollTop === lastStableScrollY) {
      return;
    }
    const target = lastStableScrollY;
    pendingClamp = true;
    setTimeout(() => {
      pendingClamp = false;
      if (lockCount > 0 && scroller.scrollTop !== target) {
        scroller.scrollTop = target;
      }
    }, 0);
  });
}

/** Лок обязан применяться в layout-фазе (до отрисовки), чтобы фон не
 * успевал скроллиться ни на кадр; на сервере layout-эффекта нет —
 * изоморфная замена (прецедент useIsomorphicLayoutEffect). */
const useClientLayoutEffect = typeof window === 'undefined' ? useEffect : useLayoutEffect;

export function useBodyScrollLock(): void {
  useClientLayoutEffect(() => {
    const previousHtml = document.documentElement.style.overflow;
    const previousBody = document.body.style.overflow;
    // Лок несёт html — документный скроллёр: overflow-x: clip на корне
    // (#709) блокирует перенос overflow с body на viewport (перенос
    // работает, только если у html оба значения visible), и лок одного
    // body в Chromium вьюпорт не останавливал — жест уезжал в страницу
    // (карта #1416, тикет #1417). По Y — hidden, не clip: clip выкидывает
    // документ из скролла с потерей позиции (план 041); hidden —
    // программно скроллируемый контейнер с сохранённым оффсетом,
    // пользовательского скролла нет. body — прежний канон 13.10.
    document.documentElement.style.overflow = 'clip hidden';
    document.body.style.overflow = 'hidden';
    lockCount += 1;
    return () => {
      lockCount -= 1;
      document.documentElement.style.overflow = previousHtml;
      document.body.style.overflow = previousBody;
    };
  }, []);
}
