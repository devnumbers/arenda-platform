// Чистая логика блока «Подписка» карточки пользователя (issue #255):
// словари переходов подписки, правило предупреждения об оплаченном остатке
// при назначении служебной подписки и расчёт остатка дней. Значения enum
// синхронизированы с apps/backend/api/openapi/openapi.yaml.

export interface Choice {
  id: string;
  name: string;
}

/** Источник подписки: платная или назначенная админом служебная. */
export const subscriptionSourceChoices: Choice[] = [
  { id: 'paid', name: 'Платная' },
  { id: 'service', name: 'Служебная' },
];

/** Инициатор перехода подписки. */
export const transitionInitiatorChoices: Choice[] = [
  { id: 'user', name: 'Пользователь' },
  { id: 'admin', name: 'Админ' },
  { id: 'system', name: 'Система' },
];

/**
 * Причины переходов подписки — реестр TransitionReason
 * apps/backend/internal/billing/domain/transition.go. Неизвестная причина
 * (журнал append-only, новые причины приходят со старыми строками) показывается
 * как есть.
 */
const transitionReasonNames: Record<string, string> = {
  registered: 'Регистрация',
  cancelled: 'Отмена',
  resumed: 'Возобновление',
  downgrade_scheduled: 'Запланирован даунгрейд',
  payment_applied: 'Оплата применена',
  grace_entered: 'Вход в грейс',
  grace_extended: 'Грейс продлён',
  scheduled_change_applied: 'Отложенная смена применена',
  expired: 'Истечение',
  refunded: 'Возврат платежа',
  service_assigned: 'Назначена служебная',
  forced_change: 'Принудительная смена тарифа',
};

export const transitionReasonLabel = (reason: string): string => transitionReasonNames[reason] ?? reason;

/** Часть карточки пользователя: subscription из GET /admin/users/{id}. */
export interface UserSubscriptionRecord {
  source?: 'paid' | 'service' | null;
  status?: 'active' | 'grace' | 'cancelled' | null;
  validUntil?: string | null;
  autoRenewEnabled?: boolean;
}

/**
 * Остаток оплаченного/служебного периода в целых днях (округление вверх):
 * неполный день считается днём. null — срока нет (базовый тариф).
 */
export const remainderDays = (validUntil: string | null | undefined, now: Date): number | null => {
  if (!validUntil) {
    return null;
  }
  const until = Date.parse(validUntil);
  if (Number.isNaN(until)) {
    return null;
  }
  return Math.max(0, Math.ceil((until - now.getTime()) / (24 * 60 * 60 * 1000)));
};

/**
 * Предупреждение об оплаченном остатке при назначении служебной подписки
 * (issue #255): назначение перезаписывает подписку, оплаченные дни не
 * возвращаются и не суммируются. null — предупреждать не о чем.
 */
export const paidRemainderWarning = (subscription: UserSubscriptionRecord, now: Date): string | null => {
  if (subscription.source !== 'paid' || !subscription.validUntil) {
    return null;
  }
  const days = remainderDays(subscription.validUntil, now);
  if (days === null || days <= 0) {
    return null;
  }
  const daysText = days === 1 ? '1 день' : `${days} дней`;
  return `У пользователя оплачен текущий тариф ещё на ${daysText}. Назначение служебной подписки перезапишет его: оплаченный остаток не возвращается и не суммируется.`;
};

/** Доступна ли операция отмены от имени пользователя: только платная и не отменённая. */
export const canCancelOnBehalf = (subscription: UserSubscriptionRecord): boolean =>
  subscription.source === 'paid' && subscription.status !== 'cancelled';

/** Доступно ли продление грейса: только подписка в статусе grace. */
export const canExtendGrace = (subscription: UserSubscriptionRecord): boolean => subscription.status === 'grace';
