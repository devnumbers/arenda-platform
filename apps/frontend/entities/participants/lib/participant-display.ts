import { pluralize } from '@/shared/lib/pluralize';
import type { Participant } from '../model/types';

/** Тон чипа агрегат-статуса: warning — жёлтая подложка с замком
 * («Превышен лимит объектов», макет 2036-82971, Row Button Bage Warning),
 * neutral — серая без иконки. */
export type ParticipantBadgeTone = 'neutral' | 'warning';

export type ParticipantBadge = {
  readonly tone: ParticipantBadgeTone;
  readonly label: string;
  /** Замок — только у warning-чипа лимита (Icon/S/Lock, макет 2036-83931). */
  readonly withLock: boolean;
};

/** Титул строки списка: имя; у pending-строки (имени нет) — почта
 * (контракт #693: «the email is the label»). */
export function participantRowTitle(participant: Participant): string {
  return participant.displayName ?? participant.email ?? '';
}

/** Подзаголовок строки: почта; у pending-строки почта уже титул — дважды
 * не повторяется (макет 2036-82971: title + email-подзаголовок). */
export function participantRowSubtitle(participant: Participant): string | undefined {
  if (participant.displayName === undefined) {
    return undefined;
  }
  return participant.email;
}

/** Чип агрегат-статуса (подписи контракта #693): «Доступ ко всем
 * объектам» / «Доступно N объектов» / «Превышен лимит объектов». */
export function participantStatusBadge(participant: Participant): ParticipantBadge {
  switch (participant.aggregateStatus) {
    case 'limit_exceeded':
      return { tone: 'warning', label: 'Превышен лимит объектов', withLock: true };
    case 'all_properties':
      return { tone: 'neutral', label: 'Доступ ко всем объектам', withLock: false };
    case 'partial':
      return {
        tone: 'neutral',
        label: `Доступно ${participant.accessiblePropertiesCount} ${pluralize(
          participant.accessiblePropertiesCount,
          'объект',
          'объекта',
          'объектов',
        )}`,
        withLock: false,
      };
  }
}
