import { describe, expect, it } from 'vitest';

import { ANONYMOUS_USER_NAME, displayUserName } from './display-name';

describe('displayUserName — ФИО-канон D4 (карта #1324)', () => {
  it('обе части — «Имя Фамилия»', () => {
    expect(displayUserName('Иван', 'Иванов')).toBe('Иван Иванов');
  });

  it('недостающая часть просто не пишется — только имя', () => {
    expect(displayUserName('Иван', '')).toBe('Иван');
  });

  it('имени нет — «Пользователь», голая фамилия не показывается', () => {
    expect(displayUserName('', 'Иванов')).toBe(ANONYMOUS_USER_NAME);
    expect(displayUserName('', '')).toBe(ANONYMOUS_USER_NAME);
  });

  it('пробельные части считаются пустыми', () => {
    expect(displayUserName('  ', 'Иванов')).toBe(ANONYMOUS_USER_NAME);
    expect(displayUserName('Иван', '  ')).toBe('Иван');
  });
});
