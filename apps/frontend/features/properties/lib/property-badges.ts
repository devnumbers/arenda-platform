import type { IsoDate } from '@/shared/lib/calendar';
import { fullMonthsBetween } from '@/shared/lib/calendar';
import { formatDayMonthDotted } from '@/shared/lib/date-format';
import { pluralize } from '@/shared/lib/pluralize';
import type { Property } from '@/entities/property';

/**
 * Бейджи карточки объекта (резолюция #584, экран #586): один арендный бейдж
 * по приоритету плюс жёлтый «на ремонте», соседствующий с арендным.
 * Канон занятости — PropertyOccupancy (properties/CONTEXT.md); «сегодня
 * владельца» приходит из списочного ответа (ADR 0048) и нужен только
 * бейджу «Осталось N месяцев» — остальные бейджи считаются сразу, пока
 * today в пути, чтобы список не мигал.
 */
export type PropertyBadgeKey =
  | 'rental-completed'
  | 'rental-months'
  | 'rental-upcoming'
  | 'maintenance'
  | 'archived';

export type PropertyBadgeTone = 'accent' | 'accent-filled' | 'warning' | 'neutral';

export type PropertyBadge = {
  readonly key: PropertyBadgeKey;
  readonly label: string;
  readonly tone: PropertyBadgeTone;
};

/** «Осталось N месяцев аренды»; N — полных месяцев от «сегодня владельца»
 * до планового окончания; при N=0 — «Осталось меньше месяца» (резолюция #584).
 * Грамматика единственного числа — как в строках аренды («Остался 1 месяц»). */
export function rentalMonthsLeftLabel(
  today: IsoDate,
  plannedEndDate: IsoDate,
): string {
  const months = fullMonthsBetween(today, plannedEndDate);
  if (months <= 0) {
    return 'Осталось меньше месяца';
  }
  if (months === 1) {
    return 'Остался 1 месяц аренды';
  }
  return `Осталось ${months} ${pluralize(months, 'месяц', 'месяца', 'месяцев')} аренды`;
}

/** Арендный бейдж по приоритету резолюции #584; без аренды или у активной
 * бессрочной — null. Явно завершённая аренда — это occupancy=none: бейджа нет.
 * «Осталось N месяцев» считается от today; пока today не пришёл из ответа,
 * бейдж откладывается — остальные рендерятся сразу. */
function rentalBadge(property: Property, today: IsoDate | undefined): PropertyBadge | null {
  const occupancy = property.occupancy;
  if (!occupancy) return null;

  switch (occupancy.status) {
    case 'needs_attention':
      // Текст как в макете: «период подошёл к концу, требует завершения».
      return { key: 'rental-completed', label: 'Аренда завершена', tone: 'accent-filled' };
    case 'active':
      if (!occupancy.planned_end_date || today === undefined) return null;
      return {
        key: 'rental-months',
        label: rentalMonthsLeftLabel(today, occupancy.planned_end_date),
        tone: 'accent',
      };
    case 'upcoming':
      if (!occupancy.start_date) return null;
      return {
        key: 'rental-upcoming',
        label: `Аренда с ${formatDayMonthDotted(occupancy.start_date)}`,
        tone: 'accent',
      };
    case 'none':
      return null;
  }
}

/** Все бейджи карточки: арендный (по приоритету) + «на ремонте». */
export function propertyBadges(property: Property, today?: IsoDate): PropertyBadge[] {
  const badges: PropertyBadge[] = [];
  const rental = rentalBadge(property, today);
  if (rental) badges.push(rental);
  if (property.status === 'maintenance') {
    badges.push({ key: 'maintenance', label: 'Объект на ремонте', tone: 'warning' });
  }
  return badges;
}

/** Единственный бейдж карточки архива (экран #587, макет 1603:92102):
 * белая пилюля с иконкой архива, смысловые бейджи занятости в архиве
 * не рендерятся. */
export const archivedPropertyBadge: PropertyBadge = {
  key: 'archived',
  label: 'В архиве',
  tone: 'neutral',
};

/** Красная точка карточки (резолюция #584): просроченная плановая операция
 * ИЛИ аренда «подошла к концу». Ничто другое точку не ставит. */
export function hasPropertyAttentionDot(property: Property): boolean {
  return property.has_overdue_operations === true
    || property.occupancy?.status === 'needs_attention';
}
