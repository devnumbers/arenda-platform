import { describe, expect, it } from 'vitest';
import { formatTimezoneLabel } from './timezone';

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
