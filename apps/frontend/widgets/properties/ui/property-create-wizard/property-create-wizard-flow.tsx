'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { fieldsForType, toWireAttributes, validateAttributes } from '@/features/property-attributes';
import {
  attributeTypeChangeNotice,
  buildPropertyCreateCommand,
  initialPropertyCreateStep,
  propertyCreateStepReady,
  PROPERTY_CREATE_TOTAL_STEPS,
  useCreateProperty,
  usePropertyCreateDraft,
  type PropertyAttributesPort,
  type PropertyCreateStep,
} from '@/features/properties';
import {
  Button,
  IconButton,
  PageContent,
  StepsChip,
  StickyBottomBar,
  TopNav,
  useTabBarSuppression,
} from '@/shared/ui/design';
import type { PropertyType } from '@/entities/property';
import { AddressStep } from './address-step';
import { CategoryStep } from './category-step';
import { CharacteristicsStep } from './characteristics-step';
import { PropertyWizardBottomBar, PropertyWizardHeading } from './wizard-chrome';

/**
 * Поток шагов визарда создания объекта (#480): клиентское состояние на
 * одном маршруте /properties/new, восстанавливается на первый
 * незавершённый шаг черновика. Хедер шагов (Figma 1213-52111 /
 * 1213-52017): шаг 1 — крестик слева; шаги 2–3 — «назад» слева и крестик
 * справа; в центре чип «шаг N из 3». Шаг 3 «Характеристики» (#482) —
 * экран рендерится по каталогу и сабмитит POST /properties; успех ведёт
 * на карточку объекта (экран успеха — #483). Экран успеха шагом не
 * считается.
 */

// Порт каталога характеристик для сабмит-либы: реальные реализации
// соседней фичи инжектятся здесь (виджету доступны обе фичи).
const attributeCatalog: PropertyAttributesPort = {
  toWireAttributes,
  validateAttributes,
  fieldKeys: (type) => new Set(fieldsForType(type).map((field) => field.key)),
};

const SUBMIT_ERROR_MESSAGE =
  'Не удалось создать объект. Проверьте соединение и попробуйте ещё раз';

export function PropertyCreateWizardFlow(): JSX.Element {
  const router = useRouter();
  const createProperty = useCreateProperty();
  const { draft, setDraft, clearDraft } = usePropertyCreateDraft();
  // Визард — экран создания: футер глушится на всех шагах, включая те,
  // где нижняя панель ещё не смонтирована (аналог визарда платежей #464).
  useTabBarSuppression();
  const [step, setStep] = useState<PropertyCreateStep>(() => initialPropertyCreateStep(draft));
  // Нотис lossless живёт во флоу: смена типа случается и на шаге 1
  // (возврат «назад» и другая категория), а шаг 3 перемонтируется.
  const [typeChangeNotice, setTypeChangeNotice] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);

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
            ? 'min-h-[calc(100dvh-72px)] justify-end pt-0 desktop:min-h-0 desktop:justify-start'
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
              onHousingTypeChange={changeType}
              name={draft.name ?? ''}
              onNameChange={(name) => setDraft((prev) => ({ ...prev, name }))}
              description={draft.description ?? ''}
              onDescriptionChange={(description) => setDraft((prev) => ({ ...prev, description }))}
              attributes={draft.attributes ?? {}}
              onAttributesChange={(attributes) => setDraft((prev) => ({ ...prev, attributes }))}
              notice={typeChangeNotice ?? undefined}
              onDismissNotice={() => setTypeChangeNotice(null)}
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

  function navigateBack(): void {
    if (step > 1) {
      setStep((prev) => ((prev - 1) as PropertyCreateStep));
      return;
    }
    dismiss();
  }

  /** Смена типа с заполненными характеристиками прежнего типа показывает
   * нотис (lossless); без чужих заполненных ключей смена тихая. */
  function changeType(nextType: PropertyType): void {
    if (draft.type !== undefined) {
      const notice = attributeTypeChangeNotice(
        draft.type,
        nextType,
        draft.attributes ?? {},
        attributeCatalog,
      );
      setTypeChangeNotice(notice ?? null);
    }
    setDraft((prev) => ({ ...prev, type: nextType }));
  }

  function selectCategory(type: PropertyType): void {
    changeType(type);
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
      clearDraft();
      // Создание сущности ведёт на её страницу заменой записи истории
      // (правило навигации); экран успеха заменит этот переход (#483).
      router.replace(ROUTES.property(property.id));
    } catch {
      setSubmitError(SUBMIT_ERROR_MESSAGE);
    }
  }
}
