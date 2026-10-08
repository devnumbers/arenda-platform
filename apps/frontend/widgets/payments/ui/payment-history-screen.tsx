'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import Image from 'next/image';
import {
  ArrowLeft,
  Edit,
  Hide,
  Show,
  VerticalMenu,
} from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { OPERATIONS_FEED_SORT } from '@/shared/api/query-keys';
import { goBack } from '@/shared/lib/navigation';
import { dateToIsoLocal } from '@/shared/lib/calendar';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';
import {
  PaymentRowButton,
  type PaymentOperation,
} from '@/entities/payment';
import { propertyPermissions } from '@/entities/property';
import { CategoryIcon, categoryStyle } from '@/features/payment-categories';
import { useProperty } from '@/features/properties';
import {
  buildPaymentHistoryTimeline,
  usePaymentChangesPaged,
  usePaymentOperationsPaged,
} from '@/features/payments';
import {
  Button,
  IconButton,
  InfiniteQueryTail,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  PageContent,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { PaymentsHeading, PaymentsStateCard } from './payments-sections';
import { PaymentGroupedListSkeleton } from './payments-skeletons';

/** Собственный параметр режима изменений в адресе — знание экрана;
 * пишется через useUrlParams с own. */
const CHANGES_MODE_PARAMS = ['changes'] as const;

/**
 * Подэкран «История платежа» (макеты 3214-73216/73857 — тикет #1195,
 * раньше «История операций» #466): по дефолту — история С изменениями,
 * чипы правок из журнала (ADR 0065, тексты — фронт, paymentChangeChips)
 * вперемешку с операциями строго по реальному времени (макет «1 сентября»:
 * правки и выше, и ниже оплаты своего дня; день группы — день факта
 * оплаты, #994). Меню «⋮» шапки скрывает изменения (?changes=0 в адресе —
 * переживает перезагрузку, конвенция состояния в адресе #785; дефолтный
 * смешанный режим параметра не пишет) и ведёт «Изменить платеж» (Full
 * Access+, матрица прав; смотрящему остаётся переключение режима).
 * «Задержана/раньше» в истории не показываются — записей о них в журнале
 * нет (ADR 0065 §4). Пустая история — иллюстрация и «Платежей еще не
 * было» (Figma 858:21271). Серверные порции по 50 с бесконечным скроллом
 * (операции — offset, журнал — keyset before_cursor). Суммы расходов —
 * со знаком минус. Подписей-дат в строках нет — канон 1302:52209
 * (решение #802).
 */
export function PaymentHistoryScreen({
  propertyId,
  paymentId,
  initialChangesMode,
}: {
  readonly propertyId: string;
  readonly paymentId: string;
  readonly initialChangesMode?: boolean;
}): JSX.Element {
  const router = useRouter();
  // Смешанный режим — дефолт; «Скрыть изменения» пишет ?changes=0
  // (дефолт в адресе не пишется): стартовое значение парсит страница на
  // сервере, переключение пишет useUrlParams.
  const [changesMode, setChangesMode] = useState(initialChangesMode ?? true);
  const { write } = useUrlParams();

  // Мутационный пункт меню — по матрице прав объекта (смотрящему — только
  // чтение); ошибка чтения объекта не ломает историю: пункт просто скрыт.
  const propertyQuery = useProperty(propertyId);
  const canMutate = propertyQuery.isSuccess
    && propertyPermissions(propertyQuery.data).canEdit;

  const operationsQuery = usePaymentOperationsPaged(propertyId, paymentId, {
    status: 'paid',
    order: 'desc',
    sort: OPERATIONS_FEED_SORT,
  });
  // Журнал изменений грузится только в режиме изменений.
  const changesQuery = usePaymentChangesPaged(propertyId, paymentId, {
    enabled: changesMode,
  });

  const toggleChangesMode = (): void => {
    const next = !changesMode;
    setChangesMode(next);
    // Показ изменений — дефолт: параметр из адреса убирается.
    write(next ? {} : { changes: '0' }, { own: CHANGES_MODE_PARAMS });
  };

  const today = dateToIsoLocal(new Date());
  // Обе фазы — один таймлайн: в дефолтном режиме журнал не подмешивается
  // (закэшированные правки не рисуются), лейблы дней общие — макет 75466
  // («1 сентября» без года в текущем).
  const timeline = buildPaymentHistoryTimeline(
    operationsQuery.data ?? [],
    changesMode ? (changesQuery.data ?? []) : [],
    today,
  );

  // Контент фазы: дефолт ждёт только операции, изменения — оба источника.
  const pending = changesMode
    ? operationsQuery.isPending || changesQuery.isPending
    : operationsQuery.isPending;
  const failed = changesMode
    ? operationsQuery.isError || changesQuery.isError
    : operationsQuery.isError;
  const showEmpty = timeline.length === 0;

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label="Назад"
            onClick={() => goBack(router, ROUTES.propertyPayment(propertyId, paymentId))}
          />
        }
        trailing={
          <Menu>
            <MenuTrigger asChild>
              <IconButton
                icon={<VerticalMenu className="h-6 w-6" />}
                label="Действия с историей"
              />
            </MenuTrigger>
            <MenuContent>
              <MenuItem
                icon={changesMode ? <Hide className="h-6 w-6" /> : <Show className="h-6 w-6" />}
                onSelect={toggleChangesMode}
              >
                {changesMode ? 'Скрыть изменения' : 'Показать изменения'}
              </MenuItem>
              {canMutate && (
                <MenuItem
                  icon={<Edit className="h-6 w-6" />}
                  onSelect={() =>
                    router.push(ROUTES.propertyPaymentEdit(propertyId, paymentId))}
                >
                  Изменить платеж
                </MenuItem>
              )}
            </MenuContent>
          </Menu>
        }
      >
        <TopNavTitle title="История платежа" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-2">
          {pending && <PaymentGroupedListSkeleton rowsPerGroup={[1, 1, 1]} />}

          {failed && (
            <PaymentsStateCard
              title={changesMode ? 'Не удалось загрузить изменения' : 'Не удалось загрузить историю'}
              hint="Проверьте подключение и попробуйте снова"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => {
                    void operationsQuery.refetch();
                    if (changesMode) {
                      void changesQuery.refetch();
                    }
                  }}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {!pending && !failed && (
            <>
              {showEmpty ? (
                // Пустая история (Figma 858:21271): иллюстрация 128 и одна
                // строка 16/18 — без карточки и подсказки.
                <div className="flex flex-col items-center gap-4 pt-24">
                  <Image
                    src="/images/payments/history-empty.png"
                    alt=""
                    width={128}
                    height={128}
                    className="h-32 w-32"
                  />
                  <p className="text-base leading-[18px] text-content">
                    Платежей еще не было
                  </p>
                </div>
              ) : (
                <>
                  {timeline.map((group) => (
                    <section key={group.date} className="flex flex-col">
                      <PaymentsHeading>{group.label}</PaymentsHeading>
                      {group.items.map((item) =>
                        item.kind === 'operation' ? (
                          <HistoryRow
                            key={item.operation.id}
                            operation={item.operation}
                            onSelect={() =>
                              router.push(
                                ROUTES.propertyOperation(propertyId, item.operation.id),
                              )}
                          />
                        ) : (
                          <ChangeChipsBlock key={item.entry.id} chips={item.chips} />
                        ),
                      )}
                    </section>
                  ))}

                  <FeedTail
                    operationsQuery={operationsQuery}
                    changesQuery={changesMode ? changesQuery : null}
                  />
                </>
              )}
            </>
          )}
        </div>
      </PageContent>
    </>
  );
}

/** Блок чипов одной строки журнала (макет 3214-73216, нода 3214-73784):
 * столбец чипов 14/16 на серых подложках radius 16; чип ведёт новым
 * значением и переносится на следующую строку без обрезки. */
function ChangeChipsBlock({ chips }: { readonly chips: ReadonlyArray<string> }): JSX.Element {
  return (
    <div className="px-6 py-1.5">
      <div className="flex flex-col gap-1.5">
        {chips.map((chip) => (
          <div
            key={chip}
            className="rounded-button bg-surface-muted px-5 py-4 text-sm font-medium leading-4 text-content"
          >
            {chip}
          </div>
        ))}
      </div>
    </div>
  );
}

/** Хвост бесконечной ленты (канон #633): в смешанном режиме sentinel
 * дозагружает оба источника, пока у каждого есть продолжение; в скрытом
 * (?changes=0) журнал не подмешивается (changesQuery = null) — хвост ведёт
 * только операции, закэшированные страницы журнала не тянутся. */
function FeedTail({
  operationsQuery,
  changesQuery,
}: {
  readonly operationsQuery: ReturnType<typeof usePaymentOperationsPaged>;
  readonly changesQuery: ReturnType<typeof usePaymentChangesPaged> | null;
}): JSX.Element | null {
  return (
    <InfiniteQueryTail
      query={{
        hasNextPage:
          operationsQuery.hasNextPage === true || changesQuery?.hasNextPage === true,
        isFetchingNextPage:
          operationsQuery.isFetchingNextPage || (changesQuery?.isFetchingNextPage ?? false),
        fetchNextPage: () =>
          Promise.all([
            operationsQuery.hasNextPage ? operationsQuery.fetchNextPage() : Promise.resolve(),
            changesQuery?.hasNextPage ? changesQuery.fetchNextPage() : Promise.resolve(),
          ]),
      }}
    />
  );
}

/** Строка истории (1302:52209, решение #802 23.09): иконка категории с
 * белым кантом (строка внутри страницы), название, сумма справа знаковая:
 * расход с минусом, доход с плюсом зелёным; подписей-дат в строке нет —
 * групповой заголовок несёт дату. onSelect ведёт на страницу операции. */
function HistoryRow({
  operation,
  onSelect,
}: {
  readonly operation: PaymentOperation;
  readonly onSelect?: () => void;
}): JSX.Element {
  const style = categoryStyle('default', operation.categorySlug);

  return (
    <PaymentRowButton
      className="px-3 py-3"
      categoryIcon={<CategoryIcon icon={style.icon} color={style.color} surface="white" />}
      title={operation.title}
      amountKopecks={
        operation.type === 'expense' ? -operation.amountKopecks : operation.amountKopecks
      }
      signedAmount
      onSelect={onSelect}
    />
  );
}
