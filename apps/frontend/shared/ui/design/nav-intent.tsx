'use client';

import { useState } from 'react';

/**
 * Полный prefetch навигационной ссылки по первому интенту (hover/
 * pointerdown/focus). Нужен для динамических маршрутов («Контакты»,
 * «Объекты»): авто-prefetch видимой ссылки качает только скелет-шелл до
 * ближайшей loading-границы, а чанк страницы ехал бы на клике; интент
 * поднимает prefetch до полного — переход остаётся мгновенным. Для
 * статических маршрутов безвреден: полный prefetch и так дефолт.
 */
export function useNavIntentLink(): {
  readonly prefetch: true | undefined;
  readonly onIntent: () => void;
} {
  const [fullPrefetch, setFullPrefetch] = useState(false);
  const onIntent = (): void => {
    setFullPrefetch(true);
  };
  return { prefetch: fullPrefetch ? true : undefined, onIntent };
}
