'use client';

import type { JSX } from 'react';
import type { PaymentOperation } from '@/entities/payment';
import { CategoryIcon, categoryStyle, type CategoryStyle } from '@/features/payment-categories';
import { StatusIcon } from '@/shared/ui/design';
import { formatMoneyKopecks } from '@/shared/lib/format-money';

/**
 * Экран успеха визарда операции (Figma 1858:105544): иконка категории
 * с зелёным бейджем «выполнено» (StatusIcon good), название операции
 * (пустое поле замещено лейблом категории ещё на сервере), объект и
 * сумма со знаком: расход с минусом, доход с плюсом. Кнопка «Готово» —
 * панель шага во flow.
 */

export type OperationSuccessProps = {
  readonly created: PaymentOperation;
  /** Имя объекта: у глобального входа ответ его не несёт — берём из
   * списка объектов, у входа с объекта — из загруженного объекта. */
  readonly propertyName: string;
};

export function OperationSuccess({
  created,
  propertyName,
}: OperationSuccessProps): JSX.Element {
  const style: CategoryStyle = categoryStyle('default', created.categorySlug ?? '');
  // Знак — отображение направления (Figma: «-6 000 ₽»); сумма хранится
  // положительной. Формат — только канонический форматтер копеек.
  const signedAmount =
    created.type === 'expense'
      ? `-${formatMoneyKopecks(created.amountKopecks)}`
      : `+${formatMoneyKopecks(created.amountKopecks)}`;

  return (
    <div className="flex flex-col items-center px-8 pt-16">
      <div className="flex flex-col items-center gap-8">
        {/* Композиция фрейма 1858:105549: кружок категории 96 (глиф ~52 —
            те же пропорции, что 24 в кружке 44), бейдж ~52 накладывается
            в угол (61, 61). */}
        <span className="relative block h-24 w-24">
          <CategoryIcon
            icon={style.icon}
            color={style.color}
            className="h-24 w-24 [&>svg]:h-13 [&>svg]:w-13"
          />
          <StatusIcon
            status="good"
            className="absolute top-[61px] left-[61px] h-13 w-13"
          />
        </span>
        <div className="flex flex-col gap-1 self-stretch">
          <h1 className="text-center text-xl leading-6 font-normal break-words text-content">
            {created.title}
          </h1>
          <p className="text-center text-base leading-[18px] text-content-secondary">
            {propertyName}
          </p>
        </div>
        <p className="text-center text-[2.5rem] leading-11 font-semibold break-words text-content">
          {signedAmount}
        </p>
      </div>
    </div>
  );
}
