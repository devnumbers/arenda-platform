'use client';

import type { JSX, ReactNode } from 'react';
import Image from 'next/image';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { PaymentRowButton, type PaymentOperation } from '@/entities/payment';
import type { PaymentHistoryGroup } from '@/features/payments';
import { PaymentsHeading } from './payments-sections';

/**
 * Список оплаченных операций, общий для экранов операций объекта (#474) и
 * «Доходы/Расходы объекта» (#475): группы по датам с лейблами Figma,
 * строки Row Button White со знаковыми суммами (доход зелёным с плюсом,
 * расход тёмным с минусом — 1492:42480), пустой период — иллюстрация 128
 * и одна строка серым (1510-77308). Хвост — слот бесконечного скролла
 * (sentinel + индикатор подгрузки), список его не знает.
 */

export function OperationsDateList({
  groups,
  onSelectOperation,
  tail,
}: {
  readonly groups: ReadonlyArray<PaymentHistoryGroup>;
  readonly onSelectOperation: (operation: PaymentOperation) => void;
  /** Sentinel бесконечного скролла и индикатор подгрузки следующей порции. */
  readonly tail?: ReactNode;
}): JSX.Element {
  if (groups.length === 0) {
    return <OperationsEmptyPeriod />;
  }
  return (
    <div className="flex flex-col gap-2">
      {groups.map((group) => (
        <section key={group.date} className="flex flex-col">
          <PaymentsHeading>{group.label}</PaymentsHeading>
          {group.operations.map((operation) => (
            <OperationRow
              key={operation.id}
              operation={operation}
              onSelect={() => onSelectOperation(operation)}
            />
          ))}
        </section>
      ))}
      {tail}
    </div>
  );
}

/** Пустой период (Figma 1510-77308): иллюстрация 128, одна строка 16/18
 * серым, блок с отступами 64. */
export function OperationsEmptyPeriod(): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-4 py-16">
      <Image
        src="/images/payments/operations-empty.png"
        alt=""
        width={128}
        height={128}
        className="h-32 w-32"
      />
      <p className="text-base leading-[18px] text-content-secondary">
        Операции не найдены. Попробуйте выбрать другой период
      </p>
    </div>
  );
}

/** Строка операции (1332:61665, Row Button White): иконка категории с белым
 * кантом, название, знаковая сумма — доход зелёным с плюсом, расход тёмным
 * с минусом (Figma 1492:42480). Опциональный подзаголовок — дата в строках
 * результатов поиска (Figma 1494-61679), списки по датам его не передают. */
export function OperationRow({
  operation,
  onSelect,
  subtitle,
}: {
  readonly operation: PaymentOperation;
  readonly onSelect: () => void;
  readonly subtitle?: ReactNode;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  return (
    <PaymentRowButton
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={operation.title}
      subtitle={subtitle}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}

export function LoadingMoreIndicator(): JSX.Element {
  return (
    <div className="flex justify-center py-4" role="status" aria-label="Загружаем еще">
      <div className="h-8 w-8 animate-pulse rounded-pill bg-surface-muted" />
    </div>
  );
}
