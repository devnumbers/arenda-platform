import type { ParticipantPropertyLeg } from '../model/types';

/** Иконка чипа ноги: Edit («Редактирование»), Eye («Просмотр»),
 * Lock (warning «Превышен лимит объектов»); undefined — без иконки. */
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
 * активная нога показывает роль (решение чарта карты #692 — роли только
 * отображение: full_access → «Редактирование» с Icon/S/Edit, viewer →
 * «Просмотр» с Icon/S/Eye); pending — «Приглашён» без иконки (роли у
 * приглашения ещё нет); suspended — «Превышен лимит объектов» с замком,
 * канон warning-чипа агрегата #697 (Icon/S/Lock).
 */
export function participantLegBadge(leg: ParticipantPropertyLeg): ParticipantLegBadge {
  switch (leg.status) {
    case 'pending':
      return { tone: 'neutral', label: 'Приглашён', icon: undefined };
    case 'suspended':
      return { tone: 'warning', label: 'Превышен лимит объектов', icon: 'lock' };
    case 'active':
      return leg.role === 'full_access'
        ? { tone: 'neutral', label: 'Редактирование', icon: 'edit' }
        : { tone: 'neutral', label: 'Просмотр', icon: 'eye' };
  }
}
