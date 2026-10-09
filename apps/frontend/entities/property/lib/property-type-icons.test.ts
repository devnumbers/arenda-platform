import { describe, expect, it } from 'vitest';

import {
  BoldBox,
  BoldBuild,
  BoldDoor,
  BoldFence,
  BoldGarage,
  BoldHome,
  BoldHouse,
  BoldLongSofa,
  BoldOfficeChair,
  BoldParking,
  BoldSofa,
} from '@/shared/assets/icons';

import { propertyTypeIcons } from './property-type-icons';

describe('реестр иконок типов объекта', () => {
  // Полнота покрытия типами домена гарантирует сам тип
  // Record<PropertyType, IconComponent>: новый тип не соберётся без иконки.
  // Маппинг — по вариантам компонента Category Icon (Figma
  // 3099:151373…151403, карта #1217): у каждого типа свой глиф.
  it('жилые типы — по глифу из фигмы', () => {
    expect(propertyTypeIcons.apartment).toBe(BoldHome);
    expect(propertyTypeIcons.studio).toBe(BoldSofa);
    expect(propertyTypeIcons.apartments).toBe(BoldLongSofa);
    expect(propertyTypeIcons.room).toBe(BoldDoor);
    expect(propertyTypeIcons.house).toBe(BoldHouse);
  });

  it('не-жилые типы — по глифу из фигмы', () => {
    expect(propertyTypeIcons.commercial).toBe(BoldBuild);
    expect(propertyTypeIcons.office).toBe(BoldOfficeChair);
    expect(propertyTypeIcons.warehouse).toBe(BoldBox);
    expect(propertyTypeIcons.garage).toBe(BoldGarage);
    expect(propertyTypeIcons.parking).toBe(BoldParking);
    expect(propertyTypeIcons.land).toBe(BoldFence);
  });

  it('все 11 типов различаются глифом', () => {
    const glyphs = Object.values(propertyTypeIcons);
    expect(new Set(glyphs).size).toBe(glyphs.length);
  });
});
