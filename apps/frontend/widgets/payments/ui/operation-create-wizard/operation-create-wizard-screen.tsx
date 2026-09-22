'use client';

import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { useProperty } from '@/features/properties';
import { propertyPermissions } from '@/entities/property';
import {
  useOperationWizardDraft,
  type OperationWizardMode,
} from '@/features/payments';
import type { PaymentType } from '@/entities/payment';
import {
  Button,
  IconButton,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  PaymentsEmptyCard,
  PaymentsStateCard,
} from '../payments-sections';
import { OperationAmountStepSkeleton } from '../payments-skeletons';
import { WizardBottomBar } from '../payment-create-wizard/wizard-chrome';
import { OperationCreateWizardFlow } from './operation-create-wizard-flow';

/**
 * Экран визарда создания одиночной операции (#570): маршруты
 * /operations/new (глобальный, с шагом «Выбрать объект») и
 * /properties/[id]/operations/new (с объекта, объектного шага нет).
 * Направление приходит пресетом точки входа (?type=). Черновик
 * монтируется только после гидрации хранилища; у входа с объекта
 * мутационный вход закрыт для смотрящего и архива (read-only, как у
 * визарда платежа).
 *
 * Загрузка (#607, паритет §7): холодный вход открывает шаг «Добавить
 * операцию» — хром шага («Закрыть», название) и нижняя панель
 * «Продолжить» рендерятся сразу, скелетон закрывает только поле суммы
 * с сегментом направления.
 */

export type OperationCreateWizardScreenProps = {
  readonly mode: OperationWizardMode;
  readonly propertyId?: string;
  readonly presetType: PaymentType;
};

export function OperationCreateWizardScreen({
  mode,
  propertyId,
  presetType,
}: OperationCreateWizardScreenProps): JSX.Element {
  const router = useRouter();
  // У глобального входа объект не читается: пустой id глушит запрос.
  const propertyQuery = useProperty(propertyId ?? '');
  const draftState = useOperationWizardDraft();

  const loading =
    !draftState.isLoaded || (mode === 'property' && propertyQuery.isPending);
  const property = propertyQuery.isSuccess ? propertyQuery.data : undefined;
  // Глобальный вход мутирует книгу читателя в целом; объектный — по
  // центральным правам объекта (#703).
  const canMutate = mode === 'global' || propertyPermissions(property).canEdit;

  // Выход с шага 1 — куда ведет «Закрыть» потока (шаг восстановится сам).
  const closeDestination =
    mode === 'property' && propertyId !== undefined
      ? ROUTES.propertyOperations(propertyId)
      : ROUTES.operations;

  return (
    <>
      {loading && (
        <>
          <TopNav
            leading={
              <IconButton
                icon={<Cancel />}
                label="Закрыть"
                onClick={() => goBack(router, closeDestination)}
              />
            }
          >
            <TopNavTitle title="Добавить операцию" />
          </TopNav>
          <PageContent className="pt-0">
            <OperationAmountStepSkeleton />
          </PageContent>
          {/* Панель «Продолжить» видна на шаге всегда — в загрузке та же
              кнопка в покое, поток подменяет её без сдвига. */}
          <StickyBottomBar>
            <WizardBottomBar>
              <Button className="w-full" disabled>
                Продолжить
              </Button>
            </WizardBottomBar>
          </StickyBottomBar>
        </>
      )}

      {!loading && mode === 'property' && propertyQuery.isError && (
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

      {!loading
        && mode === 'property'
        && !propertyQuery.isError
        && !canMutate && (
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

      {!loading && !propertyQuery.isError && canMutate && (
        <OperationCreateWizardFlow
          key={`${mode}:${propertyId ?? 'global'}`}
          mode={mode}
          propertyId={propertyId}
          presetType={presetType}
          propertyName={property?.name}
        />
      )}
    </>
  );
}
