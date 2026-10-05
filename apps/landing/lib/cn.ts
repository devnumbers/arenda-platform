import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

// Слияние классов дизайн-слоя — перенос из apps/frontend (shared/lib/cn.ts):
// tailwind-merge знают кастомные радиусы токенов, иначе последний класс
// не побеждал бы в конфликтах rounded-*.
const twMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      rounded: ["rounded-pill", "rounded-card", "rounded-sheet", "rounded-button"],
    },
  },
});

export function cn(...inputs: ReadonlyArray<ClassValue>): string {
  return twMerge(clsx(inputs));
}
