'use client';

import { useSyncExternalStore } from 'react';

/** Вид попапа успеха, который переход показывает на целевой странице. */
export type ParticipantPopupKind =
  /** «Участник приглашен» — список/хаб после приглашения из хаба (макет 2010-134458, #699). */
  | 'invited'
  /** «Доступ выдан» — страница участника (макет 2010-131859). */
  | 'granted'
  /** «У участника больше нет доступа к объекту» — страница участника (2008-83135). */
  | 'revokedFromProperty'
  /** «Участник удален» — список «Ваши участники» (2008-83716). */
  | 'deleted';

/**
 * Одноразовый флаг попапа, поставленный страницей-источником перед
 * возвратом по канону истории (goBack — CODING_STANDARDS «Навигация и
 * история браузера»): попап рендерит страница-приёмник при маунте. Живёт в
 * памяти вкладки — клиентская навигация его несёт, жёсткая перезагрузка
 * честно теряет (web storage для этого запрещён — решение #331), серверный
 * снапшот — null, поэтому гидрация не расходится.
 */
let staged: ParticipantPopupKind | null = null;
const listeners = new Set<() => void>();

export function stageParticipantPopup(kind: ParticipantPopupKind): void {
  staged = kind;
  for (const listener of listeners) {
    listener();
  }
}

/** Снять флаг (страница-приёмник — при закрытии попапа). */
export function popParticipantPopup(): ParticipantPopupKind | null {
  const value = staged;
  staged = null;
  for (const listener of listeners) {
    listener();
  }
  return value;
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot(): ParticipantPopupKind | null {
  return staged;
}

/** Текущее значение флага без React (юнит-тесты канона one-shot #771). */
export function getParticipantPopupSnapshot(): ParticipantPopupKind | null {
  return getSnapshot();
}

function getServerSnapshot(): ParticipantPopupKind | null {
  return null;
}

/** Реактивное чтение флага (страница-приёмник): значение прямо в рендере,
 * без эффектов и setState — канон React Compiler. */
export function useParticipantPopup(): ParticipantPopupKind | null {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
