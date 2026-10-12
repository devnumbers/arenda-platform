import { describe, expect, it } from 'vitest';
import type { Participant } from '../model/types';
import {
  participantRowSubtitle,
  participantRowTitle,
  participantStatusBadge,
} from './participant-display';

function participant(
  overrides: Partial<Participant> = {},
): Participant {
  return {
    id: '12111111-1111-4111-8111-111111111121',
    userId: '12111111-1111-4111-8111-111111111121',
    email: 'maria@example.com',
    displayName: 'Мария Петрова',
    aggregateStatus: 'all_properties',
    photoUrl: null,
    accessiblePropertiesCount: 3,
    properties: [],
    ...overrides,
  };
}

function leg(
  overrides: Partial<Participant['properties'][number]> = {},
): Participant['properties'][number] {
  return {
    propertyId: '33333333-3333-4333-8333-333333333333',
    title: 'Квартира на Ленина',
    role: 'viewer',
    status: 'pending',
    type: 'apartment',
    photoUrl: null,
    ...overrides,
  };
}

describe('participantRowTitle / participantRowSubtitle', () => {
  it('у зарегистрированного титул — имя, подзаголовок — почта', () => {
    const p = participant();

    expect(participantRowTitle(p)).toBe('Мария Петрова');
    expect(participantRowSubtitle(p)).toBe('maria@example.com');
  });

  it('у зарегистрированного без почты подзаголовка нет', () => {
    const p = participant({ email: undefined });

    expect(participantRowTitle(p)).toBe('Мария Петрова');
    expect(participantRowSubtitle(p)).toBeUndefined();
  });

  it('у зарегистрированного без имени титул — «Пользователь», почта подзаголовком (канон #1105, аменд #1123)', () => {
    // display_name безымянного собирает бекенд (#1106, аменд #1123):
    // «Имя Фамилия», иначе «Пользователь» — телефон и маски на
    // поверхности не бывают.
    const p = participant({ displayName: 'Пользователь' });

    expect(participantRowTitle(p)).toBe('Пользователь');
    expect(participantRowSubtitle(p)).toBe('maria@example.com');
  });

  it('pending-строка: почта — титул, подзаголовка нет (макет дважды почту не повторяет)', () => {
    const p = participant({
      id: 'invitee@example.com',
      userId: undefined,
      displayName: undefined,
      email: 'invitee@example.com',
    });

    expect(participantRowTitle(p)).toBe('invitee@example.com');
    expect(participantRowSubtitle(p)).toBeUndefined();
  });
});

describe('participantStatusBadge — ровно два статусных бейджа (макет 3313-77128, D3 #1313)', () => {
  it('pending-агрегат (юзера нет, все ноги pending) — «Аккаунт не создан», info с замком', () => {
    const p = participant({
      id: 'invitee@example.com',
      userId: undefined,
      displayName: undefined,
      email: 'invitee@example.com',
      aggregateStatus: 'partial',
      accessiblePropertiesCount: 0,
      properties: [leg()],
    });

    expect(participantStatusBadge(p)).toEqual({
      tone: 'info',
      label: 'Аккаунт не создан',
      icon: 'lock',
    });
  });

  it('limit_exceeded — «Превышен лимит объектов», warning с замком', () => {
    expect(participantStatusBadge(participant({ aggregateStatus: 'limit_exceeded' }))).toEqual({
      tone: 'warning',
      label: 'Превышен лимит объектов',
      icon: 'lock',
    });
  });

  it('all_properties — без бейджа: статусы «частично/все объекты» ушли с экранов (#1313)', () => {
    expect(participantStatusBadge(participant())).toBeNull();
  });

  it('partial — без бейджа, счётчик объектов больше не показывается (#1313)', () => {
    expect(
      participantStatusBadge(participant({ aggregateStatus: 'partial', accessiblePropertiesCount: 3 })),
    ).toBeNull();
  });

  it('зарегистрированный без активных ног (на проводе не встречается) — без бейджа', () => {
    // Registered-бакет собирается только из memberships, но предикат
    // pending не должен выдумывать «Аккаунт не создан» зарегистрированному.
    expect(
      participantStatusBadge(participant({ aggregateStatus: 'partial', accessiblePropertiesCount: 0 })),
    ).toBeNull();
  });

  it('pending-агрегат с активной ногой (на проводе невозможно) — не «Аккаунт не создан»', () => {
    const p = participant({
      aggregateStatus: 'partial',
      accessiblePropertiesCount: 1,
      properties: [leg({ status: 'active' })],
    });

    expect(participantStatusBadge(p)).toBeNull();
  });
});
