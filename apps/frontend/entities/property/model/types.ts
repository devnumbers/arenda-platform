import type { AccessRole } from '@/shared/model/access';

export type PropertyStatus = 'active' | 'maintenance' | 'archived';
export type PropertyType =
  | 'apartment'
  | 'room'
  | 'apartments'
  | 'house'
  | 'commercial'
  | 'office'
  | 'warehouse'
  | 'garage'
  | 'parking'
  | 'land';

export type PropertyPhoto = {
  readonly id: string;
  readonly url: string;
};

export type PropertyAttributes = Readonly<Record<string, string | number>>;

/** Контекст доступа актора к объекту: роль, имя и почта владельца (имя —
 * для чужих объектов на списках и детали; почта — только в детали чужого
 * объекта, контактный ряд владельца, Figma 2200-97365). */
export type PropertyAccess = {
  readonly role: AccessRole;
  readonly ownerName?: string;
  readonly ownerEmail?: string;
};

/**
 * Занятость объекта (резолюция #584): вычисленный сервером статус из
 * единственной незавершённой аренды против «сегодня владельца» (ADR 0048) —
 * канон бейджей карточки, красной точки и сортировки «По статусу».
 * `needs_attention` — плановое окончание прошло, аренда не завершена;
 * `none` — незавершённой аренды нет (явно завершённая — история).
 */
export type PropertyOccupancyStatus =
  | 'upcoming'
  | 'active'
  | 'needs_attention'
  | 'none';

/** Даты — ISO-дата (YYYY-MM-DD) незавершённой аренды; null без аренды/срока. */
export type PropertyOccupancy = {
  readonly status: PropertyOccupancyStatus;
  readonly start_date?: string | null;
  readonly planned_end_date?: string | null;
};

export type Property = {
  readonly id: string;
  readonly name: string;
  readonly type: PropertyType;
  readonly address: string;
  readonly description?: string;
  readonly attributes: PropertyAttributes;
  readonly status: PropertyStatus;
  readonly photos?: PropertyPhoto[];
  readonly access?: PropertyAccess;
  readonly members_count: number;
  /** Момент создания (ISO date-time) — сортировка «По дате создания» (#586). */
  readonly created_at: string;
  /**
   * Основной объект (канон резолюции #584): null — обычный объект, момент —
   * основной с этого времени; среди основных первый отмеченный выше.
   * Контракт API держит имя pin (#577), домен — «основной объект».
   */
  readonly pinned_at: string | null;
  /** Занятость: только списочные рендеры (GET /properties, /properties/archive). */
  readonly occupancy?: PropertyOccupancy;
  /** Просроченные плановые операции — вторая причина красной точки (резолюция #584). */
  readonly has_overdue_operations?: boolean;
};
