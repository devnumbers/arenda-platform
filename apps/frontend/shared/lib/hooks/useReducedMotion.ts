'use client';

import { useEffect, useState } from 'react';

const REDUCED_MOTION_MEDIA_QUERY = '(prefers-reduced-motion: reduce)';

/**
 * Реактивный флаг `prefers-reduced-motion`.
 *
 * На сервере и в первый клиентский рендер — `false` (в лад гидратации),
 * дальше настоящее значение медиазапроса с переподпиской на `change`.
 * Потребители движения укорачивают им JS-ожидания; CSS-сторона
 * укорачивается сама медиа-переопределениями токенов (tokens.css, §8:
 * анимацию не отключаем, а укорачиваем до ~150мс-порядка).
 */
export function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    const mql = window.matchMedia(REDUCED_MOTION_MEDIA_QUERY);
    const onChange = (): void => setReduced(mql.matches);
    onChange();
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, []);

  return reduced;
}
