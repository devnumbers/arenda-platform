'use client';

import { useEffect, useState } from 'react';

/**
 * Расчёт клавиатурного инсета из пары высот (#1151, research #1148 §B):
 * насколько низ visual viewport (то, что видит пользователь) выше низа
 * layout viewport, к которому приколочена fixed-панель. iOS Safari
 * клавиатурой уменьшает только visual viewport (группа «resizes visual
 * only», Chrome «viewport resize behavior») — инсет возникает именно там;
 * на Android с interactive-widget=resizes-content layout уже ужат метой
 * и инсет остаётся 0. Дробные CSS-пиксели округляются (translateY без
 * субпикселей), отрицательная разница (пинч-зум, оверскролл) клампится —
 * панель сдвигается только вверх.
 */
export function visualKeyboardInset(
  layoutClientHeight: number,
  visualBottom: number,
): number {
  return Math.max(0, Math.round(layoutClientHeight - visualBottom));
}

const DESKTOP_MEDIA_QUERY = '(min-width: 1024px)';

/**
 * Реактивный зазор над клавиатурой в CSS-пикселях; 0 — клавиатура закрыта
 * (сервер и первый клиентский рендер — тоже 0, в лад гидратации, как
 * useStandalone/useReducedMotion). Точка применения — StickyBottomBar:
 * translateY(-инсет) ставит fixed-панель над клавиатурой там, где мета
 * interactive-widget не работает — iOS Safari мету игнорирует (на Android
 * мет сжимает layout сам, инсет 0, хук бездействует).
 *
 * Слушатели не работают на ПК (≥1024): клавиатура ПК-браузера fixed-панель
 * не перекрывает (#1151 п.4 — ПК не трогаем). Ярус переоценивается
 * подпиской на change медиазапроса — поворот планшета через границу 1024
 * переводит хук между режимами без ремоунта. Порог — ярус desktop:
 * (--breakpoint-desktop в globals.css); CSS-переменную медиазапрос не
 * читает, число дублируется — канон комментария в globals.css.
 */
export function useVisualKeyboardInset(): number {
  const [inset, setInset] = useState(0);

  useEffect(() => {
    const mq = window.matchMedia(DESKTOP_MEDIA_QUERY);
    const vv = window.visualViewport;
    if (vv === null) {
      return undefined;
    }
    let raf = 0;
    const update = (): void => {
      raf = 0;
      // Высота layout viewport — documentElement.clientHeight: мобильный
      // WebKit исторически расходится с window.innerHeight при зуме и
      // клавиатуре (MDN innerHeight, usage note).
      const visualBottom = vv.offsetTop + vv.height;
      setInset(
        visualKeyboardInset(document.documentElement.clientHeight, visualBottom),
      );
    };
    // resize и scroll идут парой: при открытой клавиатуре visual viewport
    // ещё и панамируется — клавиатурный scroll приходит без resize.
    // События приходят пачками и во время анимации клавиатуры — батчим
    // через rAF (Chrome «The VisualViewport API»).
    const schedule = (): void => {
      if (raf === 0) {
        raf = requestAnimationFrame(update);
      }
    };
    const attach = (): void => {
      vv.addEventListener('resize', schedule, { passive: true });
      vv.addEventListener('scroll', schedule, { passive: true });
      update();
    };
    const detach = (): void => {
      if (raf !== 0) {
        cancelAnimationFrame(raf);
        raf = 0;
      }
      vv.removeEventListener('resize', schedule);
      vv.removeEventListener('scroll', schedule);
      setInset(0);
    };
    // Поворот планшета через границу яруса: на ПК слушатели сняты и инсет
    // погашен, на мобайле — навешаны и пересчитаны.
    const syncTier = (): void => {
      if (mq.matches) {
        detach();
      } else {
        attach();
      }
    };
    syncTier();
    mq.addEventListener('change', syncTier);
    return () => {
      mq.removeEventListener('change', syncTier);
      detach();
    };
  }, []);

  return inset;
}
