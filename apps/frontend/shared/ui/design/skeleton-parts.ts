/** Чистая логика составных скелетон-примитивов (#604): тон блоков и
 * детерминированный цикл ширин строк. Вынесена из .tsx, чтобы покрываться
 * юнит-тестами — тестовое окружение фронта исполняет только чистую логику
 * (node, без DOM); сам рендер проверяют экранная приёмка и e2e. */

/** Тон блоков-заглушек: `base` — плейсхолдер на белой поверхности
 * (`bg-surface-muted`), `muted` — внутри серой карточки
 * (`bg-surface-muted-hover`), чтобы блок оставался видимым (канон
 * `Skeleton`). */
export type SkeletonTone = 'base' | 'muted';

const TONE_CLASSES = {
  base: 'bg-surface-muted',
  muted: 'bg-surface-muted-hover',
} as const;

export function skeletonBlockClass(tone: SkeletonTone): string {
  return TONE_CLASSES[tone];
}

/** Ширины полей-заглушек одной строки списка: заголовок и подзаголовок. */
export type SkeletonRowWidths = {
  readonly title: string;
  readonly subtitle: string;
};

/** Ширины первой строки цикла — дефолт одиночной строки-заглушки. */
export const SKELETON_ROW_WIDTHS_DEFAULT: SkeletonRowWidths = {
  title: 'w-2/5',
  subtitle: 'w-3/5',
};

/** Цикл ширин строк: подряд идущие строки различаются, чтобы список
 * заглушек не выглядел механическим повтором. Массив константный — никакого
 * randomness: SSR и клиент рендерят одинаковый DOM, гидратация не
 * расходится. */
const ROW_WIDTH_CYCLE: ReadonlyArray<SkeletonRowWidths> = [
  SKELETON_ROW_WIDTHS_DEFAULT,
  { title: 'w-1/2', subtitle: 'w-2/5' },
  { title: 'w-3/5', subtitle: 'w-1/2' },
];

/** Ширины для `count` строк списка: цикл повторяется с начала. Отрицательное
 * и ноль — пустой список. */
export function skeletonRowWidths(count: number): ReadonlyArray<SkeletonRowWidths> {
  const total = Math.max(0, count);
  const result: SkeletonRowWidths[] = [];
  for (let index = 0; index < total; index += 1) {
    const entry = ROW_WIDTH_CYCLE[index % ROW_WIDTH_CYCLE.length];
    if (entry !== undefined) {
      result.push(entry);
    }
  }
  return result;
}
