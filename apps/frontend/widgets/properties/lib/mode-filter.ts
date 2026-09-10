/** Режим списочного экрана объектов: активная книга или архив (#586). */
export type PropertiesViewMode = 'active' | 'archived';

/** Срез списка по режиму: активные без архивных, архив — только архивные. */
export function filterPropertiesByMode<T extends { readonly status: string }>(
  items: readonly T[],
  mode: PropertiesViewMode,
): T[] {
  return items.filter((property) =>
    mode === 'archived' ? property.status === 'archived' : property.status !== 'archived',
  );
}
