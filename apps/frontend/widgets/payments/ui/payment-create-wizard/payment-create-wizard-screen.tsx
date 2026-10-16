'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import {
  usePaymentWizardDraft,
  WIZARD_TOTAL_STEPS,
  type PaymentDraftType,
} from '@/features/payments';
import {
  Button,
  IconButton,
  PageContent,
  StepsChip,
  TopNav,
} from '@/shared/ui/design';
import {
  PaymentsEmptyCard,
  PaymentsStateCard,
} from '../payments-sections';
import { CategoryRowsSkeleton } from '../payments-skeletons';
import { WizardHeading } from './wizard-chrome';
import { PaymentCreateWizardFlow } from './payment-create-wizard-flow';

/**
 * Экран визарда создания платежа (#464): маршрут /properties/[id]/payments/new,
 * тип «Платёж / Автоплатёж» приходит query-параметром (шит выбора #463).
 * Черновик монтируется только после гидрации хранилища, поэтому экран
 * показывает загрузку, а мутационный вход закрыт для смотрящего и архива
 * (read-only, история 47 спеки #453).
 *
 * Загрузка (#607, паритет §7): холодный вход открывает шаг 1 «Категория
 * платежа» — хром шага (Назад, лупа, чип шага) и заголовок рендерятся
 * сразу, скелетон закрывает только список категорий.
 */

export type PaymentCreateWizardScreenProps = {
  readonly propertyId: string;
  readonly draftType: PaymentDraftType;
};

export function PaymentCreateWizardScreen({
  propertyId,
  draftType,
}: PaymentCreateWizardScreenProps): JSX.Element {
  const router = useRouter();
  const propertyQuery = useProperty(propertyId);
  const draftState = usePaymentWizardDraft(propertyId, draftType);

  const loading = !draftState.isLoaded || propertyQuery.isPending;
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  const canMutate = propertyPermissions(property).canEdit;

  return (
    <>
      {loading && (
        <>
          <TopNav
            leading={
              <IconButton
                icon={<ArrowLeft />}
                label="Назад"
                onClick={() => goBack(router, ROUTES.propertyPayments(propertyId))}
              />
            }
            trailing={
              <IconButton icon={<Search />} label="Поиск по категориям" disabled />
            }
          >
            <StepsChip step={1} total={WIZARD_TOTAL_STEPS} size="m" />
          </TopNav>
          <PageContent className="pt-0">
            <WizardHeading title="Выберите категорию платежа" variant="h1" />
            <CategoryRowsSkeleton />
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
