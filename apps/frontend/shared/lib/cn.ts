import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

/** Слияние классов дизайн-слоя (ADR 0050): clsx отбрасывает falsy,
 * tailwind-merge разрешает конфликты Tailwind-утилит (победа последнего). */
export function cn(...inputs: ReadonlyArray<ClassValue>): string {
  return twMerge(clsx(inputs));
}
