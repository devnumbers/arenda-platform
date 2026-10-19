import type { JSX } from 'react';

/** Радиус дуги прогресса: круг слота 96 (viewBox 0 0 96 96), обод 4px —
 * r=46, чтобы дуга не резалась краем viewBox. */
const RING_RADIUS = 46;

/** Длина окружности дуги — экспорт для теста: ожидания считаются из
 * геометрии (2πr), не из реализации кольца. */
export const PHOTO_RING_LENGTH = 2 * Math.PI * RING_RADIUS;

/** Проценты → доля окружности: деление на именованную сотню — гейт #330
 * бареет литеральную /100 (деньги); здесь масштаб геометрический,
 * не копеечный. */
const PERCENT_SCALE = 100;

export type PhotoProgressRingProps = {
  /** Проценты выгрузки 0–100 (вне диапазона зажимаются). */
  readonly percent: number;
};

/**
 * Кольцо прогресса поверх круга фото (план 036, тикет #1423): дорожка —
 * полупрозрачный белый, дуга — primary; поворот −90° — старт сверху.
 * Ложится абсолютом в относительный родитель круга слота (div с бейджем
 * удаления). Без текста процентов (круг мелок), без спиннера — кнопочный
 * канон #1119; на 100% дуга держит полный круг до факта успеха/отказа
 * (сервер ещё обрабатывает).
 */
export function PhotoProgressRing({ percent }: PhotoProgressRingProps): JSX.Element {
  const clamped = Math.min(100, Math.max(0, percent));
  const offset = PHOTO_RING_LENGTH * (1 - clamped / PERCENT_SCALE);
  return (
    <svg
      aria-hidden
      viewBox="0 0 96 96"
      className="absolute inset-0 h-full w-full -rotate-90"
    >
      <circle
        cx="48"
        cy="48"
        r={RING_RADIUS}
        fill="none"
        strokeWidth="4"
        className="stroke-white/60"
      />
      <circle
        cx="48"
        cy="48"
        r={RING_RADIUS}
        fill="none"
        strokeWidth="4"
        strokeLinecap="round"
        strokeDasharray={PHOTO_RING_LENGTH}
        strokeDashoffset={offset}
        className="stroke-primary"
      />
    </svg>
  );
}
