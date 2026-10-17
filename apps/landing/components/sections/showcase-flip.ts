// FLIP-математика showcase-морфа (решение #1501, рецепт research #1504,
// внедрение #1507). В покое размеры держит CSS-система calc(--k)
// (showcase.module.css); в движении (класс .morphing) геометрия
// зафиксирована в центральной раскладке, а разница состояний уезжает
// в translate3d + scale(sx, sy) каждого узла — кадр движения не трогает
// лэйаут (только записи трансформов: дешёвый recalc стилей остаётся,
// Layout в кадре нулевой — замер до/после в тикете), композитор двигает
// готовые слои.
//
// Ключ паритета: морф линейно выражается масштабом центральной раскладки.
// Боковые метрики по кадрам 2993-77855 (ПК), 3005-78910 (планшет),
// 3009-79717 (мобилка): на таб/моб боковая карточка — ширина центральной
// при высоте ×0.8, на ПК — ×0.8 по обеим осям (на широких ПК кламп
// растит боковую ширину до ×0.96). Компенсация шага слотов (все величины
// в px модели покоя, коэффициенты из snapModel):
//
//   comp_g = D·(prefixK_g + k_g/2 − g − 1/2),   D = W_центр − W_бок
//   sx_g = (W_бок + D·k_g)/W_центр,  sy_g — аналогично по высотам
//
// где prefixK_g = Σ k_i по узлам левее g. Сдвиг ряда (фотодорожка −p,
// плашки — translate из plateStripTranslate) прикладывается уровнем
// ряда в обеих системах и в компенсацию не входит. Тождество проверено
// на покое (k ∈ {0,1}) и в полёте (любой k-вектор): фикс-раскладка с
// компенсациями даёт те же боксы, что сегодняшняя k-раскладка.

export type FlipTier = "desk" | "tab" | "mob";

// Метрики яруса — зеркало размерных CSS-переменных showcase.module.css
// (мобайл — база, планшет/ПК — медиа-оверрайды; plateGap — рядовый зазор
// --plate-gap-y, не внутренний зазор плашки --plate-gap). Как и прежний
// snapModel, ПК-боковая зеркалит clamp от clientWidth — на классических
// скроллбарах CSS-зеркало от 100vw уходит на ширину скроллбара
// (пре-существующий перекос позиций, теперь и масштабов в полёте).
// Меняются только синхронно с CSS.
export type FlipMetrics = {
  cardSide: number; // ширина боковой фотокарточки
  card: number; // ширина центральной
  gap: number; // шаг фотодорожки (не морфится)
  cardSideH: number; // высота боковой
  cardH: number; // высота центральной (= высоте окна ленты)
  glassSideW: number; // ширина стеклянной карточки, боковая
  glassW: number; // …центральная
  plateSide: number; // ширина боковой плашки
  plate: number; // …центральной
  plateSideH: number; // высота боковой плашки
  plateH: number; // высота центральной плашки (= высоте ряда)
  plateGap: number; // шаг плашечного ряда (не морфится)
};

export const FLIP_METRICS: Record<FlipTier, FlipMetrics> = {
  desk: {
    cardSide: 800, card: 1000, gap: 20, cardSideH: 320, cardH: 400,
    glassSideW: 280, glassW: 350,
    plateSide: 360, plate: 450, plateSideH: 80, plateH: 100, plateGap: 16,
  },
  tab: {
    cardSide: 500, card: 500, gap: 20, cardSideH: 320, cardH: 400,
    glassSideW: 280, glassW: 350,
    plateSide: 345, plate: 400, plateSideH: 80, plateH: 100, plateGap: 16,
  },
  mob: {
    cardSide: 300, card: 300, gap: 12, cardSideH: 192, cardH: 240,
    glassSideW: 168, glassW: 210,
    plateSide: 207, plate: 240, plateSideH: 48, plateH: 60, plateGap: 8,
  },
};

// Локальный clamp: модуль самодостаточен (без импортов из showcase.tsx).
function clampFlip(v: number, lo: number, hi: number) {
  return Math.min(hi, Math.max(lo, v));
}

export type FlipModel = {
  tier: FlipTier;
  m: FlipMetrics;
  cardSide: number; // ширина боковой с учётом роста на широких ПК
  step: number; // шаг позиций покоя фотодорожки
  copyPlain: number; // сдвиг между копиями набора
  snapArr: number[]; // позиции покоя всех узлов ленты
  zoneW: number; // ширина колонки карусели (центровка плашек)
};

// Модель покоя — бывшая snapModel showcase.tsx плюс ширина зоны.
// На ПК боковая растёт с экраном — зеркало --side-w в CSS
// (clamp(800px, 100vw − 1640px, 960px)): внешний край боковой карточки
// всегда уходит за границу экрана.
export function buildFlipModel(
  tier: FlipTier,
  viewportW: number,
  zoneW: number,
  cardsPerCopy: number,
  nodeCount: number,
): FlipModel {
  const m = FLIP_METRICS[tier];
  const cardSide =
    tier === "desk" ? clampFlip(viewportW - 1640, m.cardSide, 960) : m.cardSide;
  const step = cardSide + m.gap;
  const copyPlain = cardsPerCopy * step;
  const snap = (g: number) =>
    Math.floor(g / cardsPerCopy) * copyPlain +
    (g % cardsPerCopy) * step +
    m.card / 2 -
    viewportW / 2;
  const snapArr = Array.from({ length: nodeCount }, (_, g) => snap(g));
  return { tier, m, cardSide, step, copyPlain, snapArr, zoneW };
}

const fmtScale = (sx: number, sy: number) =>
  `scale(${sx.toFixed(5)},${sy.toFixed(5)})`;
const fmtFlip = (comp: number, sx: number, sy: number) =>
  `translate3d(${comp.toFixed(3)}px,0,0) ${fmtScale(sx, sy)}`;

// Компенсация узла g: раскладка ряда зафиксирована на центральных
// ширинах, comp возвращает бокс в точности k-раскладки. D — дельта ширин
// ряда; сдвиг самого ряда (фотодорожка −p, плашки — translate модели)
// прикладывается уровнем ряда и сюда не входит.
function nodeCompensation(dW: number, k: readonly number[], g: number): number {
  let prefix = 0;
  for (let i = 0; i < g; i += 1) prefix += k[i] ?? 0;
  return dW * (prefix + (k[g] ?? 0) / 2 - g - 1 / 2);
}

// Транслейт фотодорожки — прежний (позиция p в координатах модели покоя).
export function photoTrackTranslate(p: number): string {
  return `translate3d(${(-p).toFixed(2)}px,0,0)`;
}

// Транслейты узлов фотодорожки при морфинге (класс .morphing стоит).
export function photoNodeTransforms(
  model: FlipModel,
  k: readonly number[],
): string[] {
  const { m, cardSide } = model;
  const dW = m.card - cardSide;
  const dH = m.cardH - m.cardSideH;
  return k.map((kg, g) =>
    fmtFlip(
      nodeCompensation(dW, k, g),
      (cardSide + dW * kg) / m.card,
      (m.cardSideH + dH * kg) / m.cardH,
    ),
  );
}

// Фото внутри слайда: в покое кадрирование делает CSS object-fit: cover
// по фактической k-коробке; в движении коробка заморожена в центральной,
// и кадрирование воспроизводится собственным масштабом фото — покадрово
// ровно cover интерполированного бокса (W(k)×H(k), те же лерпы, что у
// calc(--k) в покое):
//
//   σ(k) = max(W(k)/w₀, H(k)/h₀) — cover-масштаб текущего бокса
//   σ₀   = max(W_c/w₀, H_c/h₀)   — cover-масштаб замороженной коробки
//   a = σ(k)/(σ₀·sx),  b = σ(k)/(σ₀·sy)
//
// На равных долях (ПК-база) a=b=1 — трансформа нет, композиция
// инвариантна к равномерному масштабу. На таб/моб (ширина боковой равна
// центральной, высота ×0.8) и кламповых ПК состав кропа между
// состояниями действительно разный — здесь трансформа тянет кадр за
// анизотропным масштабом слайда: кроп меняется при прокрутке непрерывно
// и центрированно (то, что раньше делал cover каждый кадр), пропорции
// самого фото не искажаются (видимый рендер всегда σ(k)·(w₀, h₀)), и
// щелчка на посадке нет: в k=0 и k=1 значения совпадают с CSS-cover.
export type PhotoDims = { w: number; h: number };

export function photoImageTransforms(
  model: FlipModel,
  k: readonly number[],
  dims: ReadonlyArray<PhotoDims>,
): Array<string | null> {
  const { m } = model;
  const dW = m.card - model.cardSide;
  return k.map((kg, g) => {
    const dim = dims[g % dims.length];
    if (!dim) return null;
    const wk = model.cardSide + dW * kg;
    const hk = m.cardSideH + (m.cardH - m.cardSideH) * kg;
    const sx = wk / m.card;
    const sy = hk / m.cardH;
    const sigma0 = Math.max(m.card / dim.w, m.cardH / dim.h);
    const sigmak = Math.max(wk / dim.w, hk / dim.h);
    const a = sigmak / (sigma0 * sx);
    const b = sigmak / (sigma0 * sy);
    if (Math.abs(a - 1) < 1e-6 && Math.abs(b - 1) < 1e-6) return null;
    return `scale(${a.toFixed(5)},${b.toFixed(5)})`;
  });
}

// Стеклянная карточка внутри слайда: трансформ не нужен, когда
// стеклянная и слайдовая доли боковой ширины совпадают (ПК-база:
// стекло ×0.8 = слайд ×0.8 — стекло тянет родитель). На таб/моб боковой
// слайд той же ширины, что центральный (доля слайда 1) при стекле ×0.8,
// на кламповых ПК кламп растит слайд при стекле ×0.8 — там стекло
// масштабируется своим scale по X (высоту содержимого тянет sy
// родителя, текст остаётся равномерным).
export function photoGlassTransforms(
  model: FlipModel,
  k: readonly number[],
): Array<string | null> {
  const { m } = model;
  const glassFrac = m.glassSideW / m.glassW;
  const slideFrac = model.cardSide / m.card;
  if (Math.abs(glassFrac - slideFrac) < 1e-9) {
    return k.map(() => null);
  }
  return k.map((kg) => {
    const glassFracLerp = glassFrac + (1 - glassFrac) * kg;
    const slideSx = slideFrac + (1 - slideFrac) * kg;
    return fmtScale(glassFracLerp / slideSx, 1);
  });
}

// Транслейт плашечного ряда — прежняя модель: ряд центрируется по той же p,
// транслейт линейно по p между позициями покоя (frac линеен по p). Формула
// через накопление текущих ширин не годится: при смене активного индекса
// она даёт скачок (centerW−sideW)/2 — найдено замером (44.7px на прибытии).
export function plateStripTranslate(model: FlipModel, p: number): number {
  const { m, snapArr, zoneW } = model;
  const g = snapArr.length;
  const pitch = m.plateSide + m.plateGap;
  let gLo = 0;
  while (gLo < g - 1 && (snapArr[gLo + 1] ?? 0) <= p) gLo += 1;
  const next = snapArr[Math.min(gLo + 1, g - 1)] ?? 0;
  const span = Math.max(next - (snapArr[gLo] ?? 0), 1);
  const frac = clampFlip((p - (snapArr[gLo] ?? 0)) / span, 0, 1);
  return zoneW / 2 - (gLo * pitch + m.plate / 2) - frac * pitch;
}

// Транслейты узлов плашечного ряда при морфинге; возврат — трансформы
// плашек (сам ряд двигает plateStripTranslate, как и прежде).
export function plateNodeTransforms(
  model: FlipModel,
  k: readonly number[],
): string[] {
  const { m } = model;
  const dW = m.plate - m.plateSide;
  const dH = m.plateH - m.plateSideH;
  return k.map((kg, g) =>
    fmtFlip(
      nodeCompensation(dW, k, g),
      (m.plateSide + dW * kg) / m.plate,
      (m.plateSideH + dH * kg) / m.plateH,
    ),
  );
}
