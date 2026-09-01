'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import {
  IconButton,
  PageContent,
  StepsChip,
  TopNav,
  useTabBarSuppression,
} from '@/shared/ui/design';
import {
  initialPropertyCreateStep,
  PROPERTY_CREATE_TOTAL_STEPS,
  usePropertyCreateDraft,
  type PropertyCreateStep,
} from '@/features/properties';
import { CategoryStep } from './category-step';
import { PropertyWizardHeading } from './wizard-chrome';

/**
 * Поток шагов визарда создания объекта (#480): клиентское состояние на
 * одном маршруте /properties/new, восстанавливается на первый
 * незавершённый шаг черновика. Хедер шагов (Figma 1213-52111 /
 * 1213-52017): шаг 1 — крестик слева; шаги 2–3 — «назад» слева и крестик
 * справа; в центре чип «шаг N из 3». Экран успеха шагом не считается —
 * придёт с POST-сабмитом (#482) и своей разметкой (#483).
 */

export function PropertyCreateWizardFlow(): JSX.Element {
  const router = useRouter();
  const { draft, setDraft } = usePropertyCreateDraft();
  // Визард — экран создания: футер глушится на всех шагах, включая те,
  // где нижняя панель ещё не смонтирована (аналог визарда платежей #464).
  useTabBarSuppression();
  const [step, setStep] = useState<PropertyCreateStep>(() => initialPropertyCreateStep(draft));

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
              onSelect={(type) => {
                setDraft((prev) => ({ ...prev, type }));
                setStep(2);
              }}
            />
          </>
        )}
        {/* Шаг 2 «Адрес» — тикет #481 (Figma 1213-52017/52391, 1519-94336),
            шаг 3 «Характеристики» — #482 (Figma 1218-54295): хедер и
            переходы флоу финальные, контент шагов дозревает в своих
            тикетах. */}
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
}
