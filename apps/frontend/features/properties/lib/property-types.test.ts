import { describe, expect, it } from 'vitest';
import { propertyAutonamePhrase, propertyTypeOptions } from './property-types';

describe('propertyTypeOptions', () => {
  it('одиннадцать типов реестра, студия в семье квартиры после апартаментов (#1003)', () => {
    expect(propertyTypeOptions).toHaveLength(11);
    const values = propertyTypeOptions.map((option) => option.value);
    expect(values).toContain('studio');
    expect(values.indexOf('studio')).toBe(values.indexOf('apartments') + 1);
    expect(propertyTypeOptions.find((option) => option.value === 'studio')?.label).toBe('Студия');
  });
});

describe('propertyAutonamePhrase', () => {
  it('фразы одиннадцати типов дословно как автонейм бэка (property_name.go, #1001)', () => {
    expect(propertyAutonamePhrase('apartment')).toBe('Моя квартира');
    expect(propertyAutonamePhrase('room')).toBe('Моя комната');
    expect(propertyAutonamePhrase('apartments')).toBe('Мои апартаменты');
    expect(propertyAutonamePhrase('studio')).toBe('Моя студия');
    expect(propertyAutonamePhrase('house')).toBe('Мой дом');
    expect(propertyAutonamePhrase('commercial')).toBe('Моё коммерческое помещение');
    expect(propertyAutonamePhrase('office')).toBe('Мой офис');
    expect(propertyAutonamePhrase('warehouse')).toBe('Мой склад');
    expect(propertyAutonamePhrase('garage')).toBe('Мой гараж');
    expect(propertyAutonamePhrase('parking')).toBe('Моё машиноместо');
    expect(propertyAutonamePhrase('land')).toBe('Мой земельный участок');
  });

  it('без типа — фолбэк бэка «Мой объект»', () => {
    expect(propertyAutonamePhrase(undefined)).toBe('Мой объект');
  });

  it('карта полна: фраза есть у каждого типа реестра, фолбэк никому не достался', () => {
    const phrases = propertyTypeOptions.map((option) => propertyAutonamePhrase(option.value));
    expect(phrases).toHaveLength(11);
    expect(phrases).not.toContain('Мой объект');
    expect(new Set(phrases).size).toBe(11);
  });
});
