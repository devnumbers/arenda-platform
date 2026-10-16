import { describe, expect, it } from 'vitest';
import { isValidLoginCode, loginCodeFromInput } from './login-code';

describe('loginCodeFromInput', () => {
  it('оставляет только цифры', () => {
    expect(loginCodeFromInput('1a2b3c')).toBe('123');
  });

  it('режет длину до 6 цифр', () => {
    expect(loginCodeFromInput('1234567890')).toBe('123456');
  });
});

describe('isValidLoginCode', () => {
  it('ровно 6 цифр валидны', () => {
    expect(isValidLoginCode('123456')).toBe(true);
  });

  it('короткий код невалиден — «Продолжить» флоу остаётся disabled', () => {
    expect(isValidLoginCode('12345')).toBe(false);
  });

  it('пустой код невалиден', () => {
    expect(isValidLoginCode('')).toBe(false);
  });
});
