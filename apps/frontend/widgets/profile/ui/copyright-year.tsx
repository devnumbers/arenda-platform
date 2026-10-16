'use client';

import { useSyncExternalStore, type JSX } from 'react';

// Год в подписи не порождает событий сам по себе — подписка не нужна,
// снапшот перечитывается React'ом на гидратации и после монтирования.
const subscribe = (): (() => void) => () => {};

/** Год в подписи «© Рентли» — живой, по часам устройства просматривающего:
 * экран правовой информации показывает актуальный год после 1 января без
 * деплоя. До гидратации показывается переданный с сервера год сборки, поэтому
 * hydrate-проход посимвольно совпадает с SSR-HTML; после монтирования
 * useSyncExternalStore сверяет снапшот клиентских часов и перерисовывает
 * текст актуальным годом — независимо от политики патча текста при
 * гидратации. */
export function CopyrightYear({
  initialYear,
}: {
  readonly initialYear: number;
}): JSX.Element {
  const hydrated = useSyncExternalStore(
    subscribe,
    () => true,
    () => false,
  );

  return <span>{hydrated ? new Date().getFullYear() : initialYear}</span>;
}
