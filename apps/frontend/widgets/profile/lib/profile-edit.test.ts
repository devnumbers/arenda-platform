import { describe, expect, it } from 'vitest';
import type { User } from '@/entities/user';
import { isEmailValid, profileFieldPatch } from './profile-edit';

const me: User = {
  id: 'u1',
  phone: '79990000000',
  role: 'owner',
  name: 'Даниил',
  surname: null,
  patronymic: null,
  email: 'daniil@yandex.ru',
  timezone: 'Europe/Moscow',
  subscription: null,
};

describe('isEmailValid', () => {
  it('пустая почта валидна (поле необязательное)', () => {
    expect(isEmailValid('')).toBe(true);
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

describe('profileFieldPatch', () => {
  it('неизменённое поле не даёт патча', () => {
    expect(profileFieldPatch(me, 'name', 'Даниил')).toBeNull();
  });

  it('обрезает пробелы по краям', () => {
    expect(profileFieldPatch(me, 'name', '  Иван  ')).toEqual({ name: 'Иван' });
  });

  it('пустая строка превращается в null', () => {
    expect(profileFieldPatch(me, 'name', '')).toEqual({ name: null });
  });

  it('пробельная строка при пустом сохранённом значении не даёт патча', () => {
    expect(profileFieldPatch(me, 'surname', '   ')).toBeNull();
  });

  it('изменённая почта даёт патч email', () => {
    expect(profileFieldPatch(me, 'email', 'new@mail.ru')).toEqual({
      email: 'new@mail.ru',
    });
  });

  it('очищенная почта даёт патч email: null', () => {
    expect(profileFieldPatch(me, 'email', '')).toEqual({ email: null });
  });
});
