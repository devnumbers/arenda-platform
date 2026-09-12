'use client';

import { useEffect, useRef, useState, type RefCallback } from 'react';

/**
 * Дозагрузка списка «по 50 + бесконечный скролл» (правило платформы,
 * резолюция #452): sentinel-элемент внизу списка, наблюдаемый
 * IntersectionObserver. Когда он входит в вьюпорт (с запасом 300px, чтобы
 * порция подгружалась до докрутки до края), вызывается `onReachEnd` —
 * экран решает сам, догружать ли следующую порцию или нарастить окно
 * проекции. Обсервер переживает смену колбэка; правило действует только
 * пока `enabled` (порций больше нет — наблюдение снято).
 *
 * Sentinel отдаётся callback-ref'ом: наблюдение перевешивается на сам
 * факт монтирования ноды, а не на момент, когда `enabled` стал true —
 * при тёплом кэше react-query обе вещи случаются в одном коммите, и
 * эффект, привязанный только к enabled, ноду мог не увидеть (#631).
 *
 * `resetKey` пересоздаёт обсервер при смене значения (типично — флаг
 * «порция легла»): IntersectionObserver сообщает состояние только при
 * пересечении порога и при первичном наблюдении, поэтому sentinel,
 * оставшийся во вьюпорте после догрузки, без пересоздания больше не
 * выстрелит — лента замрёт. Пересоздание даёт свежий первичный отчёт:
 * пока sentinel видим, дозагрузка продолжается.
 */
export function useInfiniteScroll(
  onReachEnd: () => void,
  enabled = true,
  resetKey?: unknown,
): RefCallback<HTMLDivElement> {
  const [sentinel, setSentinel] = useState<HTMLDivElement | null>(null);
  const handlerRef = useRef(onReachEnd);

  useEffect(() => {
    handlerRef.current = onReachEnd;
  }, [onReachEnd]);

  useEffect(() => {
    if (!enabled || sentinel === null) {
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          handlerRef.current();
        }
      },
      { rootMargin: '300px' },
    );
    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [enabled, sentinel, resetKey]);

  return setSentinel;
}
