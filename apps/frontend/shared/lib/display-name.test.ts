import { describe, expect, it } from 'vitest';

import {
  ANONYMOUS_USER_NAME,
  displayOwnUserName,
  displayUserName,
  OWN_USER_NAME_SUFFIX,
} from './display-name';

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

describe('displayOwnUserName — база своей строки (карта #1354, тикет #1356)', () => {
  it('своей строки фамилия не пишется (макет «Даниил (Вы)») — только имя', () => {
    expect(displayOwnUserName('Даниил')).toBe('Даниил');
  });

  it('пробелы вокруг имени срезаются', () => {
    expect(displayOwnUserName('  Даниил ')).toBe('Даниил');
  });

  it('имени нет — «Пользователь» (на виде сложится «Пользователь (Вы)»)', () => {
    expect(displayOwnUserName('')).toBe(ANONYMOUS_USER_NAME);
    expect(displayOwnUserName('  ')).toBe(ANONYMOUS_USER_NAME);
  });
});

describe('OWN_USER_NAME_SUFFIX — текст суффикса своей строки (#1356)', () => {
  it('« (Вы)» с ведущим пробелом — рисуется вплотную к имени', () => {
    expect(OWN_USER_NAME_SUFFIX).toBe(' (Вы)');
  });
});
