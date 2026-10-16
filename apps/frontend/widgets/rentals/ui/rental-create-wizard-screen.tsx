'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import { WIZARD_TOTAL_STEPS } from '@/features/rentals';
import { Button, IconButton, PageContent, StepsChip, TopNav } from '@/shared/ui/design';
import { RentalAmountDayStepSkeleton } from './rental-skeletons';
import { WizardHeading } from './wizard-chrome';
import { RentalCreateWizardFlow } from './rental-create-wizard-flow';

/**
 * Экран визарда создания аренды (#530): маршрут /properties/[id]/rentals/new.
 * Поток монтируется после загрузки объекта — до этого скелет; состояние
 * шагов живёт в носителе сессии (rental-wizard-session), он синхронен и
 * гидрации не требует. Мутационный вход закрыт для смотрящего и архива
 * (создание — Full Access, ADR 0053 §3; предикат общий с платежами и
 * контактами).
 *
 * Загрузка (#607, паритет §7): холодный вход открывает шаг 1 «Цена и число
 * оплаты» — хром шага (крестик, чип шага) и заголовок рендерятся сразу,
 * скелетон закрывает только поля; кнопка шага скрыта до готовности, в
 * загрузке её тоже нет.
 */

export type RentalCreateWizardScreenProps = {
  readonly propertyId: string;
};

export function RentalCreateWizardScreen({
  propertyId,
}: RentalCreateWizardScreenProps): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);

  const loading = propertyQuery.isPending;
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const canMutate = property !== undefined && propertyPermissions(property).canEdit;

  return (
    <>
      {loading && (
        <>
          <TopNav
            leading={
              <IconButton
                icon={<Cancel />}
                label="Закрыть"
                onClick={() => goBack(router, ROUTES.property(propertyId))}
              />
            }
          >
            <StepsChip step={1} total={WIZARD_TOTAL_STEPS} size="m" />
          </TopNav>
          <PageContent className="pt-0">
            <WizardHeading title="Цена и число оплаты" />
            <RentalAmountDayStepSkeleton />
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
