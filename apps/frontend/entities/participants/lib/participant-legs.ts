import { ACCESS_ROLE_ICON_NAMES, ACCESS_ROLE_LABELS } from '@/shared/model/access';
import type { AccessRole } from '@/shared/model/access';
import type { ParticipantPropertyLeg } from '../model/types';
import type { ParticipantRowBadge } from './participant-badge';

/** Чип ноги доступа — общий ParticipantRowBadge (анатомия «Row Button
 * Bage», Figma 2036:83107), тот же тип, что у агрегат-чипа. */
export type ParticipantLegBadge = ParticipantRowBadge;

/**
 * Чип ноги доступа в ряде «Доступных объектов» (страница участника #698;
 * макеты 2008-81468, 2177-59620 — компонент Row Button Bage 2036:83107):
 * активная и suspended нога показывает роль (роли только отображение:
 * full_access → «Управление» с Icon/S/Edit, #1358,
 * viewer → «Просмотр» с Icon/S/Eye); suspended-состояние коммуницируется
 * на уровне участника — warning-чипом в шапке и жёлтой карточкой
 * (макет 2036-84861, правка приёмки #756), а не бейджем ноги.
 * pending — «Приглашён» без иконки (роли у приглашения ещё нет).
 * owner — «Владелец» с замком: вход читает member-режим «Прав участника»
 * (#1362), где строкой может быть и синтезированный ряд владельца
 * (страница участника ноги с ролью owner не бывает — здесь он не мешает).
 */
/** Вход бейджа — используемая часть ноги: статус и роль (тип/фото аватара
 * бейдж не читают). */
export function participantLegBadge(leg: {
  readonly status: ParticipantPropertyLeg['status'];
  readonly role: AccessRole;
}): ParticipantLegBadge {
  if (leg.status === 'pending') {
    return { tone: 'neutral', label: 'Приглашён', icon: undefined };
  }
  if (leg.role === 'owner') {
    return { tone: 'neutral', label: 'Владелец', icon: 'lock' };
  }
  return {
    tone: 'neutral',
    label: ACCESS_ROLE_LABELS[leg.role],
    icon: ACCESS_ROLE_ICON_NAMES[leg.role],
  };
}
