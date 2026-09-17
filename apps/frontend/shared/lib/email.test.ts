import { describe, expect, it } from 'vitest';
import { isEmailValid } from './email';

describe('isEmailValid', () => {
  it('пустая почта невалидна — обязательность решает вызывающий код (контакты: «не указано», флоу смены #722: disabled «Продолжить»)', () => {
    expect(isEmailValid('')).toBe(false);
  });

  it('принимает корректный адрес', () => {
    expect(isEmailValid('daniil@yandex.ru')).toBe(true);
  });

  it('отклоняет адрес без домена', () => {
    expect(isEmailValid('daniil@')).toBe(false);
  });

  it('отклоняет адрес с пробелом', () => {
    expect(isEmailValid('da niil@yandex.ru')).toBe(false);
  });
});
