'use client';

import type { JSX } from 'react';
import type { Property } from '@/entities/property';
import {
  PROPERTY_CREATE_RENTAL_STUB_TOAST,
  propertyCreateSuccessCopy,
  propertyTypeIcons,
} from '@/features/properties';
import { notify } from '@/shared/lib/notifications';
import { Button, StatusIcon, StickyBottomBar } from '@/shared/ui/design';
import { PropertyWizardBottomBar } from './wizard-chrome';

/**
 * Экран успеха визарда создания объекта (#483, Figma 1425-55788): в центре
 * иконка категории — круг 96 (#F3F4F6) с кантом под поверхность и глифом
 * типа (#D3D7D9, как в макете), снизу справа белый бейдж «готово»; под ней
 * заголовок «Объект «{название}» создан» (20/24 SemiBold) и подзаголовок.
 * Кнопки — пара из макета (override владельца 2026-09-02): primary
 * «Добавить аренду» и secondary «Открыть объект». Домена аренд в продукте
 * нет (ADR 0046) — «Добавить аренду» заглушка: тост и остаёмся на экране.
 * Хедер экрана рисует флоу: только крестик слева, без чипа шага.
 */

export type WizardSuccessProps = {
  readonly created: Property;
  readonly onOpen: () => void;
};

export function WizardSuccess({ created, onOpen }: WizardSuccessProps): JSX.Element {
  const copy = propertyCreateSuccessCopy(created.name);
  // Выборка из статичного реестра, не вызов: react-hooks/static-components.
  const Icon = propertyTypeIcons[created.type];

  return (
    <div className="flex flex-col gap-8 px-8 pt-16">
      <div className="flex flex-col items-center gap-8">
        {/* Композиция фрейма 1425:55793: круг 96, глиф 52 по центру, бейдж
            «готово» 52 накладывается со смещением (61, 61) — выходит за
            круг в правый нижний угол. */}
        <span className="relative block h-24 w-24">
          <span className="flex h-24 w-24 items-center justify-center rounded-full bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]">
            <Icon className="h-[52px] w-[52px] text-input-border" aria-hidden />
          </span>
          <StatusIcon
            status="good"
            className="absolute left-[61px] top-[61px] h-[52px] w-[52px]"
          />
        </span>
        <div className="flex flex-col gap-3 self-stretch">
          <h1 className="text-center font-sans text-xl leading-6 font-semibold text-content">
            {copy.heading}
          </h1>
          <p className="text-center text-base leading-[18px] text-content-secondary">
            {copy.description}
          </p>
        </div>
      </div>
      <StickyBottomBar>
        <PropertyWizardBottomBar>
          <Button
            className="w-full"
            onClick={() => notify.info(PROPERTY_CREATE_RENTAL_STUB_TOAST)}
          >
            Добавить аренду
          </Button>
          <Button variant="secondary" className="w-full" onClick={onOpen}>
            Открыть объект
          </Button>
        </PropertyWizardBottomBar>
      </StickyBottomBar>
    </div>
  );
}
