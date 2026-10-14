/** Тон чипа строки участника — общий для агрегат-чипа (#697) и чипа
 * ноги (#698): warning — жёлтая подложка, info — голубая (pending,
 * D6 #1312), neutral — серая. */
export type ParticipantRowBadgeTone = 'neutral' | 'warning' | 'info';

/** Иконка чипа: Edit («Управление»), Eye («Просмотр»), Lock
 * (warning-чип лимита); undefined — без иконки. */
export type ParticipantRowBadgeIcon = 'edit' | 'eye' | 'lock';

/** Чип строки участника — общая анатомия «Row Button Bage» (Figma
 * 2036:83107): пилюля radius 8, паддинг 4×8, зазор 4, текст 14/16;
 * warning — жёлтая подложка (--color-warning-bg). Один тип и один
 * компонент на оба чипа — различалась только иконка. */
export type ParticipantRowBadge = {
  readonly tone: ParticipantRowBadgeTone;
  readonly label: string;
  readonly icon?: ParticipantRowBadgeIcon | undefined;
};
