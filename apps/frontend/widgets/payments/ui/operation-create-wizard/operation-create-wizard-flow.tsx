'use client';

import { useEffect, useRef, useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel, Search } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { isoTodayInTimezone } from '@/shared/lib/calendar';
import { deviceTimezone, useMe } from '@/features/auth';
import {
  Button,
  CalendarDatePicker,
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
  operationWizardStepReady,
  useCreateOperation,
  type OperationWizardDraft,
  type OperationWizardMode,
  type OperationWizardStep,
} from '@/features/payments';
import { paymentCategoryBySlug } from '@/features/payment-categories';
import { useProperties } from '@/features/properties';
import {
  effectiveSelectedPropertyId,
  filterEditableProperties,
  propertiesByPrimaryOrder,
} from '@/entities/property';
import { CategorySearchHint, WizardBottomBar, WizardHeading } from '../payment-create-wizard/wizard-chrome';
import { CategoryStep } from '../payment-create-wizard/category-step';
import { PropertyStep } from '../payment-create-wizard/property-step';
import { OperationAmountStep } from './operation-amount-step';
import { OperationTitleStep } from './operation-title-step';
import { OperationSuccess } from './operation-success';

/**
 * Поток шагов визарда создания операции (#570; Figma 1858:104557/105397,
 * 1863:67296, 1858:105011, 1858:105544): сумма и направление → дата
 * («Выберите дату» #1394, макет 3431-78480 — экран целиком
 * CalendarDatePicker) → название → категория → объект (только глобальный
 * вход — аннотация фрейма выбора) → успех. Клиентское состояние на одном
 * маршруте, старт всегда с шага суммы: черновика у операции нет (карта
 * #1052, Q2=В) — уход со страницы, перезагрузка и закрытие дают чистый
 * лист. Шаги без чипа — шапка несёт названия шагов из макета; шаг даты —
 * самостоятельная полноэкранная поверхность пикера с её шапкой и
 * «Продолжить»; на последнем шаге кнопка сабмита «Добавить операцию»
 * (аннотация фрейма названия: у входа с объекта она живёт на категории).
 * «Сегодня» для шага даты — календарь профиля через Intl (ADR 0048):
 * сервер остаётся авторитетным гейтом — будущее отклоняется 400-й с
 * понятным тостом (operationCreateError читает problem+json).
 */

export type OperationCreateWizardFlowProps = {
  readonly mode: OperationWizardMode;
  /** Объект входа с объекта; у глобального входа выбирается на шаге 5. */
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
  // «Сегодня» календаря профиля для шага даты (#1394): профиль уже в
  // кеше — его читает провайдер пользователя шапки кабинета. Пока профиль
  // едет (холодный вход), пояс устройства — ближайшая догадка, дефолт
  // Москвы не путается с «профиль не пришёл»; авторитет «сегодня» в любом
  // случае сервер (будущее отклоняется 400-й).
  const { data: me } = useMe();
  const today = isoTodayInTimezone(me?.timezone ?? deviceTimezone());
  // Состояние шагов живёт только пока смонтирован поток (канон — в модели).
  const [draft, setDraft] = useState<OperationWizardDraft>({});
  // Визард — экран создания: футер глушится на всех шагах (как у платежей).
  useTabBarSuppression();
  const [step, setStep] = useState<OperationWizardStep>(1);
  const [created, setCreated] = useState<PaymentOperation | null>(null);
  // Поиск категорий живёт в хедере шага категории (как в визарде платежа):
  // лупа меняет название шага на поле, «Назад» возвращает название и
  // сбрасывает запрос.
  const [categorySearchOpen, setCategorySearchOpen] = useState(false);
  const [categoryQuery, setCategoryQuery] = useState('');
  const searchInputRef = useRef<HTMLInputElement | null>(null);
  // Имя объекта для успеха глобального входа — из списка объектов,
  // загруженного шагом выбора (ответ POST имя не несёт); у входа с
  // объекта имя приходит из экрана, список не запрашивается. Тот же
  // список даёт дефолт предвыбора шага 5.
  const propertiesQuery = useProperties({ enabled: mode === 'global' });
  const editableProperties = propertiesByPrimaryOrder(
    filterEditableProperties(propertiesQuery.data ?? []),
  );
  // Видимый на шаге и уходящий в сабмит объект: явный выбор пользователя
  // сильнее, до явного — дефолт (верхняя строка, канон «Основного
  // объекта»); в состоянии шагов хранится только явный выбор.
  const selectedPropertyId = effectiveSelectedPropertyId(
    draft.propertyId,
    editableProperties,
  );


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

  // Шаг даты (#1394) — экран целиком CalendarDatePicker (макет 3431-78480):
  // своя шапка «Выберите дату» и «Продолжить», дата обязательна (снять
  // нельзя), дефолт — предвыбранное «сегодня владельца», прошлое открыто
  // без пола, будущее закрыто. «Назад» пикера — возврат на шаг суммы.
  if (step === 2) {
    return (
      <CalendarDatePicker
        title="Выберите дату"
        confirmLabel="Продолжить"
        today={today}
        value={draft.date ?? null}
        required
        allowPast
        maxDate={today}
        onClose={() => goToStep(1)}
        onConfirm={(date) => {
          if (date === null) {
            return;
          }
          setDraft((prev) => ({ ...prev, date }));
          goToStep(3);
        }}
      />
    );
  }

  return (
    <>
      <TopNav
        variant={step === 4 && categorySearchOpen ? 'search' : 'default'}
        leading={
          <IconButton
            icon={step === 1 ? <Cancel /> : <ArrowLeft />}
            label={step === 1 ? 'Закрыть' : 'Назад'}
            onClick={navigateBack}
          />
        }
        trailing={
          step === 4 && !categorySearchOpen ? (
            <IconButton
              icon={<Search />}
              label="Поиск по категориям"
              onClick={() => setCategorySearchOpen(true)}
            />
          ) : undefined
        }
      >
        {step === 4 && categorySearchOpen ? (
          <SearchField
            focusOnMount
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
        {step === 3 && (
          <>
            <OperationTitleStep
              title={draft.title ?? ''}
              onTitleChange={(title) => setDraft((prev) => ({ ...prev, title }))}
            />
            <StickyBottomBar>
              <WizardBottomBar>
                <Button className="w-full" onClick={() => goToStep(4)}>
                  Продолжить
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
        {step === 4 && (
          <>
            {/* Открытый поиск меняет контент (канон платежного визарда
                #1152, решение владельца 13.10): заголовок — только при
                закрытом поиске; с открытым — пустой запрос даёт
                иллюстрацию-подсказку, запрос — отфильтрованный список без
                заголовка. */}
            {categorySearchOpen ? (
              categoryQuery === '' ? (
                <CategorySearchHint text="Начните искать категорию" />
              ) : (
                <CategoryStep
                  selectedSlug={draft.categorySlug}
                  onSelect={(slug) => setDraft((prev) => ({ ...prev, categorySlug: slug }))}
                  query={categoryQuery}
                />
              )
            ) : (
              <>
                {/* H1 как на шаге категории платежа #1152 (решение
                    владельца 13.10) — без подзаголовка. */}
                <WizardHeading title="Выберите категорию операции" variant="h1" />
                <CategoryStep
                  selectedSlug={draft.categorySlug}
                  onSelect={(slug) => setDraft((prev) => ({ ...prev, categorySlug: slug }))}
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
                    <Button className="w-full" onClick={() => goToStep(5)}>
                      Продолжить
                    </Button>
                  )}
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {step === 5 && (
          <>
            <PropertyStep
              selectedPropertyId={selectedPropertyId}
              onSelect={(id) => setDraft((prev) => ({ ...prev, propertyId: id }))}
              ariaLabel="Объект операции"
            />
            {/* Сабмит глобального входа: с дефолтом предвыбора активен
                сразу, гаснет только на пустом справочнике редактируемых. */}
            <StickyBottomBar>
              <WizardBottomBar>
                <Button
                  className="w-full"
                  disabled={
                    !operationWizardStepReady(5, {
                      ...draft,
                      propertyId: selectedPropertyId,
                    })
                  }
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
    if (step === 4 && categorySearchOpen) {
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

  /** Объект сабмита: у входа с объекта — маршрут, у глобального — видимый
   * выбор шага 5 (явный или дефолт). */
  function targetPropertyId(): string | undefined {
    return mode === 'property' ? propertyId : selectedPropertyId;
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
      notify.scenarios.payments.operationCreated();
      setCreated(operation);
    } catch (error) {
      notify.scenarios.payments.operationCreateError(error);
    }
  }
}

/** Названия шагов в шапке (Figma 1858:104557 «Добавить операцию»,
 * 1863:67296 «Операция», 1858:105038 «Выбрать объект»); у шага категории
 * название из макета платежного визарда адаптировано под операцию. Шага 2
 * в реестре нет — экран даты рисует CalendarDatePicker со своей шапкой
 * «Выберите дату» (#1394). */
const HEADER_TITLES: Record<Exclude<OperationWizardStep, 2>, string> = {
  1: 'Добавить операцию',
  3: 'Операция',
  4: 'Категория',
  5: 'Выбрать объект',
};
