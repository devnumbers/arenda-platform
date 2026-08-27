'use client';

import type { JSX } from 'react';
import { useProperty } from '@/features/properties';
import { usePaymentWizardDraft, type PaymentDraftType } from '@/features/payments';
import { Button, PageContent, TopNav } from '@/shared/ui/design';
import {
  PaymentsEmptyCard,
  PaymentsSkeleton,
  PaymentsStateCard,
} from '../payments-sections';
import { PaymentCreateWizardFlow } from './payment-create-wizard-flow';

/**
 * Экран визарда создания платежа (#464): маршрут /properties/[id]/payments/new,
 * тип «Платёж / Автоплатёж» приходит query-параметром (шит выбора #463).
 * Черновик монтируется только после гидрации хранилища, поэтому экран
 * показывает скелет, а мутационный вход закрыт для смотрящего и архива
 * (read-only, история 47 спеки #453).
 */

export type PaymentCreateWizardScreenProps = {
  readonly propertyId: string;
  readonly draftType: PaymentDraftType;
};

export function PaymentCreateWizardScreen({
  propertyId,
  draftType,
}: PaymentCreateWizardScreenProps): JSX.Element {
  const propertyQuery = useProperty(propertyId);
  const draftState = usePaymentWizardDraft(propertyId, draftType);

  const loading = !draftState.isLoaded || propertyQuery.isPending;
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const role = property?.access?.role;
  const canMutate =
    property !== undefined && role !== undefined && role !== 'viewer' && property.status !== 'archived';

  return (
    <>
      {loading && (
        <>
          <TopNav />
          <PageContent>
            <div className="flex flex-col gap-4 pt-6">
              <PaymentsSkeleton />
              <PaymentsSkeleton />
            </div>
          </PageContent>
        </>
      )}

      {!loading && propertyQuery.isError && (
        <>
          <TopNav />
          <PageContent>
            <div className="pt-6">
              <PaymentsStateCard
                title="Не удалось загрузить объект"
                hint="Проверьте подключение и попробуйте снова"
                action={
                  <Button
                    variant="secondary"
                    size="small"
                    onClick={() => void propertyQuery.refetch()}
                  >
                    Повторить
                  </Button>
                }
              />
            </div>
          </PageContent>
        </>
      )}

      {!loading && !propertyQuery.isError && !canMutate && (
        <>
          <TopNav />
          <PageContent>
            <div className="pt-6">
              <PaymentsEmptyCard
                title="Создание недоступно"
                hint="У вас доступ только для просмотра этого объекта"
              />
            </div>
          </PageContent>
        </>
      )}

      {!loading && canMutate && (
        <PaymentCreateWizardFlow
          key={`${propertyId}:${draftType}`}
          propertyId={propertyId}
          draftType={draftType}
        />
      )}
    </>
  );
}
