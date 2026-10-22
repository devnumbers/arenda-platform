/**
 * Геометрия рядов круглых иконок категорий (полоса «Регулярных платежей»
 * на детали объекта, стопки карточек хаба «Платежи»): сколько кругов
 * влезает в один ряд — невлезающие не показываются (решение владельца
 * 22.10: на телефоне лишний круг заворачивался/наезжал, теперь его не
 * рендерим). Круг 44 (h-11), шаг нахлёста 32 (−12px: gap -12 / -ml-3);
 * потолок задаёт модель группы (обе группы — 4, одна — 7). Ширину колонки
 * клиент меряет ResizeObserver'ом; до замера — колонка эталонного
 * телефона 375 (как в calendar-month-metrics): фолбэк уже точен на всех
 * актуальных телефонах, замер правит только широкие окна (4-й круг) и
 * legacy-узкие (<330: три не влезают — видно два).
 */

/** Диаметр круга иконки: CategoryIcon h-11 w-11. */
const CIRCLE_PX = 44;

/** Шаг между кругами: 44 − 12 нахлёста (gap -12 / -ml-3). */
const STEP_PX = 32;

/** Зазор между колонками групп: gap-4. */
const GROUPS_GAP_PX = 16;

/** Контентная ширина эталонного телефона 375 (обёртка px-6 + паддинг
 * карточки/секции p-6: 375 − 48 − 48) — вся колонка одной группы. */
const FULL_COLUMN_AT_375_PX = 279;

/** …и полуколонка при двух группах: (279 − 16) / 2. */
const HALF_COLUMN_AT_375_PX = 131.5;

/** Сколько кругов по CIRCLE_PX с шагом STEP_PX влезает в width, под
 * потолком max; меньше одного круга — ноль. */
export function iconsFitInRow(width: number, max: number): number {
  if (width < CIRCLE_PX) {
    return 0;
  }
  return Math.min(max, Math.floor((width - CIRCLE_PX) / STEP_PX) + 1);
}

/** Ширина колонки группы: одна — весь контент, две — пополам с зазором. */
export function stackColumnWidth(contentWidth: number, singleGroup: boolean): number {
  return singleGroup ? contentWidth : (contentWidth - GROUPS_GAP_PX) / 2;
}

/** Ширина для первого кадра (до замера): колонка эталонного 375. */
export function stackFallbackWidth(singleGroup: boolean): number {
  return singleGroup ? FULL_COLUMN_AT_375_PX : HALF_COLUMN_AT_375_PX;
}
