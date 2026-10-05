import { describe, expect, it } from 'vitest';
import type { Participant } from '../model/types';
import {
  filterParticipantsByQuery,
  sortParticipantsByName,
} from './participant-list';

function participant(id: string, displayName: string | undefined, email: string | undefined): Participant {
  return {
    id,
    userId: displayName === undefined ? undefined : id,
    email,
    displayName,
    aggregateStatus: 'all_properties',
    accessiblePropertiesCount: 1,
    properties: [],
  };
}

const maria = participant('id-maria', 'Мария Петрова', 'maria@example.com');
const sergey = participant('id-sergey', 'Сергей Сидоров', 'sergey@mail.ru');
const pending = participant('invitee@example.com', undefined, 'invitee@example.com');
// Зарегистрированный без имени: display_name = полный телефон (канон
// #1105, собирает бекенд #1106).
const nameless = participant('id-nameless', '+79131234567', 'nameless@example.com');

describe('sortParticipantsByName — чип-сортировка «Имя»', () => {
  it('asc — русская коллация по титулу строки (у pending — почта)', () => {
    const sorted = sortParticipantsByName([sergey, pending, maria], 'asc');

    expect(sorted.map((p) => p.id)).toEqual(['id-maria', 'id-sergey', 'invitee@example.com']);
  });

  it('desc — обратный порядок, исходный список не мутируется', () => {
    const source = [maria, sergey];
    const sorted = sortParticipantsByName(source, 'desc');

    expect(sorted.map((p) => p.id)).toEqual(['id-sergey', 'id-maria']);
    expect(source.map((p) => p.id)).toEqual(['id-maria', 'id-sergey']);
  });

  it('регистр не важен («анна» до «Мария»)', () => {
    const anna = participant('id-anna', 'анна бирюкова', 'anna@example.com');
    const sorted = sortParticipantsByName([maria, anna], 'asc');

    expect(sorted[0]?.id).toBe('id-anna');
  });

  it('телефон-титул безымянного сортируется по общей коллации (канон #1105)', () => {
    // Цифры в русской коллации раньше букв — безымянные идут первыми,
    // ряд не ломает сортировку и не мутирует источник.
    const sorted = sortParticipantsByName([maria, nameless], 'asc');

    expect(sorted.map((p) => p.id)).toEqual(['id-nameless', 'id-maria']);
  });
});

describe('filterParticipantsByQuery — клиентский поиск (иконка в шапке, объём мал)', () => {
  it('пустой запрос (и из пробелов) возвращает весь список', () => {
    const list = [maria, sergey];

    expect(filterParticipantsByQuery(list, '')).toEqual(list);
    expect(filterParticipantsByQuery(list, '   ')).toEqual(list);
  });

  it('ищет подстрокой без регистра по имени', () => {
    expect(filterParticipantsByQuery([maria, sergey], 'петр')).toEqual([maria]);
    expect(filterParticipantsByQuery([maria, sergey], 'ПЕТРОВА')).toEqual([maria]);
  });

  it('ищет по почте — у зарегистрированных и у pending-строк', () => {
    expect(filterParticipantsByQuery([maria, sergey, pending], 'mail.ru')).toEqual([sergey]);
    expect(filterParticipantsByQuery([maria, sergey, pending], 'invitee@')).toEqual([pending]);
  });

  it('ищет по телефону безымянного — он титул строки (канон #1105)', () => {
    expect(filterParticipantsByQuery([maria, nameless], '913')).toEqual([nameless]);
    expect(filterParticipantsByQuery([maria, nameless], '+7913')).toEqual([nameless]);
  });

  it('без совпадений — пустой список', () => {
    expect(filterParticipantsByQuery([maria], 'александр')).toEqual([]);
  });
});
