'use client';

import type { JSX, ReactNode } from 'react';
import Image from 'next/image';
import { EmptyState, Skeleton } from '@/shared/ui/design';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { PaymentRowButton, type PaymentOperation } from '@/entities/payment';
import type { PaymentHistoryGroup } from '@/features/payments';
import { PaymentsHeading } from './payments-sections';

/**
 * Список оплаченных операций, общий для экранов операций объекта (#474) и
 * «Доходы/Расходы объекта» (#475): группы по датам с лейблами Figma,
 * строки Row Button White со знаковыми суммами (доход зелёным с плюсом,
 * расход тёмным с минусом — 1492:42480), пустой период — иллюстрация 128
 * и одна строка серым (1510-77308). Здесь же — состояние совсем пустого
 * объекта «Операций еще не было» (1518-92899, #478). Хвост — слот
 * бесконечного скролла (sentinel + индикатор подгрузки), список его
 * не знает.
 */

export function OperationsDateList({
  groups,
  onSelectOperation,
  tail,
  renderSubtitle,
  inset = true,
}: {
  readonly groups: ReadonlyArray<PaymentHistoryGroup>;
  readonly onSelectOperation: (operation: PaymentOperation) => void;
  /** Sentinel бесконечного скролла и индикатор подгрузки следующей порции. */
  readonly tail?: ReactNode;
  /** Подзаголовок строки: глобальная лента (#541) пишет имя объекта
   * (макет 1733-26973), объектные списки подзаголовка не передают. */
  readonly renderSubtitle?: (operation: PaymentOperation) => ReactNode;
  /** Собственные вставки заголовков и строк (px-6/px-3): объектные экраны
   * (#474) держат их внутри PageContent; глобальная лента (#541) держит
   * ритм 24px всей страницей — строки прижаты к её краю (решение
   * владельца 2026-09-05). */
  readonly inset?: boolean;
}): JSX.Element {
  if (groups.length === 0) {
    return <OperationsEmptyPeriod />;
  }
  return (
    <div className="flex flex-col gap-2">
      {groups.map((group) => (
        <section key={group.date} className="flex flex-col">
          <PaymentsHeading inset={inset}>{group.label}</PaymentsHeading>
          {group.operations.map((operation) => (
            <OperationRow
              key={operation.id}
              operation={operation}
              subtitle={renderSubtitle?.(operation)}
              className={inset ? undefined : '-mx-3 px-0 py-3'}
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
 * серым — на каноне EmptyState (py-16, без заголовка). */
export function OperationsEmptyPeriod(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/payments/operations-empty.png"
      className="py-16"
      description="Операции не найдены. Попробуйте выбрать другой период"
    />
  );
}

/**
 * Полностью пустой объект (Figma 1518-92899, #478): вместо чипов, сводки
 * и списка — иллюстрация 128 через 64px после хедера (pt-10 поверх
 * встроенных pt-6 PageContent), заголовок 16/500 и подпись 14/400 серым
 * (320 по ширине). Иконка поиска в хедере вместе с этим состоянием
 * скрывается — искать нечего.
 */
export function OperationsNeverHad(): JSX.Element {
  return (
    <div className="flex flex-col items-center px-6 pt-10">
      <Image
        src="/images/payments/operations-empty.png"
        alt=""
        width={128}
        height={128}
        className="h-32 w-32"
      />
      <div className="mt-4 flex max-w-[320px] flex-col gap-1 text-center">
        <p className="text-base font-medium leading-[18px] text-content">
          Операций еще не было
        </p>
        <p className="text-sm leading-4 text-content-secondary">
          Добавьте аренду, другие платежи и начните отмечать оплату
        </p>
      </div>
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
  className = 'px-3 py-3',
}: {
  readonly operation: PaymentOperation;
  readonly onSelect: () => void;
  readonly subtitle?: ReactNode;
  /** Дополнение/замена вставок кнопки: tailwind-merge в PaymentRowButton
   * поглощает базовый px-3, а -mx-3 дополнительно гасит внутренний px-3
   * контентного фрейма кнопки (глобальная лента: контент строки прижат
   * к ритму страницы 24px). */
  readonly className?: string;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  return (
    <PaymentRowButton
      className={className}
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
      <Skeleton className="h-8 w-8" />
    </div>
  );
}
