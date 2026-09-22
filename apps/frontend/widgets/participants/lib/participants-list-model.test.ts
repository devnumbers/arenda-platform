import { describe, expect, it } from 'vitest';
import {
  DEFAULT_PARTICIPANTS_LIST_ORDER,
  PARTICIPANTS_LIST_ORDER_PARAMS,
  parseParticipantsListOrderParams,
  serializeParticipantsListOrderToParams,
} from './participants-list-model';

describe('parseParticipantsListOrderParams — разбор ?order= «Ваших участников» (#785)', () => {
  it('отсутствие, пустое и неизвестное значение — дефолт «А→Я»', () => {
    expect(parseParticipantsListOrderParams(undefined)).toBe('asc');
    expect(parseParticipantsListOrderParams('')).toBe('asc');
    expect(parseParticipantsListOrderParams('по дате')).toBe('asc');
  });

  it('массивное значение — битый дубликат, дефолт (канон parseEnumParam, #786)', () => {
    expect(parseParticipantsListOrderParams(['desc'])).toBe('asc');
  });

  it('читает «Я→А»', () => {
    expect(parseParticipantsListOrderParams('desc')).toBe('desc');
  });

  it('дефолт — asc: сервер приходит name ASC (#697), «А→Я» в адресе не живёт', () => {
    expect(DEFAULT_PARTICIPANTS_LIST_ORDER).toBe('asc');
  });

  it('собственный параметр поверхности — только order', () => {
    expect(PARTICIPANTS_LIST_ORDER_PARAMS).toEqual(['order']);
  });
});

describe('serializeParticipantsListOrderToParams — патч ?order= для адреса (#785)', () => {
  it('дефолт «А→Я» параметров не создаёт — пустой query даёт голый адрес', () => {
    expect(serializeParticipantsListOrderToParams('asc')).toStrictEqual({});
  });

  it('«Я→А» пишет order=desc', () => {
    expect(serializeParticipantsListOrderToParams('desc')).toStrictEqual({ order: 'desc' });
  });
});
