import { describe, expect, it } from 'vitest';
import { getProfileDisplayName } from './profile-display';

describe('getProfileDisplayName', () => {
  it('склеивает имя и фамилию', () => {
    expect(getProfileDisplayName({ name: 'Даниил', surname: 'Козлов' })).toBe('Даниил Козлов');
  });

  it('оставляет только имя, когда фамилии нет', () => {
    expect(getProfileDisplayName({ name: 'Даниил', surname: null })).toBe('Даниил');
  });

  it('оставляет только фамилию, когда имени нет', () => {
    expect(getProfileDisplayName({ name: null, surname: 'Козлов' })).toBe('Козлов');
  });

  it('пустые строки считает отсутствующими', () => {
    expect(getProfileDisplayName({ name: '', surname: '' })).toBe('Пользователь');
  });

  it('без имени и фамилии возвращает «Пользователь»', () => {
    expect(getProfileDisplayName({ name: null, surname: null })).toBe('Пользователь');
  });
});
