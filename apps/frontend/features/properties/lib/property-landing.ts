import { ROUTES } from '@/shared/config/routes';
import type { Property } from '@/entities/property';

/**
 * Лендинг таба «Объекты» (карта #583, решение гриллинга 10.09): есть
 * основной объект — таб ведёт на его страницу; основного нет, но активный
 * объект один — сразу на его страницу; иначе — на список. Данные списка
 * ещё не загружены — безопасный фолбэк, список.
 */
export function resolvePropertiesLandingHref(
  properties: readonly Property[] | undefined,
): string {
  if (!properties) return ROUTES.properties;

  const active = properties.filter((property) => property.status !== 'archived');

  const primaries = active.filter(
    (property): property is Property & { readonly pinned_at: string } =>
      property.pinned_at !== null,
  );
  primaries.sort((a, b) => (a.pinned_at < b.pinned_at ? -1 : a.pinned_at > b.pinned_at ? 1 : 0));

  const [firstPrimary] = primaries;
  if (firstPrimary) return ROUTES.property(firstPrimary.id);
  if (active.length === 1) {
    const [only] = active;
    if (only) return ROUTES.property(only.id);
  }
  return ROUTES.properties;
}
