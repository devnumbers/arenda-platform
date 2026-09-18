import { describe, expect, it } from 'vitest';
import type { SessionDevice } from '@/entities/session';
import {
  deviceIconName,
  sessionSubtitle,
  sessionTitle,
  splitSessions,
} from './devices';

const now = new Date('2026-09-18T12:00:00');

function device(overrides: Partial<SessionDevice>): SessionDevice {
  return {
    id: 's1',
    deviceType: 'computer',
    browser: 'Chrome',
    browserMajor: 121,
    os: 'macOS',
    city: 'Москва',
    lastIp: '91.108.4.10',
    lastSeenAt: '2026-09-18T10:00:00Z',
    createdAt: '2026-09-01T08:00:00Z',
    current: false,
    ...overrides,
  };
}

describe('splitSessions', () => {
  it('текущая сессия отделяется, прочие сохраняют порядок бэка', () => {
    const current = device({ id: 'cur', current: true });
    const other1 = device({ id: 'o1' });
    const other2 = device({ id: 'o2' });

    expect(splitSessions([other1, current, other2])).toStrictEqual({
      current,
      others: [other1, other2],
    });
  });

  it('без текущей сессии — current null, список полный', () => {
    const other = device({ id: 'o1' });
    expect(splitSessions([other])).toStrictEqual({ current: null, others: [other] });
  });

  it('одна только текущая — других нет', () => {
    const current = device({ id: 'cur', current: true });
    expect(splitSessions([current])).toStrictEqual({ current, others: [] });
  });
});

describe('sessionTitle', () => {
  it('браузер с мажорной версией: «Chrome 121»', () => {
    expect(sessionTitle(device({}))).toBe('Chrome 121');
  });

  it('браузер без версии: «Safari»', () => {
    expect(sessionTitle(device({ browser: 'Safari', browserMajor: null }))).toBe('Safari');
  });

  it('нераспознанный браузер — ОС: «macOS»', () => {
    expect(sessionTitle(device({ browser: '', browserMajor: null }))).toBe('macOS');
  });

  it('не распознано ничего — «Устройство»', () => {
    expect(sessionTitle(device({ browser: '', browserMajor: null, os: '' }))).toBe('Устройство');
  });
});

describe('deviceIconName', () => {
  it('телефон — иконка телефона', () => {
    expect(deviceIconName('phone')).toBe('phone');
  });

  it('планшет и ТВ — большой экран; нераспознанное — компьютер', () => {
    expect(deviceIconName('tablet')).toBe('computer');
    expect(deviceIconName('tv')).toBe('computer');
    expect(deviceIconName('computer')).toBe('computer');
    expect(deviceIconName('unknown')).toBe('computer');
  });
});

describe('sessionSubtitle', () => {
  it('текущая сессия — «В сети» и город синим тоном', () => {
    expect(sessionSubtitle(device({ current: true }), now)).toStrictEqual({
      text: 'В сети • Москва',
      online: true,
    });
    expect(sessionSubtitle(device({ current: true, city: null }), now)).toStrictEqual({
      text: 'В сети',
      online: true,
    });
  });

  it('чужая сессия — момент активности и город серым тоном', () => {
    expect(
      sessionSubtitle(
        device({ current: false, lastSeenAt: '2026-08-14T14:41:00', city: 'Екатеринбург' }),
        now,
      ),
    ).toStrictEqual({ text: '14 августа, 14:41 • Екатеринбург', online: false });
    expect(
      sessionSubtitle(device({ current: false, lastSeenAt: '2025-08-14T09:08:00', city: null }), now),
    ).toStrictEqual({ text: '14 августа 2025, 09:08', online: false });
  });
});
