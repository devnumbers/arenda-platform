import type { Metadata } from 'next';
import { OperationCreateWizardScreen } from '@/widgets/payments';
import { operationPresetFromQueryParam } from '@/features/payments';

/**
 * Визард создания одиночной операции — глобальный вход (#570): шаги
 * сумма → название → категория → «Выбрать объект» → успех. Направление —
 * пресет ?type= от точки входа (Расходы→Расход, Доходы→Доход, иначе
 * Расход), переключаемо на шаге суммы.
 */

export const metadata: Metadata = {
  title: 'Новая операция — Рентли',
};

export default async function OperationNewRoutePage({
  searchParams,
}: PageProps<'/operations/new'>) {
  const query = await searchParams;

  return (
    <OperationCreateWizardScreen
      mode="global"
      presetType={operationPresetFromQueryParam(query.type)}
    />
  );
}
