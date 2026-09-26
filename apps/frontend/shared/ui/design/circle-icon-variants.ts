/** Вариантная карта канона «круг 44 с кольцом 2.5px» (тикет #846):
 * чистые данные вынесены из circle-icon.tsx, чтобы покрываться юнит-тестами
 * — тестовое окружение фронта исполняет только чистую логику (node, без
 * DOM); сам рендер проверяют экранная приёмка и e2e (прецедент
 * skeleton-parts). */

/** Подложка, на которой лежит круг: `muted` — серая (--dl-surface-muted),
 * `white` — белая (--dl-surface). Кант красится цветом подложки, круг —
 * инверсией относительно неё (макет 1527:74139 — белый круг на серой
 * карточке, 1527:74837 — серый круг на белом фоне; карта-референс
 * поверхностей — PropertyAvatar card/row). */
export type CircleIconVariant = 'muted' | 'white';

/** Кант 2.5px цветом подложки — для кругов с фоном вне пары surface
 * (подложки каталога категорий, служебные точки-бейджи, primary/10). */
export const circleIconRing: Record<CircleIconVariant, string> = {
  muted: 'shadow-[0_0_0_2.5px_var(--dl-surface-muted)]',
  white: 'shadow-[0_0_0_2.5px_var(--dl-surface)]',
} as const;

/** Пара «фон + кант» канонного круга: на серой подложке — белый круг,
 * на белой — серый. */
export const circleIconPair: Record<CircleIconVariant, string> = {
  muted: `bg-surface ${circleIconRing.muted}`,
  white: `bg-surface-muted ${circleIconRing.white}`,
} as const;
