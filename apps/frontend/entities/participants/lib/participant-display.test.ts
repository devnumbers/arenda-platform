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

describe('participantStatusBadge — чип агрегат-статуса (макеты 2036-82971, 2008-82943)', () => {
  it('all_properties — «Доступ ко всем объектам», серый без иконки', () => {
    expect(participantStatusBadge(participant())).toEqual({
      tone: 'neutral',
      label: 'Доступ ко всем объектам',
      icon: undefined,
    });
  });

  it('partial — «Доступно N объектов» с русской плюрализацией', () => {
    expect(participantStatusBadge(participant({ aggregateStatus: 'partial', accessiblePropertiesCount: 3 })).label).toBe(
      'Доступно 3 объекта',
    );
    expect(participantStatusBadge(participant({ aggregateStatus: 'partial', accessiblePropertiesCount: 1 })).label).toBe(
      'Доступно 1 объект',
    );
    expect(participantStatusBadge(participant({ aggregateStatus: 'partial', accessiblePropertiesCount: 5 })).label).toBe(
      'Доступно 5 объектов',
    );
  });

  it('limit_exceeded — «Превышен лимит объектов», warning с замком', () => {
    expect(participantStatusBadge(participant({ aggregateStatus: 'limit_exceeded' }))).toEqual({
      tone: 'warning',
      label: 'Превышен лимит объектов',
      icon: 'lock',
    });
  });

  it('pending-агрегат (юзера нет, все ноги pending) — «Приглашён», нейтральный без замка (Q11=А, #772)', () => {
    const p = participant({
      id: 'invitee@example.com',
      userId: undefined,
      displayName: undefined,
      email: 'invitee@example.com',
      aggregateStatus: 'partial',
      accessiblePropertiesCount: 0,
      properties: [leg()],
    });

    // «Доступно 0 объектов» читалось как отказ — приглашённому честнее
    // «Приглашён».
    expect(participantStatusBadge(p)).toEqual({
      tone: 'neutral',
      label: 'Приглашён',
      icon: undefined,
    });
  });

  it('зарегистрированный без активных ног — чипы как прежде (детектор pending не срабатывает)', () => {
    // На проводе не встречается (registered-бакет собирается только из
    // memberships), но предикат не должен прятать честный счётчик.
    expect(
      participantStatusBadge(participant({ aggregateStatus: 'partial', accessiblePropertiesCount: 0 })).label,
    ).toBe('Доступно 0 объектов');
  });

  it('pending-агрегат с активной ногой (на проводе невозможно) — считанный статус, не «Приглашён»', () => {
    const p = participant({
      aggregateStatus: 'partial',
      accessiblePropertiesCount: 1,
      properties: [leg({ status: 'active' })],
    });

    expect(participantStatusBadge(p).label).toBe('Доступно 1 объект');
  });
});
