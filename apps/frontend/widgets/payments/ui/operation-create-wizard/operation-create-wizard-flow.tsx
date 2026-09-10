'use client';

import { useEffect, useRef, useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import {
  Button,
  IconButton,
  PageContent,
  SearchField,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
  useTabBarSuppression,
} from '@/shared/ui/design';
import type { PaymentOperation, PaymentType } from '@/entities/payment';
import {
  buildOperationCreateCommand,
  effectiveOperationType,
  initialOperationWizardStep,
  operationWizardStepReady,
  useCreateOperation,
  useOperationWizardDraft,
  type OperationWizardMode,
  type OperationWizardStep,
} from '@/features/payments';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import { useProperties } from '@/features/properties';
import { CategorySearchHint, WizardBottomBar, WizardHeading } from '../payment-create-wizard/wizard-chrome';
import { CategoryStep } from '../payment-create-wizard/category-step';
import { OperationAmountStep } from './operation-amount-step';
import { OperationTitleStep } from './operation-title-step';
import { OperationPropertyStep } from './operation-property-step';
import { OperationSuccess } from './operation-success';

/**
 * Поток шагов визарда создания операции (#570; Figma 1858:104557/105397,
 * 1863:67296, 1858:105011, 1858:105544): сумма и направление → название →
 * категория → объект (только глобальный вход — аннотация фрейма выбора) →
 * успех. Клиентское состояние на одном маршруте; черновик переживает
 * закрытие и перезагрузку (решение владельца 08.09), восстанавливается
 * на первый незавершённый шаг. Шаги без чипа — шапка несёт названия
 * шагов из макета; на последнем шаге кнопка сабмита «Добавить операцию»
 * (аннотация фрейма названия: у входа с объекта она живёт на категории).
 */

export type OperationCreateWizardFlowProps = {
  readonly mode: OperationWizardMode;
  /** Объект входа с объекта; у глобального входа выбирается на шаге 4. */
  readonly propertyId?: string;
  /** Пресет направления точки входа (Расходы→Расход, Доходы→Доход, иначе Расход). */
  readonly presetType: PaymentType;
  /** Имя объекта входа с объекта — подзаголовок экрана успеха. */
  readonly propertyName?: string;
};

export function OperationCreateWizardFlow({
  mode,
  propertyId,
  presetType,
  propertyName,
}: OperationCreateWizardFlowProps): JSX.Element {
  const router = useRouter();
  const createOperation = useCreateOperation();
  const { draft, setDraft, clearDraft } = useOperationWizardDraft();
  // Визард — экран создания: футер глушится на всех шагах (как у платежей).
  useTabBarSuppression();
  const [step, setStep] = useState<OperationWizardStep>(() =>
    initialOperationWizardStep(draft, mode),
  );
  const [created, setCreated] = useState<PaymentOperation | null>(null);
  // Поиск категорий живёт в хедере шага категории (как в визарде платежа):
  // лупа меняет название шага на поле, «Назад» возвращает название и
  // сбрасывает запрос.
  const [categorySearchOpen, setCategorySearchOpen] = useState(false);
  const [categoryQuery, setCategoryQuery] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  // Имя объекта для успеха глобального входа — из списка объектов,
  // загруженного шагом выбора (ответ POST имя не несёт); у входа с
  // объекта имя приходит из экрана, список не запрашивается.
  const propertiesQuery = useProperties({ enabled: mode === 'global' });

  // Открытие поиска сразу делает поле активным (программный фокус —
  // устоявшийся a11y-паттерн вместо autoFocus).
  useEffect(() => {
    if (categorySearchOpen) {
      searchInputRef.current?.focus();
    }
  }, [categorySearchOpen]);

  // Клик вне хедера при пустом запросе закрывает поиск (канон платежного
  // визарда); с непустым поиск остаётся открытым.
  useEffect(() => {
    if (!categorySearchOpen || categoryQuery !== '') {
      return undefined;
    }
    const handleOutside = (event: MouseEvent): void => {
      if (!(event.target instanceof Element) || !event.target.closest('header')) {
        setCategorySearchOpen(false);
      }
    };
    document.addEventListener('mousedown', handleOutside);
    return () => document.removeEventListener('mousedown', handleOutside);
  }, [categorySearchOpen, categoryQuery]);

  if (created !== null) {
    return (
      <>
        <TopNav
          leading={
            <IconButton icon={<Cancel />} label="Закрыть" onClick={closeAfterCreation} />
          }
        />
        <PageContent className="pt-0">
          <OperationSuccess
            created={created}
            propertyName={
              mode === 'property'
                ? (propertyName ?? '')
                : (createdPropertyName(created.propertyId) ?? '')
            }
          />
        </PageContent>
        <StickyBottomBar>
          <WizardBottomBar>
            <Button className="w-full" onClick={closeAfterCreation}>
              Готово
            </Button>
          </WizardBottomBar>
        </StickyBottomBar>
      </>
    );
  }

  return (
    <>
      <TopNav
        variant={step === 3 && categorySearchOpen ? 'search' : 'default'}
        leading={
          <IconButton
            icon={step === 1 ? <Cancel /> : <ArrowLeft />}
            label={step === 1 ? 'Закрыть' : 'Назад'}
            onClick={navigateBack}
          />
        }
        trailing={
          step === 3 && !categorySearchOpen ? (
            <IconButton
              icon={<Search />}
              label="Поиск по категориям"
              onClick={() => setCategorySearchOpen(true)}
            />
          ) : undefined
        }
      >
        {step === 3 && categorySearchOpen ? (
          <SearchField
            ref={searchInputRef}
            value={categoryQuery}
            onChange={(event) => setCategoryQuery(event.target.value)}
            onClear={() => {
              setCategoryQuery('');
              setCategorySearchOpen(false);
            }}
            placeholder="Найти категорию"
            aria-label="Поиск по названиям категорий"
          />
        ) : (
          <TopNavTitle title={HEADER_TITLES[step]} />
        )}
      </TopNav>
      <PageContent className="pt-0">
        {step === 1 && (
          <>
            <OperationAmountStep
              amountKopecks={draft.amountKopecks}
              onAmountChange={(amountKopecks) =>
                setDraft((prev) => ({ ...prev, amountKopecks }))
              }
              type={effectiveOperationType(draft.type, presetType)}
              onTypeChange={(type) => setDraft((prev) => ({ ...prev, type }))}
            />
            {/* Кнопка видна всегда, неактивна без суммы (Figma 1858:104557). */}
            <StickyBottomBar>
              <WizardBottomBar>
                <Button
                  className="w-full"
                  disabled={!operationWizardStepReady(1, draft)}
                  onClick={() => goToStep(2)}
                >
                  Продолжить
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === 2 && (
          <>
            <OperationTitleStep
              title={draft.title ?? ''}
              onTitleChange={(title) => setDraft((prev) => ({ ...prev, title }))}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                <Button className="w-full" onClick={() => goToStep(3)}>
                  Продолжить
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === 3 && (
          <>
            {/* Открытый поиск меняет контент (канон платежного визарда):
                пустой запрос — иллюстрация-подсказка вместо списка. */}
            {categorySearchOpen && categoryQuery === '' ? (
              <CategorySearchHint text="Начните искать категорию" />
            ) : (
              <>
                <WizardHeading title="Категория операции" subtitle="Выберите категорию" />
                <CategoryStep
                  selectedSlug={draft.categorySlug}
                  onSelect={(slug) => setDraft((prev) => ({ ...prev, categorySlug: slug }))}
                  query={categorySearchOpen ? categoryQuery : ''}
                />
              </>
            )}
            {/* Категория — последний шаг входа с объекта: на ней сабмит
                (аннотация Figma 1863:67305). У глобального входа —
                «Продолжить» к выбору объекта. Кнопка — после выбора
                категории (канон шага категории платежа). */}
            {draft.categorySlug !== undefined && (
              <StickyBottomBar>
                <WizardBottomBar>
                  {mode === 'property' ? (
                    <Button
                      className="w-full"
                      loading={createOperation.isPending}
                      onClick={() => void submit()}
                    >
                      Добавить операцию
                    </Button>
                  ) : (
                    <Button className="w-full" onClick={() => goToStep(4)}>
                      Продолжить
                    </Button>
                  )}
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === 4 && (
          <>
            <OperationPropertyStep
              selectedPropertyId={draft.propertyId}
              onSelect={(id) => setDraft((prev) => ({ ...prev, propertyId: id }))}
            />
            {/* Сабмит глобального входа: неактивен, пока объект не выбран. */}
            <StickyBottomBar>
              <WizardBottomBar>
                <Button
                  className="w-full"
                  disabled={!operationWizardStepReady(4, draft)}
                  loading={createOperation.isPending}
                  onClick={() => void submit()}
                >
                  Добавить операцию
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
      </PageContent>
    </>
  );

  function closeAfterCreation(): void {
    goBack(router, fallbackHref());
  }

  function navigateBack(): void {
    if (step === 3 && categorySearchOpen) {
      setCategorySearchOpen(false);
      setCategoryQuery('');
      return;
    }
    if (step > 1) {
      setStep((prev) => ((prev - 1) as OperationWizardStep));
      return;
    }
    goBack(router, fallbackHref());
  }

  function goToStep(next: OperationWizardStep): void {
    setStep(next);
  }

  function fallbackHref(): string {
    return mode === 'property' && propertyId !== undefined
      ? ROUTES.propertyOperations(propertyId)
      : ROUTES.operations;
  }

  /** Объект сабмита: у входа с объекта — маршрут, у глобального — выбор
   * шага 4. */
  function targetPropertyId(): string | undefined {
    return mode === 'property' ? propertyId : draft.propertyId;
  }

  function createdPropertyName(id: string): string | undefined {
    return propertiesQuery.data?.find((property) => property.id === id)?.name;
  }

  async function submit(): Promise<void> {
    const resolvedPropertyId = targetPropertyId();
    if (resolvedPropertyId === undefined) {
      return;
    }
    const command = buildOperationCreateCommand(draft, {
      propertyId: resolvedPropertyId,
      presetType,
      resolveTitle: (slug) => paymentCategoryBySlug(slug)?.label,
    });
    if (command === undefined) {
      return;
    }
    try {
      const operation = await createOperation.mutateAsync({ propertyId: resolvedPropertyId, command });
      clearDraft();
      notify.scenarios.payments.operationCreated();
      setCreated(operation);
    } catch (error) {
      notify.scenarios.payments.operationCreateError(error);
    }
  }
}

/** Названия шагов в шапке (Figma 1858:104557 «Добавить операцию»,
 * 1863:67296 «Операция», 1858:105038 «Выбрать объект»); у шага категории
 * название из макета платежного визарда адаптировано под операцию. */
const HEADER_TITLES: Record<OperationWizardStep, string> = {
  1: 'Добавить операцию',
  2: 'Операция',
  3: 'Категория',
  4: 'Выбрать объект',
};
