import { describe, expect, it } from 'vitest';
import type { PropertyAccessMember } from '@/entities/access';
import {
  filterPropertyParticipantsByQuery,
  propertyParticipantRows,
  sortPropertyParticipantsByTitle,
} from './property-participants-list';

const OWNER_ID = '11111111-1111-4111-8111-111111111111';

function member(overrides: Partial<PropertyAccessMember>): PropertyAccessMember {
  return {
    id: 'member-1',
    userId: null,
    email: 'maria@example.com',
    role: 'viewer',
    isOwner: false,
    displayName: 'Мария Петрова',
    status: 'active',
    ...overrides,
  };
}

const owner = member({
  id: 'owner-row',
  userId: OWNER_ID,
  email: 'ivan@example.com',
  role: 'owner',
  isOwner: true,
  displayName: 'Иван Иванов',
});
const maria = member({});
const sergey = member({
  id: 'member-2',
  email: 'sergey@mail.ru',
  displayName: 'Сергей Сидоров',
});
const pending = member({
  id: 'invitation-1',
  email: 'invitee@example.com',
  displayName: '',
  status: 'pending',
});
const suspended = member({
  id: 'member-3',
  email: 'oleg@example.com',
  displayName: 'Олег Олегов',
  status: 'suspended',
});

describe('propertyParticipantRows — VM ряда «Участники объекта» (макет 1980-107096)', () => {
  it('владелец идёт первым отдельным рядом, у читателя-владельца — «(Вы)»', () => {
    const rows = propertyParticipantRows([maria, owner, sergey], OWNER_ID);

    expect(rows.map((row) => row.role)).toEqual(['owner', 'viewer', 'viewer']);
    expect(rows[0]?.title).toBe('Иван Иванов (Вы)');
  });

  it('чужой владелец (читатель — viewer) без «(Вы)»', () => {
    const rows = propertyParticipantRows([owner, maria], '99999999-9999-4999-8999-999999999999');

    expect(rows[0]?.title).toBe('Иван Иванов');
  });

  it('почта — подзаголовок; у pending-строки почта уже титул, без дубля', () => {
    const rows = propertyParticipantRows([maria, pending], OWNER_ID);

    expect(rows[0]?.subtitle).toBe('maria@example.com');
    expect(rows[1]?.title).toBe('invitee@example.com');
    expect(rows[1]?.subtitle).toBeUndefined();
  });

  it('иконка у почты: владелец — замок, full_access — перо, viewer — глаз', () => {
    const rows = propertyParticipantRows(
      [
        maria,
        member({ id: 'm4', role: 'full_access' }),
        owner,
      ],
      OWNER_ID,
    );

    expect(rows.map((row) => row.emailIcon)).toEqual(['owner', 'eye', 'edit']);
  });

  it('suspended-чип только у приостановленного; participantId — uuid, у pending — почта', () => {
    const rows = propertyParticipantRows([maria, suspended, pending], OWNER_ID);
    const byKey = new Map(rows.map((row) => [row.key, row]));

    expect(byKey.get('member-1')?.suspended).toBe(false);
    expect(byKey.get('member-1')?.participantId).toBeUndefined(); // userId неизвестен
    expect(byKey.get('member-3')?.suspended).toBe(true);
    expect(byKey.get('invitation-1')?.participantId).toBe('invitee@example.com');
  });

  it('почта приходит в каждой строке контракта — без обогащения (решение #758)', () => {
    // Контракт members отдаёт email и у зарегистрированных, и у pending,
    // и у владельца — зритель видит те же адреса, что и владелец.
    const rows = propertyParticipantRows(
      [
        member({
          id: 'm-registered',
          userId: '12111111-1111-4111-8111-111111111199',
          email: 'maria@example.com',
          displayName: 'Мария Петрова',
        }),
        member({ id: 'm-owner', userId: OWNER_ID, email: 'ivan@example.com', role: 'owner', isOwner: true, displayName: 'Иван Иванов' }),
        pending,
      ],
      '99999999-9999-4999-8999-999999999999',
    );
    const byKey = new Map(rows.map((row) => [row.key, row]));

    expect(byKey.get('m-registered')?.subtitle).toBe('maria@example.com');
    expect(byKey.get('m-owner')?.subtitle).toBe('ivan@example.com');
    expect(byKey.get('invitation-1')?.title).toBe('invitee@example.com');
    // Почта pending уже титул — не дублируется.
    expect(byKey.get('invitation-1')?.subtitle).toBeUndefined();
  });

  it('без почты подзаголовок не рендерится (пустой строки с иконкой нет)', () => {
    const rows = propertyParticipantRows([member({ email: null })], OWNER_ID);

    expect(rows[0]?.subtitle).toBeUndefined();
  });
});

describe('filterPropertyParticipantsByQuery — клиентский поиск (имя или почта)', () => {
  const rows = propertyParticipantRows([owner, maria, pending], OWNER_ID);

  it('пустой запрос (и из пробелов) возвращает всё', () => {
    expect(filterPropertyParticipantsByQuery(rows, '')).toHaveLength(3);
    expect(filterPropertyParticipantsByQuery(rows, '  ')).toHaveLength(3);
  });

  it('ищет без регистра по имени и по почте', () => {
    expect(filterPropertyParticipantsByQuery(rows, 'петр')).toEqual([rows[1]]);
    expect(filterPropertyParticipantsByQuery(rows, 'INVITEE')).toEqual([rows[2]]);
  });
});

describe('sortPropertyParticipantsByTitle — чип «Имя»; владелец всегда первым', () => {
  it('asc по титулу, владелец не участвует и держится впереди', () => {
    const rows = propertyParticipantRows([owner, sergey, maria], OWNER_ID);
    const sorted = sortPropertyParticipantsByTitle(rows, 'asc');

    expect(sorted.map((row) => row.role)).toEqual(['owner', 'viewer', 'viewer']);
    expect(sorted[1]?.title).toBe('Мария Петрова');
  });

  it('desc — обратный порядок участников, владелец по-прежнему первым', () => {
    const rows = propertyParticipantRows([owner, sergey, maria], OWNER_ID);
    const sorted = sortPropertyParticipantsByTitle(rows, 'desc');

    expect(sorted[0]?.role).toBe('owner');
    expect(sorted[1]?.title).toBe('Сергей Сидоров');
  });
});
