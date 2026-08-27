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
 */
export function useInfiniteScroll(
  onReachEnd: () => void,
  enabled = true,
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
  }, [enabled]);

  return sentinelRef;
}
