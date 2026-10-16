'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { buildReturnUrl, goBack } from '@/shared/lib/navigation';
import {
  buildPropertyCreateCommand,
  propertyCreateStepReady,
  PROPERTY_CREATE_TOTAL_STEPS,
  useCreateProperty,
  type PropertyCreateDraft,
  type PropertyCreateStep,
} from '@/features/properties';
import { attributeCatalog } from '../property-fields/attribute-catalog';
import {
  Button,
  IconButton,
  PageContent,
  StepsChip,
  StickyBottomBar,
  TopNav,
  useTabBarSuppression,
} from '@/shared/ui/design';
import type { Property, PropertyType } from '@/entities/property';
import { AddressStep } from './address-step';
import { CategoryStep } from './category-step';
import { CharacteristicsStep } from './characteristics-step';
import { PropertyWizardBottomBar, PropertyWizardHeading } from './wizard-chrome';
import { WizardSuccess } from './wizard-success';

/**
 * Поток шагов визарда создания объекта (#480): клиентское состояние на
 * одном маршруте /properties/new, старт всегда с шага категории:
 * черновика у создания объекта нет (карта #1052, Q2=В) — уход со
 * страницы, перезагрузка и закрытие дают чистый лист. Хедер шагов
 * (Figma 1213-52111 / 1213-52017): шаг 1 — крестик слева; шаги 2–3 —
 * «назад» слева и крестик справа; в центре чип «шаг N из 3». Шаг 3
 * «Характеристики» (#482) — экран рендерится по каталогу и сабмитит
 * POST /properties; после успешного POST показывается экран успеха
 * (#483, Figma 1425-55788) — он шагом не считается. Хедер успеха —
 * только крестик слева (без чипа), он ведёт на карточку созданного
 * объекта; туда же ведёт кнопка «Открыть объект». Кнопка «Добавить
 * аренду» открывает визард создания аренды созданного объекта (карта
 * #984, push: возврат из визарда — на экран успеха).
 *
 * С заданным returnTo (#483, контракт возврата в вызывающий флоу) экран
 * успеха пропускается: визард заменяет запись истории на адрес returnTo,
 * дополненный параметром propertyId созданного объекта.
 */

const SUBMIT_ERROR_MESSAGE =
  'Не удалось создать объект. Проверьте соединение и попробуйте ещё раз';

export type PropertyCreateWizardFlowProps = {
  /** Санитизированный ?returnTo= маршрута: внутренний абсолютный путь.
   * Задан — успеха не показываем, после создания уводим обратно. */
  readonly returnTo?: string;
};

export function PropertyCreateWizardFlow({ returnTo }: PropertyCreateWizardFlowProps): JSX.Element {
  const router = useRouter();
  const createProperty = useCreateProperty();
  // Состояние шагов живёт только пока смонтирован поток (канон — в модели).
  const [draft, setDraft] = useState<PropertyCreateDraft>({});
  // Визард — экран создания: футер глушится на всех шагах, включая те,
  // где нижняя панель ещё не смонтирована (аналог визарда платежей #464).
  useTabBarSuppression();
  const [step, setStep] = useState<PropertyCreateStep>(1);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [created, setCreated] = useState<Property | null>(null);

  if (created !== null) {
    return (
      <>
        <TopNav
          leading={
            <IconButton icon={<Cancel />} label="Закрыть" onClick={openCreatedProperty} />
          }
        />
        <PageContent>
          <WizardSuccess
            created={created}
            onAddRental={() => router.push(ROUTES.propertyRentalNew(created.id))}
            onOpen={openCreatedProperty}
          />
        </PageContent>
      </>
    );
  }

  return (
    <>
      <TopNav
        leading={
          step === 1 ? (
            <IconButton icon={<Cancel />} label="Закрыть" onClick={dismiss} />
          ) : (
            <IconButton icon={<ArrowLeft />} label="Назад" onClick={navigateBack} />
          )
        }
        trailing={
          step > 1 ? <IconButton icon={<Cancel />} label="Закрыть" onClick={dismiss} /> : undefined
        }
      >
        <StepsChip step={step} total={PROPERTY_CREATE_TOTAL_STEPS} size="m" />
      </TopNav>
      {/* Шаг 1: контент прижат к низу области под хедером (Figma
          1213-52111: Page Content высотой с вьюпорт, alignItems flex-end,
          136px снизу; pb-[136px] PageContent — тот самый отступ). На
          десктопе — сверху, как принято для высоких окон (решение
          владельца). */}
      <PageContent
        className={
          step === 1
            ? 'min-h-[calc(100dvh-72px)] justify-end pt-0 tablet:min-h-0 tablet:justify-start'
            : 'pt-0'
        }
      >
        {step === 1 && (
          <>
            <PropertyWizardHeading title="Выберите, какая у вас недвижимость" />
            <CategoryStep
              selected={draft.type}
              onSelect={selectCategory}
            />
          </>
        )}
        {step === 2 && (
          <>
            <AddressStep
              value={draft.address ?? ''}
              onChange={(address) => setDraft((prev) => ({ ...prev, address }))}
            />
            {/* «Продолжить» — после непустого адреса (Figma 1213-52017
                без панели, 1213-52391 с панелью). */}
            {propertyCreateStepReady(2, draft) && (
              <StickyBottomBar>
                <PropertyWizardBottomBar>
                  <Button className="w-full" onClick={() => setStep(3)}>
                    Продолжить
                  </Button>
                </PropertyWizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}
        {/* Шаг 3 «Характеристики» (#482, Figma 1218-54295): название,
            тип жилья для «Квартиры», поля каталога по типу, описание;
            внизу «Создать объект» — POST /properties. Шаг достижим только
            с выбранной категорией, поэтому draft.type определён. */}
        {step === 3 && draft.type !== undefined && (
          <>
            <PropertyWizardHeading
              title="Характеристики"
              hint="Вы можете создать объект, а характеристики заполнить позже"
            />
            <CharacteristicsStep
              type={draft.type}
              onHousingTypeChange={(type) => setDraft((prev) => ({ ...prev, type }))}
              name={draft.name ?? ''}
              onNameChange={(name) => setDraft((prev) => ({ ...prev, name }))}
              description={draft.description ?? ''}
              onDescriptionChange={(description) => setDraft((prev) => ({ ...prev, description }))}
              attributes={draft.attributes ?? {}}
              onAttributesChange={(attributes) => setDraft((prev) => ({ ...prev, attributes }))}
            />
            <StickyBottomBar>
              <PropertyWizardBottomBar>
                {submitError !== null && (
                  <div
                    role="alert"
                    className="rounded-button bg-surface-danger px-4 py-3 text-sm leading-4 text-danger"
                  >
                    {submitError}
                  </div>
                )}
                <Button
                  className="w-full"
                  disabled={!propertyCreateStepReady(3, draft)}
                  loading={createProperty.isPending}
                  onClick={() => void submit()}
                >
                  Создать объект
                </Button>
              </PropertyWizardBottomBar>
            </StickyBottomBar>
          </>
        )}
      </PageContent>
    </>
  );

  function dismiss(): void {
    goBack(router, ROUTES.properties);
  }

  /** Карточка созданного объекта — заменой записи истории (правило
   * навигации): и крестик хедера успеха, и кнопка «Открыть объект». */
  function openCreatedProperty(): void {
    if (created === null) {
      return;
    }
    router.replace(ROUTES.property(created.id));
  }

  function navigateBack(): void {
    if (step > 1) {
      setStep((prev) => ((prev - 1) as PropertyCreateStep));
      return;
    }
    dismiss();
  }

  function selectCategory(type: PropertyType): void {
    setDraft((prev) => ({ ...prev, type }));
    setStep(2);
  }

  async function submit(): Promise<void> {
    setSubmitError(null);
    const command = buildPropertyCreateCommand(draft, attributeCatalog);
    if (command === undefined || createProperty.isPending) {
      return;
    }
    try {
      const property = await createProperty.mutateAsync(command);
      // Возврат в вызывающий флоу (например, будущего визарда операции):
      // успех пропускается, объект передаётся параметром propertyId.
      if (returnTo !== undefined) {
        router.replace(buildReturnUrl(returnTo, { propertyId: property.id }));
        return;
      }
      setCreated(property);
    } catch {
      setSubmitError(SUBMIT_ERROR_MESSAGE);
    }
  }
}
