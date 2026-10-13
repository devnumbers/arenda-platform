import type { IsoDate } from '@/shared/lib/calendar';
import { isoYear } from '@/shared/lib/calendar';
import { formatDayMonthDotted, formatDottedDate } from '@/shared/lib/date-format';
import { rentalRemainingLine } from '@/shared/lib/rental-remaining-line';
import type { Property } from '@/entities/property';

/**
 * Бейджи карточки объекта (резолюция #584, экран #586): один арендный бейдж
 * по приоритету плюс жёлтый «на ремонте», соседствующий с арендным.
 * Канон занятости — PropertyOccupancy (properties/CONTEXT.md); «сегодня
 * владельца» приходит из списочного ответа (ADR 0048) и нужен бейджам
 * «Осталось N месяцев» и «Аренда с» (правило года, владелец 13.10) —
 * остальные бейджи считаются сразу, пока today в пути, чтобы список
 * не мигал.
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

/** Бейдж списка — общая метка остатка (#1387): месяцы, при остатке меньше
 * месяца — дни (текст «Осталось меньше месяца» резолюции #584 уходит), в
 * сам день плана — «Заканчивается сегодня». Активная аренда с прошедшим
 * планом сервером не отдаётся (это уже needs_attention), поэтому фолбэк
 * недостижим и назван завершённым текстом. */
export function rentalRemainingLabel(
  today: IsoDate,
  plannedEndDate: IsoDate,
): string {
  return rentalRemainingLine(today, plannedEndDate) ?? 'Аренда завершена';
}

/** Арендный бейдж по приоритету резолюции #584; без аренды или у активной
 * бессрочной — null. Явно завершённая аренда — это occupancy=none: бейджа нет.
 * «Осталось N месяцев» и «Аренда с» считаются от today (правило года);
 * пока today не пришёл из ответа, бейдж откладывается — остальные
 * рендерятся сразу. */
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
        label: rentalRemainingLabel(today, occupancy.planned_end_date),
        tone: 'accent',
      };
    case 'upcoming':
      // Форма «DD.MM» из макета #586 — только в текущем году; начало бывает
      // и в следующем (декабрьское «сегодня») — там точечная дата с годом,
      // как у formatRangeBound (правило владельца 13.10).
      if (!occupancy.start_date || today === undefined) return null;
      return {
        key: 'rental-upcoming',
        label: `Аренда с ${isoYear(occupancy.start_date) === isoYear(today)
          ? formatDayMonthDotted(occupancy.start_date)
          : formatDottedDate(occupancy.start_date)}`,
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
