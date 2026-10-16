import { afterEach, describe, expect, it } from 'vitest';

import {
  getParticipantPopupSnapshot,
  popParticipantPopup,
  stageParticipantPopup,
} from './participant-popups';

// Канон one-shot флага в памяти модуля (#698; web storage запрещён #331).
// #771: обход #759 поймал редкую потерю попапа «Участник приглашен» на хабе;
// гипотеза «маунт читателя сбрасывает флаг» кодом не подтверждается — сброс
// только по закрытию попапа (pop). Тесты закрепляют: флаг не гасится
// чтением/перемонтами читателя и расходуется ровно один раз.

describe('participant-popups: one-shot флаг в памяти модуля', () => {
  afterEach(() => {
    popParticipantPopup();
  });

  it('флаг не гасится чтением — перемонты читателя его переживают', () => {
    stageParticipantPopup('invited');
    expect(getParticipantPopupSnapshot()).toBe('invited');
    expect(getParticipantPopupSnapshot()).toBe('invited');
    expect(popParticipantPopup()).toBe('invited');
    expect(getParticipantPopupSnapshot()).toBeNull();
  });

  it('повторная постановка перезаписывает значение (последний источник побеждает)', () => {
    stageParticipantPopup('invited');
    stageParticipantPopup('granted');
    expect(popParticipantPopup()).toBe('granted');
    expect(getParticipantPopupSnapshot()).toBeNull();
  });

  it('pop без staged-флага — null, состояние не меняется', () => {
    expect(popParticipantPopup()).toBeNull();
    expect(getParticipantPopupSnapshot()).toBeNull();
  });
});
