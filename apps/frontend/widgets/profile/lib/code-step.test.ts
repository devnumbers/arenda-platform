import { describe, expect, it } from 'vitest';
import { ApiError } from '@/shared/api/errors';
import { codeFieldError, invalidCodeDetail, RESEND_COOLDOWN_MS } from './code-step';

describe('invalidCodeDetail', () => {
  it('401 verify-мутации — inline-текст поля, detail бэка («Неверный код»)', () => {
    const error = new ApiError('Unauthorized', 'Неверный код', undefined, 401);
    expect(invalidCodeDetail(error)).toBe('Неверный код');
  });

  it('остальные статусы — не inline (идут тостами сценариев)', () => {
    expect(invalidCodeDetail(new ApiError('TooManyRequests', 'Превышен лимит запросов', undefined, 429))).toBeNull();
    expect(invalidCodeDetail(new ApiError('Conflict', 'Эта электронная почта уже используется', undefined, 409))).toBeNull();
    expect(invalidCodeDetail(new ApiError('BadRequest', 'bad request', undefined, 400))).toBeNull();
  });

  it('статус неизвестен — не inline', () => {
    expect(invalidCodeDetail(new ApiError('Internal', 'boom'))).toBeNull();
  });
});

describe('RESEND_COOLDOWN_MS', () => {
  it('равен серверному троттлингу повторной отправки — 1 минута', () => {
    expect(RESEND_COOLDOWN_MS).toBe(60_000);
  });
});

describe('codeFieldError', () => {
  it('до сабмита ошибки нет даже при пустом поле', () => {
    expect(codeFieldError(false, '', null)).toBeUndefined();
  });

  it('после сабмита неполный код — «Введите 6-значный код»', () => {
    expect(codeFieldError(true, '123', null)).toBe('Введите 6-значный код');
  });

  it('после сабмита валидный код — ошибки нет', () => {
    expect(codeFieldError(true, '123456', null)).toBeUndefined();
  });

  it('inline 401 виден без сабмита', () => {
    expect(codeFieldError(false, '', 'Неверный код')).toBe('Неверный код');
  });

  it('валидация неполного кода сильнее inline', () => {
    expect(codeFieldError(true, '123', 'Неверный код')).toBe('Введите 6-значный код');
  });

  it('валидный код с inline — inline', () => {
    expect(codeFieldError(true, '123456', 'Неверный код')).toBe('Неверный код');
  });
});
