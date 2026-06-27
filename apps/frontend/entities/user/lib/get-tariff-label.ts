export const TARIFF_LABELS: Record<string, string> = {
  basic: 'Базовый',
  pro: 'Pro',
  business: 'Бизнес',
};

export function getTariffLabel(tariffName?: string | null): string {
  if (!tariffName) {
    return 'Без подписки';
  }
  return TARIFF_LABELS[tariffName] ?? tariffName;
}
