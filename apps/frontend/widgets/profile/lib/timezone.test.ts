import { describe, expect, it } from 'vitest';
import { filterPickerOptions } from '@/shared/ui/design/picker-filter';
import { formatTimezoneLabel, timezoneOptions } from './timezone';

describe('timezoneOptions', () => {
  it('полный набор зон территории РФ (UTC+2…UTC+12)', () => {
    expect(timezoneOptions).toHaveLength(22);
    expect(timezoneOptions.map((option) => option.value)).toEqual([
      'Europe/Kaliningrad',
      'Europe/Moscow',
      'Europe/Samara',
      'Europe/Saratov',
      'Asia/Yekaterinburg',
      'Asia/Omsk',
      'Asia/Novosibirsk',
      'Asia/Barnaul',
      'Asia/Tomsk',
      'Asia/Novokuznetsk',
      'Asia/Krasnoyarsk',
      'Asia/Irkutsk',
      'Asia/Chita',
      'Asia/Yakutsk',
      'Asia/Khandyga',
      'Asia/Vladivostok',
      'Asia/Ust-Nera',
      'Asia/Magadan',
      'Asia/Sakhalin',
      'Asia/Srednekolymsk',
      'Asia/Kamchatka',
      'Asia/Anadyr',
    ]);
  });

  it('отсортированы с запада на восток по смещению', () => {
    const offsets = timezoneOptions.map((option) => {
      const match = /UTC([+-]\d+)\)$/.exec(option.label);
      if (match === null) {
        throw new Error(`неожиданная подпись: ${option.label}`);
      }
      return Number(match[1]);
    });
    const sorted = [...offsets].sort((a, b) => a - b);
    expect(offsets).toEqual(sorted);
    expect(offsets[0]).toBe(2);
    expect(offsets[offsets.length - 1]).toBe(12);
  });

  it('значения уникальны, подписи в формате «Город (UTC±N)»', () => {
    const values = timezoneOptions.map((option) => option.value);
    expect(new Set(values).size).toBe(values.length);
    for (const option of timezoneOptions) {
      expect(option.label).toMatch(/^.+ \(UTC[+-]\d+\)$/);
    }
  });
});

describe('поиск по справочнику (канон filterPickerOptions)', () => {
  it('пустой запрос возвращает весь список', () => {
    expect(filterPickerOptions(timezoneOptions, '   ')).toHaveLength(timezoneOptions.length);
  });

  it('ищет по городу без учёта регистра', () => {
    const visible = filterPickerOptions(timezoneOptions, 'москв');
    expect(visible).toHaveLength(1);
    expect(visible[0]?.value).toBe('Europe/Moscow');
  });

  it('ищет по смещению', () => {
    const visible = filterPickerOptions(timezoneOptions, 'utc+3');
    expect(visible.map((option) => option.value)).toEqual(['Europe/Moscow']);
    expect(filterPickerOptions(timezoneOptions, '+12')).toHaveLength(2);
  });
});

describe('formatTimezoneLabel', () => {
  it('зона из справочника — подпись «Город (UTC±N)»', () => {
    expect(formatTimezoneLabel('Europe/Moscow')).toBe('Москва (UTC+3)');
    expect(formatTimezoneLabel('Asia/Novosibirsk')).toBe('Новосибирск (UTC+7)');
  });

  it('зона вне справочника — сырой IANA-идентификатор', () => {
    expect(formatTimezoneLabel('Asia/Tbilisi')).toBe('Asia/Tbilisi');
  });

  it('пустое значение — пустая строка', () => {
    expect(formatTimezoneLabel(null)).toBe('');
    expect(formatTimezoneLabel('')).toBe('');
    expect(formatTimezoneLabel(undefined)).toBe('');
  });
});
