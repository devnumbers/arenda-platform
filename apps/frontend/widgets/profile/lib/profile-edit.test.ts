import { describe, expect, it } from 'vitest';
import type { User } from '@/entities/user';
import { profileFieldPatch } from './profile-edit';

const me: User = {
  id: 'u1',
  phone: '79990000000',
  role: 'owner',
  name: 'Даниил',
  surname: null,
  patronymic: null,
  email: 'daniil@yandex.ru',
  timezone: 'Europe/Moscow',
  photoUrl: null,
  subscription: null,
};

describe('profileFieldPatch', () => {
  it('неизменённое поле не даёт патча', () => {
    expect(profileFieldPatch(me, 'name', 'Даниил')).toBeNull();
  });

  it('обрезает пробелы по краям', () => {
    expect(profileFieldPatch(me, 'name', '  Иван  ')).toEqual({ name: 'Иван' });
  });

  it('пустая строка очищает поле — уходит "", а не null (контракт бэка: null = «не менять»)', () => {
    expect(profileFieldPatch(me, 'name', '')).toEqual({ name: '' });
  });

  it('пробельная строка при пустом сохранённом значении не даёт патча', () => {
    expect(profileFieldPatch(me, 'surname', '   ')).toBeNull();
  });
});
