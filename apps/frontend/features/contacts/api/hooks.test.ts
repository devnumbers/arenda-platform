import { beforeEach, describe, expect, it, vi } from 'vitest';

// Сеть подменяется стабом, ловящим путь: тест пинит wire-контракт общего
// горла (ключ кэша + query GET /contacts). Рендер хука в node-окружении
// недоступен — queryFn зовётся напрямую по собранному конфигу.
vi.mock('@/shared/api/client', () => ({ apiClient: vi.fn() }));

import { apiClient } from '@/shared/api/client';
import { CONTACTS_PAGE_SIZE, contactsListQuery } from './hooks';
import type { useContacts } from './hooks';

const apiClientMock = vi.mocked(apiClient);

/** Пустая страница /contacts: маппинг строк тест не интересует. */
const emptyPage = { items: [], nextCursor: null };

/** Поисковые параметры первого вызова apiClient. */
function requestedQuery(): URLSearchParams {
  const path = apiClientMock.mock.calls[0]?.[0];
  if (path === undefined) {
    throw new Error('apiClient не вызван');
  }
  return new URL(path, 'https://unit.test').searchParams;
}

describe('contactsListQuery — общее горло списка контактов (#600, #847)', () => {
  beforeEach(() => {
    apiClientMock.mockReset();
    apiClientMock.mockResolvedValue(emptyPage);
  });

  it('срез объекта sort=created/order=desc: desc — часть ключа, срез не смешивается с asc-кэшем (#847)', () => {
    const config = contactsListQuery({
      propertyId: 'prop-1',
      search: 'иван',
      sort: 'created',
      order: 'desc',
    });

    expect(config.queryKey).toStrictEqual([
      'contacts',
      'list',
      'prop-1',
      'иван',
      'created',
      'desc',
    ]);
  });

  it('queryFn уходит в GET /contacts?property_id=…&search=…&sort=created&order=desc&limit=50; asc/name не едут (#847)', async () => {
    const config = contactsListQuery({
      propertyId: 'prop-1',
      search: 'иван',
      sort: 'created',
      order: 'desc',
    });

    await config.queryFn({ pageParam: undefined });

    expect(apiClientMock).toHaveBeenCalledTimes(1);
    expect([...requestedQuery().entries()]).toStrictEqual([
      ['property_id', 'prop-1'],
      ['search', 'иван'],
      ['sort', 'created'],
      ['order', 'desc'],
      ['limit', String(CONTACTS_PAGE_SIZE)],
    ]);
  });

  it('keyset-продолжение: курсор прошлой порции едет параметром cursor', async () => {
    const config = contactsListQuery({
      propertyId: 'prop-1',
      search: '',
      sort: 'created',
      order: 'desc',
    });

    await config.queryFn({ pageParam: 'cursor-2' });

    expect(requestedQuery().get('cursor')).toBe('cursor-2');
  });

  it('дефолтная ось name/asc не меняет ни ключ, ни запрос — прежние вызовы читают прежний кэш', async () => {
    const config = contactsListQuery({
      propertyId: 'prop-1',
      search: '',
      sort: 'name',
      order: 'asc',
    });

    expect(config.queryKey).toStrictEqual([
      'contacts',
      'list',
      'prop-1',
      '',
      'name',
      'asc',
    ]);

    await config.queryFn({ pageParam: undefined });

    const query = requestedQuery();
    expect(query.has('sort')).toBe(false);
    expect(query.has('order')).toBe(false);
    expect(query.get('property_id')).toBe('prop-1');
    expect(query.get('limit')).toBe(String(CONTACTS_PAGE_SIZE));
  });

  it('опции useContacts несут order рядом с sort — проводка desc закреплена типами (#847)', () => {
    type ContactsOptions = NonNullable<Parameters<typeof useContacts>[2]>;
    // Отсутствие order в опциях падает присваиванием в tsc; сам конфиг
    // (ключ + wire) пинят тесты contactsListQuery выше.
    const pickerOptions: ContactsOptions = { sort: 'created', order: 'desc' };
    expect(pickerOptions).toStrictEqual({ sort: 'created', order: 'desc' });
  });
});
