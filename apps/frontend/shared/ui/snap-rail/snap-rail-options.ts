import type { EmblaOptionsType } from 'embla-carousel';

/**
 * Конфигурация движка горизонтальных snap-лент (Embla v9, тикет #1349,
 * решение владельца 13.10 — переход с кастомного pointer-прокси на Embla).
 * Механика и эталоны Apple — DESIGN.md §8 «Drag горизонтальных snap-лент».
 */
export function snapRailOptions({
  reducedMotion,
}: {
  readonly reducedMotion: boolean;
}): EmblaOptionsType {
  return {
    // Резинка у краёв: loop прокрутки по кругу запрещает ScrollBounds-сопротивление.
    loop: false,
    // Карточка снапится левым краем к левому краю контейнера: контейнер
    // сидит внутри bleed-паддинга вьюпорта потребителя, поэтому «край
    // контейнера» = визуальная линия 24 — отдельный align-офсет не нужен.
    align: 'start',
    // «snap» — семантика scrollViewWillEndDragging (WWDC18 803): бросок
    // даёт естественную точку остановки, доезд — к ближайшей snap-точке.
    // reduced-motion — без инерционного доезда: короткий докат к снапу.
    dragFree: reducedMotion ? false : 'snap',
  };
}
