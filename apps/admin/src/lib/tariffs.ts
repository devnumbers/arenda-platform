// Чистая логика отображения тарифов для экрана «Тарифы» (issue #247).
// Значения name синхронизированы с enum TariffName
// apps/backend/api/openapi/openapi.yaml (basic, pro, business).

export type TariffName = 'basic' | 'pro' | 'business';

export const tariffNameChoices: { id: TariffName; name: string }[] = [
  { id: 'basic', name: 'Базовый' },
  { id: 'pro', name: 'Про' },
  { id: 'business', name: 'Бизнес' },
];

/** Русская подпись названия тарифа; неизвестное значение — как есть. */
export const tariffName = (name: string): string =>
  tariffNameChoices.find((choice) => choice.id === name)?.name ?? name;

/** Безлимитный лимит активных объектов (domain.UnlimitedPropertyLimit). */
export const unlimitedPropertyLimit = -1;

/** Лимит активных объектов: -1 → «Безлимит», иначе число. */
export const formatPropertyLimit = (limit: number): string =>
  limit === unlimitedPropertyLimit ? 'Безлимит' : String(limit);
