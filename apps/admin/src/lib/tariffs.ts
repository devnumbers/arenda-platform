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

/** Представление записи тарифа для react-admin: русское название, иначе #id. */
export const tariffRepresentation = (record: { id?: unknown; name?: unknown }): string => {
  const name = typeof record.name === 'string' && record.name !== '' ? tariffName(record.name) : '';
  return name || `#${record.id}`;
};

/** Поля тарифа, редактируемые админом (issue #256); цены — копейки. */
export interface TariffPricingInput {
  monthlyPriceKopecks: number;
  yearlyPriceKopecks: number;
  activePropertyLimit: number;
}

/**
 * Валидация редактируемых полей тарифа (issue #256), зеркально домену:
 * цены в копейках ≥ 0, лимит ≥ −1 (−1 = безлимит).
 * Возвращает русское сообщение об ошибке или null, если значения корректны.
 */
export const validateTariffPricing = (input: TariffPricingInput): string | null => {
  if (input.monthlyPriceKopecks < 0 || input.yearlyPriceKopecks < 0) {
    return 'Цены не могут быть отрицательными';
  }
  if (input.activePropertyLimit < unlimitedPropertyLimit) {
    return 'Лимит должен быть −1 (безлимит) или больше';
  }
  return null;
};

/**
 * Разбор значения числового поля диалога тарифа: целое число или null
 * (пустая строка, не число, дробное). Копейки и лимит — только целые.
 */
export const parseTariffNumber = (value: string): number | null => {
  const trimmed = value.trim();
  if (trimmed === '' || !/^-?\d+$/.test(trimmed)) {
    return null;
  }
  return Number(trimmed);
};
