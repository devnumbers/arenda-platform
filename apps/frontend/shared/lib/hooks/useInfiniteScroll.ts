'use client';

import { useEffect, useRef, type RefObject } from 'react';

/**
 * Дозагрузка списка «по 50 + бесконечный скролл» (правило платформы,
 * резолюция #452): sentinel-элемент внизу списка, наблюдаемый
 * IntersectionObserver. Когда он входит в вьюпорт (с запасом 300px, чтобы
 * порция подгружалась до докрутки до края), вызывается `onReachEnd` —
 * экран решает сам, догружать ли следующую порцию или нарастить окно
 * проекции. Обсервер ставится один раз и переживает смену колбэка; правило
 * действует только пока `enabled` (порций больше нет — наблюдение снято).
 *
 * `resetKey` пересоздаёт обсервер при смене значения (типично — флаг
 * «порция легла»): IntersectionObserver сообщает состояние только при
 * пересечении порога и при первичном наблюдении, поэтому sentinel,
 * оставшийся во вьюпорте после догрузки, без пересоздания больше не
 * выстрелит — лента замрёт при тёплом кэше (баг #631). Пересоздание даёт
 * свежий первичный отчёт: пока sentinel видим, дозагрузка продолжается.
 */
export function useInfiniteScroll(
  onReachEnd: () => void,
  enabled = true,
  resetKey?: unknown,
): RefObject<HTMLDivElement | null> {
  const sentinelRef = useRef<HTMLDivElement>(null);
  const handlerRef = useRef(onReachEnd);

  useEffect(() => {
    handlerRef.current = onReachEnd;
  }, [onReachEnd]);

  useEffect(() => {
    const sentinel = sentinelRef.current;
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
  }, [enabled, resetKey]);

  return sentinelRef;
}
