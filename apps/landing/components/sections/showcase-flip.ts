// Геометрия showcase-ленты (карта #1459…/#1507). Модель «постоянный
// шаг» (решение владельца 17.10 — отказ от слотовой компенсации, которая
// перераспределяла место между слотами и читалась как дёргание):
//
//   - у всех карточек один базовый слот (= боковые размеры кадров
//     2993-77855 / 3005-78910 / 3009-79717), зазор постоянный;
//   - позиция покоя: s_g = g·шаг + слот/2 − экран/2, шаг = слот + зазор;
//   - единственный размерный эффект — масштаб слота от --k, целиком
//     в CSS (showcase.module.css): фото-слот scale(1 + 0.25·k) —
//     центральная карточка = база × 1.25, ровно центральные размеры
//     кадров; масштаб равномерный, поэтому кроп object-fit: cover не
//     меняется никогда, а кадр движения не содержит лэйаута вовсе;
//   - плашки масштабируются тем же --k с своим отношением
//     (×1.1594 по X, ×1.25 по Y — их центральный размер не кратен
//     боковому равномерно).
//
// Соседние карточки при смене едут строго линейно с дорожкой —
// перераспределений нет по построению. JS ведёт только позицию p и
// записи --k; геометрия покоя здесь нужна движку для позиций снапа,
// тихой пересадки между копиями и центровки плашечного ряда.

export type FlipTier = "desk" | "tab" | "mob";

// Метрики яруса — зеркало размерных CSS-переменных showcase.module.css:
// cardSide/gap — слот и зазор фотоленты, plateSide/plateGap — слот и
// зазор плашечного ряда. Меняются только синхронно с CSS.
export type FlipMetrics = {
  cardSide: number; // ширина базового слота фотокарточки
  gap: number; // зазор слотов фотоленты (не морфится)
  plateSide: number; // ширина базового слота плашки
  plateGap: number; // зазор слотов плашечного ряда (не морфится)
};

export const FLIP_METRICS: Record<FlipTier, FlipMetrics> = {
  desk: { cardSide: 800, gap: 12, plateSide: 360, plateGap: 16 },
  tab: { cardSide: 400, gap: 12, plateSide: 345, plateGap: 16 },
  mob: { cardSide: 240, gap: 12, plateSide: 207, plateGap: 8 },
};

export type FlipModel = {
  tier: FlipTier;
  m: FlipMetrics;
  step: number; // шаг позиций покоя фотодорожки
  copyPlain: number; // сдвиг между копиями набора
  snapArr: number[]; // позиции покоя всех узлов ленты
  zoneW: number; // ширина колонки карусели (центровка плашек)
};

// Модель покоя: слоты одинаковой ширины, позиции — арифметика шага.
export function buildFlipModel(
  tier: FlipTier,
  viewportW: number,
  zoneW: number,
  cardsPerCopy: number,
  nodeCount: number,
): FlipModel {
  const m = FLIP_METRICS[tier];
  const step = m.cardSide + m.gap;
  const copyPlain = cardsPerCopy * step;
  const snap = (g: number) =>
    Math.floor(g / cardsPerCopy) * copyPlain +
    (g % cardsPerCopy) * step +
    m.cardSide / 2 -
    viewportW / 2;
  const snapArr = Array.from({ length: nodeCount }, (_, g) => snap(g));
  return { tier, m, step, copyPlain, snapArr, zoneW };
}

// Транслейт фотодорожки: позиция p в координатах модели покоя.
export function photoTrackTranslate(p: number): string {
  return `translate3d(${(-p).toFixed(2)}px,0,0)`;
}

// Транслейт плашечного ряда — линейно по p между позициями покоя
// (frac линеен по p): ряд центруется по той же позиции, что и фотолента.
// Формула через накопление текущих ширин не годится: при смене активного
// индекса она даёт скачок — найдено замером (44.7px на прибытии).
export function plateStripTranslate(model: FlipModel, p: number): number {
  const { m, snapArr, zoneW } = model;
  const g = snapArr.length;
  const pitch = m.plateSide + m.plateGap;
  let gLo = 0;
  while (gLo < g - 1 && (snapArr[gLo + 1] ?? 0) <= p) gLo += 1;
  const next = snapArr[Math.min(gLo + 1, g - 1)] ?? 0;
  const span = Math.max(next - (snapArr[gLo] ?? 0), 1);
  const frac = Math.min(Math.max((p - (snapArr[gLo] ?? 0)) / span, 0), 1);
  return zoneW / 2 - (gLo * pitch + m.plateSide / 2) - frac * pitch;
}
