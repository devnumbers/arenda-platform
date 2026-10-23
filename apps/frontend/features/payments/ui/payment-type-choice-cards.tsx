'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import type { PaymentDraftType } from '../lib/use-payment-wizard-draft';

/**
 * Сетка выбора типа создания «Платёж / Автоплатёж» (Figma 3065:71128) —
 * один канон двух входов: шита единого входа #453 (объект известен заранее)
 * и модалки хаба «Платежей» (ведёт в глобальный визард /payments/new,
 * решение владельца 23.10.2026). Куда ведёт выбор, решает потребитель.
 */

/** Копирайт карточек — дословно из фрейма 3065:71128 (включая «Платеж»
 * без «ё»): label + синий action, длинные description прежних фреймов
 * (1134:40415 / 1134:39570) в новом макете не видны — снесены. */
const TYPE_META: Record<
  PaymentDraftType,
  {
    readonly label: string;
    readonly action: string;
    readonly image: string;
  }
> = {
  payment: {
    label: 'Платеж',
    action: 'Отмечайте оплату\nвручную',
    image: '/images/payments/sheet-choice-payment.webp',
  },
  autopayment: {
    label: 'Автоплатеж',
    action: 'Отмечается автоматически',
    image: '/images/payments/sheet-choice-autopayment.webp',
  },
};

export function PaymentTypeChoiceCards({
  onSelect,
}: {
  readonly onSelect: (type: PaymentDraftType) => void;
}): JSX.Element {
  return (
    <div className="grid grid-cols-2 gap-2">
      {(Object.keys(TYPE_META) as PaymentDraftType[]).map((type) => (
        <ChoiceCard key={type} type={type} onSelect={() => onSelect(type)} />
      ))}
    </div>
  );
}

function ChoiceCard({
  type,
  onSelect,
}: {
  readonly type: PaymentDraftType;
  readonly onSelect: () => void;
}): JSX.Element {
  const meta = TYPE_META[type];
  return (
    <button
      type="button"
      onClick={onSelect}
      className="pressable flex cursor-pointer flex-col gap-4 rounded-card bg-surface-muted p-4 text-left outline-none focus-visible:ring-4 focus-visible:ring-primary hover:opacity-80"
    >
      {/* priority — как в EmptyState: иллюстрация внутри открывающегося
          шита, ленивую загрузку наблюдатель вьюпорта может пропустить.
          Слот 64 — по макету (Figma 3065:71128); sizes = 2x слота —
          суперсэмплинг (визуально 64, грузится 128/256), единое качество
          выдачи 90; исходники 512² — канонный класс источников
          (нормализация импорта #1070, решение владельца 22.10.2026). */}
      <Image
        src={meta.image}
        alt=""
        width={64}
        height={64}
        priority
        sizes="128px"
        quality={90}
        className="h-16 w-16"
      />
      <span className="flex flex-col gap-2">
        <span className="text-base font-medium leading-[18px] text-content">{meta.label}</span>
        <span className="whitespace-pre-line text-sm font-medium leading-4 text-primary">{meta.action}</span>
      </span>
    </button>
  );
}
