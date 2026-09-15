// Чистая логика панели управления временем жизненного цикла подписки
// (issue #666): каталог пресетов приёмки, границы сырого сдвига и расписание
// ретраев долга. Значения пресетов синхронизированы с реестром TimeShiftPreset
// apps/backend/internal/billing/application/admin_time_travel_service.go,
// расписание ретраев — с apps/backend/internal/billing/application/phases.go.

/**
 * Пресеты сервера — реестр TimeShiftPreset. Сервер считает дельту от живого
 * состояния, поэтому кнопка сценария приводит границу к цели, а не стакает
 * слепые офсеты.
 */
export const timeShiftPresets = ['period_expired', 'retry_24h_due', 'retry_72h_due', 'reminder_window'] as const;

export type TimeShiftPreset = (typeof timeShiftPresets)[number];

/**
 * Кнопки сценариев приёмки полного цикла (гриллинг #648): пять кнопок над
 * четырьмя пресетами — «войти в grace» и «истечь из grace» ссылаются на один
 * period_expired, фазы воркера различают сценарии по найденному статусу
 * подписки.
 */
export const acceptanceScenarioButtons: readonly { id: string; preset: TimeShiftPreset; name: string }[] = [
  { id: 'enter_grace', preset: 'period_expired', name: 'Войти в grace' },
  { id: 'retry_24', preset: 'retry_24h_due', name: 'В окно ретрая +24' },
  { id: 'retry_72', preset: 'retry_72h_due', name: 'В окно ретрая +72' },
  { id: 'reminder', preset: 'reminder_window', name: 'В окно напоминания' },
  { id: 'expire_grace', preset: 'period_expired', name: 'Истечь из grace' },
];

/** Лимит одного сдвига ±90 дней (MaxTimeShift) в часах — тот же, что у API. */
export const maxTimeShiftHours = 90 * 24;

/** Сырой сдвиг: целое число часов, ненулевое, в пределах ±MaxTimeShift. */
export const isValidTimeShiftHours = (hours: number): boolean =>
  Number.isInteger(hours) && hours !== 0 && Math.abs(hours) <= maxTimeShiftHours;

/** Одна точка расписания ретраев: метка, момент и признак «уже должен был пройти». */
export interface RetrySchedulePoint {
  label: string;
  at: Date;
  due: boolean;
}

const hourMs = 60 * 60 * 1000;

/**
 * Расписание ретраев долга от анкера (ms эпохи): +24 ч и +72 ч — продуктовая
 * константа фазы долга, не конфиг. null — анкера нет (пользователь не входил
 * в grace или история переходов недоступна).
 */
export function retrySchedule(anchorMs: null, now: Date): null;
export function retrySchedule(anchorMs: number, now: Date): RetrySchedulePoint[];
export function retrySchedule(anchorMs: number | null, now: Date): RetrySchedulePoint[] | null {
  if (anchorMs === null) {
    return null;
  }
  const dueAt = (offsetHours: number): { at: Date; due: boolean } => ({
    at: new Date(anchorMs + offsetHours * hourMs),
    due: anchorMs + offsetHours * hourMs <= now.getTime(),
  });
  const first = dueAt(24);
  const second = dueAt(72);
  return [
    { label: 'Ретрай +24 ч', ...first },
    { label: 'Ретрай +72 ч', ...second },
  ];
}

/** Минимальная форма перехода истории подписки, нужная для поиска анкера. */
export type TransitionTimestamp = {
  createdAt?: string | null;
  reason?: string | null;
};

/**
 * Анкер ретраев — created_at последнего перехода grace_entered: фаза ретраев
 * читает именно его (сдвиг #665 переносит анкер вместе с границами).
 */
export const latestGraceEntryAt = (transitions: readonly TransitionTimestamp[]): number | null => {
  const anchors = transitions
    .filter((transition): transition is { createdAt: string; reason: string } =>
      transition.reason === 'grace_entered' && typeof transition.createdAt === 'string')
    .map((transition) => Date.parse(transition.createdAt))
    .filter((ms) => !Number.isNaN(ms));
  if (anchors.length === 0) {
    return null;
  }
  return Math.max(...anchors);
};
