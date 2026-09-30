import { describe, expect, it } from 'vitest';

import { propertyTypeIcons } from './property-type-icons';

describe('реестр иконок типов объекта', () => {
  // Полнота покрытия типами домена гарантирует сам тип
  // Record<PropertyType, IconComponent>: новый тип не соберётся без иконки.
  it('квартира — bold-home (мок 1425-55788: Icon/Bold/Home); дом и апартаменты делят её с квартирой', () => {
    expect(propertyTypeIcons.apartments).toBe(propertyTypeIcons.apartment);
    expect(propertyTypeIcons.house).toBe(propertyTypeIcons.apartment);
  });

  it('студия — диван, как у комнаты (спальное место, #1003)', () => {
    expect(propertyTypeIcons.studio).toBe(propertyTypeIcons.room);
  });

  it('остальные типы различаются: у каждого не-жилого типа своя иконка', () => {
    const distinct = [
      propertyTypeIcons.commercial,
      propertyTypeIcons.office,
      propertyTypeIcons.warehouse,
      propertyTypeIcons.garage,
      propertyTypeIcons.parking,
      propertyTypeIcons.land,
    ];
    expect(new Set(distinct).size).toBe(distinct.length);
  });
});
