import type { ParticipantPropertyLeg } from '../model/types';

/** Иконка чипа ноги: Edit («Редактирование»), Eye («Просмотр»);
 * undefined — без иконки. */
export type ParticipantLegBadgeIcon = 'edit' | 'eye' | 'lock';

/** Тон чипа ноги — общий с агрегат-чипом (#697): warning — жёлтая
 * подложка, neutral — серая. */
export type ParticipantLegBadgeTone = 'neutral' | 'warning';

export type ParticipantLegBadge = {
  readonly tone: ParticipantLegBadgeTone;
  readonly label: string;
  readonly icon: ParticipantLegBadgeIcon | undefined;
};

/**
 * Чип ноги доступа в ряде «Доступных объектов» (страница участника #698;
 * макеты 2008-81468, 2177-59620 — компонент Row Button Bage 2036:83107):
 * активная и suspended нога показывает роль (решение чарта карты #692 —
 * роли только отображение: full_access → «Редактирование» с Icon/S/Edit,
 * viewer → «Просмотр» с Icon/S/Eye); suspended-состояние коммуницируется
 * на уровне участника — warning-чипом в шапке и жёлтой карточкой
 * (макет 2036-84861, правка приёмки #756), а не бейджем ноги.
 * pending — «Приглашён» без иконки (роли у приглашения ещё нет).
 */
export function participantLegBadge(leg: ParticipantPropertyLeg): ParticipantLegBadge {
  if (leg.status === 'pending') {
    return { tone: 'neutral', label: 'Приглашён', icon: undefined };
  }
  return leg.role === 'full_access'
    ? { tone: 'neutral', label: 'Редактирование', icon: 'edit' }
    : { tone: 'neutral', label: 'Просмотр', icon: 'eye' };
}
