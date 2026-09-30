import { describe, expect, it } from 'vitest';
import { propertyTypeOptions } from './property-types';

describe('propertyTypeOptions', () => {
  it('одиннадцать типов реестра, студия в семье квартиры после апартаментов (#1003)', () => {
    expect(propertyTypeOptions).toHaveLength(11);
    const values = propertyTypeOptions.map((option) => option.value);
    expect(values).toContain('studio');
    expect(values.indexOf('studio')).toBe(values.indexOf('apartments') + 1);
    expect(propertyTypeOptions.find((option) => option.value === 'studio')?.label).toBe('Студия');
  });
});
