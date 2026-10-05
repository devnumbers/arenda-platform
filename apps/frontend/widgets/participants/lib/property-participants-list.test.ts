import { describe, expect, it } from 'vitest';
import { ACCESS_ROLE_LABELS, type PropertyAccessMember } from '@/entities/access';
import {
  filterPropertyParticipantsByQuery,
  parsePropertyParticipantOrderParams,
  parsePropertyParticipantRoleFilterParams,
  propertyParticipantRows,
  ROLE_FILTER_LABELS,
  serializePropertyParticipantOrderToParams,
  serializePropertyParticipantRoleFilterToParams,
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

  it('безымянный зарегистрированный: «Пользователь» в титуле, почта подзаголовком (канон карты #1105, аменд #1123)', () => {
    // display_name безымянного юзера — анонимный лейбл (шов displayName
    // бекенда, #1106, аменд #1123): телефон и маски на поверхности не
    // бывают.
    const rows = propertyParticipantRows(
      [member({ id: 'm-nameless', email: 'nameless@example.com', displayName: 'Пользователь' })],
      OWNER_ID,
    );

    expect(rows[0]?.title).toBe('Пользователь');
    expect(rows[0]?.subtitle).toBe('nameless@example.com');
  });

  it('своя безымянная строка: «(Вы)» прицепляется к «Пользователь» как к любому титулу', () => {
    const MY_NAMELESS_ID = '16111111-1111-4111-8111-111111111161';
    const rows = propertyParticipantRows(
      [
        member({
          id: 'm-me-nameless',
          userId: MY_NAMELESS_ID,
          email: 'nameless@example.com',
          displayName: 'Пользователь',
        }),
      ],
      MY_NAMELESS_ID,
    );

    expect(rows[0]?.title).toBe('Пользователь (Вы)');
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

describe('своя строка читателя — isMe (тикет #770)', () => {
  const MY_ID = '12111111-1111-4111-8111-111111111121';

  it('manage-участник на чужом объекте: своя строка isMe, чужие — нет', () => {
    const rows = propertyParticipantRows(
      [owner, member({ id: 'm-me', userId: MY_ID, displayName: 'Мария Петрова' }), pending],
      MY_ID,
    );
    const byKey = new Map(rows.map((row) => [row.key, row]));

    expect(byKey.get('m-me')?.isMe).toBe(true);
    expect(byKey.get('m-me')?.title).toBe('Мария Петрова (Вы)');
    expect(byKey.get('owner-row')?.isMe).toBe(false);
    // pending-строка идентифицируется почтой, не uuid — «своей» быть не может.
    expect(byKey.get('invitation-1')?.isMe).toBe(false);
  });

  it('читатель-владелец: ряд владельца тоже свой', () => {
    const rows = propertyParticipantRows([owner, maria], OWNER_ID);

    expect(rows[0]?.isMe).toBe(true);
    expect(rows[1]?.isMe).toBe(false);
  });

  it('пока /me не известно — ни одна строка не своя', () => {
    const rows = propertyParticipantRows([owner, member({ userId: MY_ID })], undefined);

    expect(rows.every((row) => !row.isMe)).toBe(true);
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

describe('адрес «Участников объекта» — parse порядок и фильтра ролей (#785)', () => {
  it('направление: отсутствие, пустое, неизвестное и массивное — дефолт «А→Я»', () => {
    expect(parsePropertyParticipantOrderParams(undefined)).toBe('asc');
    expect(parsePropertyParticipantOrderParams('')).toBe('asc');
    expect(parsePropertyParticipantOrderParams('по роли')).toBe('asc');
    expect(parsePropertyParticipantOrderParams(['desc'])).toBe('asc');
    expect(parsePropertyParticipantOrderParams('desc')).toBe('desc');
  });

  it('фильтр ролей: отсутствие, мусор и owner — «Все роли»', () => {
    expect(parsePropertyParticipantRoleFilterParams(undefined)).toBe('all');
    expect(parsePropertyParticipantRoleFilterParams('')).toBe('all');
    expect(parsePropertyParticipantRoleFilterParams('owner')).toBe('all');
    expect(parsePropertyParticipantRoleFilterParams(['viewer'])).toBe('all');
  });

  it('фильтр ролей читает канонические значения ролей', () => {
    expect(parsePropertyParticipantRoleFilterParams('viewer')).toBe('viewer');
    expect(parsePropertyParticipantRoleFilterParams('full_access')).toBe('full_access');
  });
});

describe('адрес «Участников объекта» — serialize патчей (#785)', () => {
  it('дефолты параметров не создают — пустой query даёт голый адрес', () => {
    expect(serializePropertyParticipantOrderToParams('asc')).toStrictEqual({});
    expect(serializePropertyParticipantRoleFilterToParams('all')).toStrictEqual({});
  });

  it('не-дефолты пишутся, каждый в свой ключ', () => {
    expect(serializePropertyParticipantOrderToParams('desc')).toStrictEqual({ order: 'desc' });
    expect(serializePropertyParticipantRoleFilterToParams('viewer')).toStrictEqual({ role: 'viewer' });
    expect(serializePropertyParticipantRoleFilterToParams('full_access')).toStrictEqual({
      role: 'full_access',
    });
  });
});

describe('ROLE_FILTER_LABELS — подписи фильтра ролей из канона ролей', () => {
  it('«Все роли» + канон ACCESS_ROLE_LABELS, второй копии литералов нет', () => {
    expect(ROLE_FILTER_LABELS.all).toBe('Все роли');
    expect(ROLE_FILTER_LABELS.viewer).toBe(ACCESS_ROLE_LABELS.viewer);
    expect(ROLE_FILTER_LABELS.full_access).toBe(ACCESS_ROLE_LABELS.full_access);
  });
});
