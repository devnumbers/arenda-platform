'use client';

import type { JSX, ReactNode } from 'react';
import { EmptyState } from '@/shared/ui/design';
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
}: {
  readonly groups: ReadonlyArray<PaymentHistoryGroup>;
  readonly onSelectOperation: (operation: PaymentOperation) => void;
  /** Sentinel бесконечного скролла и индикатор подгрузки следующей порции. */
  readonly tail?: ReactNode;
  /** Подзаголовок строки: глобальная лента (#541) пишет имя объекта
   * (макет 1733-26973), объектные списки подзаголовка не передают. */
  readonly renderSubtitle?: (operation: PaymentOperation) => ReactNode;
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
              subtitle={renderSubtitle?.(operation)}
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
      imageSrc="/images/payments/operations-empty.webp"
      className="py-16"
      description="Операции не найдены. Попробуйте выбрать другой период"
    />
  );
}

/**
 * Полностью пустой объект/книга (#478, Figma 1518-92899): вместо чипов,
 * сводки и списка — канон EmptyState (унификация #1004: заголовок 20/24,
 * описание 16/18, канонный отступ pt-16 — прежняя самописная разметка
 * 16/500 + 14/400 с pt-10 снята). Иконка поиска в хедере вместе с этим
 * состоянием скрывается — искать нечего. Опциональный CTA под описанием
 * (#571, решение владельца 2026-09-08) — «Добавить операцию» ведёт в
 * визард.
 */
export function OperationsNeverHad({
  action,
}: {
  readonly action?: ReactNode;
}): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/payments/operations-empty.webp"
      title="Операций еще не было"
      description="Добавьте аренду, другие платежи и начните отмечать оплату"
      action={action}
    />
  );
}

/** Строка операции (1332:61665, Row Button White): иконка категории с белым
 * кантом, название, знаковая сумма — доход зелёным с плюсом, расход тёмным
 * с минусом (Figma 1492:42480). Опциональный подзаголовок — дата в строках
 * результатов объектного поиска (Figma 1494-61679), списки по датам его не
 * передают; опциональное правое нижнее поле — дата под суммой в строках
 * глобального поиска (макет 1726-90433, #543). */
export function OperationRow({
  operation,
  onSelect,
  subtitle,
  description,
  className = 'px-3 py-3',
}: {
  readonly operation: PaymentOperation;
  readonly onSelect: () => void;
  readonly subtitle?: ReactNode;
  readonly description?: ReactNode;
  /** Дополнение/замена вставок кнопки: tailwind-merge в PaymentRowButton
   * поглощает базовый px-3. */
  readonly className?: string;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);
  return (
    <PaymentRowButton
      className={className}
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={operation.title}
      subtitle={subtitle}
      description={description}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}
