'use client';

import { useEffect, useRef, useState } from 'react';
import type { RefObject } from 'react';

/** Живая контентная ширина элемента (contentRect ResizeObserver'а);
 * null — до монтирования/первого замера: первый кадр считает по
 * телефонному фолбэку (см. shared/lib/icon-row-fit, прецедент —
 * calendar-date-picker). */
export function useElementWidth<T extends HTMLElement>(): [RefObject<T | null>, number | null] {
  const ref = useRef<T | null>(null);
  const [width, setWidth] = useState<number | null>(null);

  useEffect(() => {
    const node = ref.current;
    if (node === null || typeof ResizeObserver === 'undefined') {
      return;
    }
    const observer = new ResizeObserver((entries) => {
      setWidth(entries[0]?.contentRect.width ?? null);
    });
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  return [ref, width];
}
