import { clsx, type ClassValue } from 'clsx';
import { extendTailwindMerge } from 'tailwind-merge';

/** Слияние классов дизайн-слоя (ADR 0050): clsx отбрасывает falsy,
 * tailwind-merge разрешает конфликты Tailwind-утилит (победа последнего).
 * Кастомные радиусы из @theme (--radius-pill/card/sheet/button) учат мержер
 * группе rounded — иначе пара «rounded-pill + rounded-[32px]» остаётся
 * в обоих классах, и побеждает порядок утилит в stylesheet, а не
 * потребитель (проявилось на скелетоне hero «Тарифа», #620). */
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      rounded: ['rounded-pill', 'rounded-card', 'rounded-sheet', 'rounded-button'],
    },
  },
});

export function cn(...inputs: ReadonlyArray<ClassValue>): string {
  return twMerge(clsx(inputs));
}
