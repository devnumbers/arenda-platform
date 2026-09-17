import { describe, expect, it } from 'vitest';

import {
  participantsCountLabel,
  sharedPropertiesCountLabel,
} from './participants-counters';

describe('participantsCountLabel — счётчик карточки «Ваши участники»', () => {
  it('ноль — «Нет участников» (макет 1967-86441)', () => {
    expect(participantsCountLabel(0)).toBe('Нет участников');
  });

  it('единица — именительный падеж единственного числа', () => {
    expect(participantsCountLabel(1)).toBe('1 участник');
    expect(participantsCountLabel(21)).toBe('21 участник');
  });

  it('два–четыре — родительный падеж единственного числа', () => {
    expect(participantsCountLabel(2)).toBe('2 участника');
    expect(participantsCountLabel(4)).toBe('4 участника');
    expect(participantsCountLabel(22)).toBe('22 участника');
  });

  it('пять–двадцать — родительный падеж множественного числа', () => {
    expect(participantsCountLabel(5)).toBe('5 участников');
    expect(participantsCountLabel(11)).toBe('11 участников');
    expect(participantsCountLabel(12)).toBe('12 участников');
    expect(participantsCountLabel(111)).toBe('111 участников');
  });
});

describe('sharedPropertiesCountLabel — счётчик карточки «Объекты пользователей»', () => {
  it('ноль — «Нет объектов» (макет 1967-86441)', () => {
    expect(sharedPropertiesCountLabel(0)).toBe('Нет объектов');
  });

  it('единица — именительный падеж единственного числа', () => {
    expect(sharedPropertiesCountLabel(1)).toBe('1 объект');
    expect(sharedPropertiesCountLabel(21)).toBe('21 объект');
  });

  it('два–четыре — родительный падеж единственного числа', () => {
    expect(sharedPropertiesCountLabel(2)).toBe('2 объекта');
    expect(sharedPropertiesCountLabel(3)).toBe('3 объекта');
  });

  it('пять–двадцать — родительный падеж множественного числа', () => {
    expect(sharedPropertiesCountLabel(5)).toBe('5 объектов');
    expect(sharedPropertiesCountLabel(12)).toBe('12 объектов');
  });
});
