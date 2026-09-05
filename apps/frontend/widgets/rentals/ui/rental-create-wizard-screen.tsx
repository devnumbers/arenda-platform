'use client';

import type { JSX } from 'react';
import { useProperty, canMutateProperty } from '@/features/properties';
import { useRentalWizardDraft } from '@/features/rentals';
import { Button, PageContent, Skeleton, TopNav } from '@/shared/ui/design';
import { RentalCreateWizardFlow } from './rental-create-wizard-flow';

/**
 * Экран визарда создания аренды (#530): маршрут /properties/[id]/rentals/new.
 * Черновик монтируется только после гидрации хранилища — до этого скелет;
 * мутационный вход закрыт для смотрящего и архива (создание — Full Access,
 * ADR 0053 §3; предикат общий с платежами и контактами).
 */

export type RentalCreateWizardScreenProps = {
  readonly propertyId: string;
};

export function RentalCreateWizardScreen({
  propertyId,
}: RentalCreateWizardScreenProps): JSX.Element {
  const propertyQuery = useProperty(propertyId);
  const draftState = useRentalWizardDraft(propertyId);

  const loading = !draftState.isLoaded || propertyQuery.isPending;
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const canMutate = property !== undefined && canMutateProperty(property);

  return (
    <>
      {loading && (
        <>
          <TopNav />
          <PageContent>
            <div className="flex flex-col gap-4 pt-6">
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
              <Skeleton className="h-14 w-full" />
            </div>
          </PageContent>
        </>
      )}

      {!loading && propertyQuery.isError && (
        <>
          <TopNav />
          <PageContent>
            <div className="flex flex-col items-center gap-4 pt-6">
              <p className="text-center text-base leading-[18px] text-content-secondary">
                Не удалось загрузить объект
              </p>
              <Button
                variant="secondary"
                size="small"
                onClick={() => void propertyQuery.refetch()}
              >
                Повторить
              </Button>
            </div>
          </PageContent>
        </>
      )}

      {!loading && !propertyQuery.isError && !canMutate && (
        <>
          <TopNav />
          <PageContent>
            <p className="pt-6 text-center text-base leading-[18px] text-content-secondary">
              У вас доступ только для просмотра этого объекта
            </p>
          </PageContent>
        </>
      )}

      {!loading && canMutate && (
        <RentalCreateWizardFlow key={propertyId} propertyId={propertyId} />
      )}
    </>
  );
}
