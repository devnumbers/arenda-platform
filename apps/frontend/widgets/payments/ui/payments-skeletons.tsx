import type { JSX, ReactNode } from 'react';
import {
  Skeleton,
  SkeletonFormField,
  SkeletonListRow,
  skeletonBlockClass,
  skeletonRowWidths,
} from '@/shared/ui/design';

/**
 * Скелетоны-архетипы детализаций платежей (#606, паритет — §7 DESIGN.md):
 * композиции примитивов #604 под финальный лейаут каждой страницы —
 * страница правила (hero-карточка, круглые кнопки, серые секции со
 * строками PaymentRowButton), страница операции и
 * проекция (hero, «Данные операции», «Подробнее»), плоские группы
 * «заголовок + строки» истории и графика. Шапки (TopNav, заголовок) и
 * чип сортировки истории рендерятся вне фазы загрузки — контент занимает
 * место скелетона без сдвига.
 */

/** Приглушённый тон блоков внутри серых карточек (§7, skeletonBlockClass). */
const MUTED = skeletonBlockClass('muted');

/** Серая группа-заглушка: каркас PaymentsGroup — заголовок 20/24 (pt-6 pb-2)
 * и строки 24/12, снизу баланс 12; боковые поля экрана приносит секция
 * (mx-6). */
function SkeletonPaymentsGroup({ children }: { readonly children: ReactNode }): JSX.Element {
  return (
    <section aria-hidden className="mx-6 rounded-card bg-surface-muted pb-3">
      <div className="px-6 pb-2 pt-6">
        <Skeleton className={`h-6 w-40 ${MUTED}`} />
      </div>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}

/** Серая секция страницы операции-заглушка: каркас OperationSection —
 * заголовок H3 20/24 (px-6 pb-2) и строки без карточки-подложки. */
function SkeletonOperationSection({ children }: { readonly children: ReactNode }): JSX.Element {
  return (
    <section aria-hidden>
      <div className="px-6 pb-2">
        <Skeleton className="h-6 w-40" />
      </div>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}

/** Круглая кнопка с подписью в две строки (94 = 56 + 8 + 30) — высота
 * ряда круглых кнопок арендного платежа («Изменить аренду», «Завершить
 * аренду», решение #802) и тройки страницы аренды; однострочный архетип
 * SkeletonRoundAction (79) на 15 ниже и сдвигал бы секции. */
function SkeletonRoundActionTwoLineCaption(): JSX.Element {
  return (
    <span aria-hidden className="flex flex-col items-center gap-2">
      <Skeleton className="h-14 w-14 rounded-pill" />
      <span className="flex flex-col items-center">
        <Skeleton className={`h-[15px] w-16 ${MUTED}`} />
        <Skeleton className={`h-[15px] w-12 ${MUTED}`} />
      </span>
    </span>
  );
}

/**
 * Скелетон страницы платежа (#606): hero-карточка правила, круглые кнопки
 * с подписями в две строки и три секции каркаса в срезе типичной активной
 * аренды (решение владельца 22.10): «Ближайший платеж» — строка,
 * «Просроченные платежи» — пустой подзаголовок (у здорового платежа
 * просрочек нет), «История платежа» — строка: блок постоянен в данных
 * (#1338), порция ждётся серверным префетчем. Высоты — по реальному
 * контенту: кнопочный ряд 94, секция со строкой 152 (строка 68: py-3
 * скелетона против py-3 реальной PaymentRowButton — канон ряда 12/12
 * после payment-row-spacing; прежняя калибровка 60/py-1.5 сидела на
 * px-3 pb-2), секция
 * с пустым подзаголовком 116 (hint 32 = текст 16 + pb-4). Зазор секций
 * 16px (3214:76434, #1194 — как у контента); плиток подэкранов нет
 * (снесены #1073). Состояния вне типового среза («все оплачено» —
 * «Ближайший платеж» пустой, свежее правило — история пуста, просрочки)
 * доезжают в пределах одной секции: статичный каркас не знает состояния
 * до загрузки.
 */
export function PaymentDetailSkeleton(): JSX.Element {
  return (
    <>
      <div className="px-6">
        <section aria-hidden className="flex flex-col gap-3 rounded-card bg-surface-muted p-6">
          <span className="flex flex-col items-start gap-2">
            <Skeleton className={`h-[18px] w-2/5 ${MUTED}`} />
            <Skeleton className={`h-6 w-1/2 ${MUTED}`} />
            <Skeleton className={`h-4 w-1/4 ${MUTED}`} />
          </span>
          <span className="flex items-center gap-1.5">
            <Skeleton className={`h-4 w-4 shrink-0 ${MUTED}`} />
            <Skeleton className={`h-4 w-2/5 ${MUTED}`} />
          </span>
        </section>
      </div>

      <div className="px-6" aria-hidden>
        <div className="grid grid-cols-3">
          <SkeletonRoundActionTwoLineCaption />
          <SkeletonRoundActionTwoLineCaption />
          <SkeletonRoundActionTwoLineCaption />
        </div>
      </div>

      <div className="flex flex-col gap-4">
        <SkeletonPaymentsGroup>
          <SkeletonListRow value tone="muted" className="py-3" />
        </SkeletonPaymentsGroup>

        <SkeletonPaymentsGroup>
          <div className="px-6 pb-4">
            <Skeleton className={`h-4 w-56 ${MUTED}`} />
          </div>
        </SkeletonPaymentsGroup>

        <SkeletonPaymentsGroup>
          <SkeletonListRow value tone="muted" className="py-3" />
        </SkeletonPaymentsGroup>
      </div>
    </>
  );
}

/** Строка «Подробнее»-заглушка: каркас DetailRow — лейбл слева и значение
 * справа, 14/16 с py-1, прямо на белом. */
function SkeletonDetailRow(): JSX.Element {
  return (
    <span aria-hidden className="flex items-center justify-between gap-3 px-6 py-1">
      <Skeleton className="h-4 w-16" />
      <Skeleton className="h-4 w-24" />
    </span>
  );
}

/**
 * Скелетон страницы операции (#606): hero (круг категории 96, название,
 * чип, сумма 40, подпись срока), секции «Данные операции» (строки-ссылки
 * на правило и объект) и «Подробнее» (строки «лейбл — значение»). Служит
 * и проекционному просмотру — тот же OperationView. Sticky-кнопка оплаты
 * вне потока — скелетоном не зеркалится.
 */
export function OperationDetailSkeleton(): JSX.Element {
  return (
    <>
      <div aria-hidden className="flex flex-col items-center gap-3 px-6 pt-6 pb-6">
        <Skeleton className="h-24 w-24 rounded-pill" />
        <Skeleton className="h-6 w-2/5" />
        <Skeleton className="h-8 w-28 rounded-pill" />
        <Skeleton className="h-[44px] w-3/5" />
        <Skeleton className="h-[18px] w-1/4" />
      </div>

      <SkeletonOperationSection>
        <SkeletonListRow className="py-3" />
        <SkeletonListRow className="py-3" />
      </SkeletonOperationSection>

      <SkeletonOperationSection>
        <SkeletonDetailRow />
        <SkeletonDetailRow />
        <SkeletonDetailRow />
      </SkeletonOperationSection>
    </>
  );
}

/**
 * Скелетон плоской группы «заголовок + строки» (#606) — история операций
 * и график платежей: заголовок даты/секции и строки канона
 * PaymentRowButton px-3 py-3 со суммой. Число групп и строк — типовой
 * срез контента (первая порция).
 */
export function PaymentGroupedListSkeleton({
  rowsPerGroup,
}: {
  readonly rowsPerGroup: ReadonlyArray<number>;
}): JSX.Element {
  return (
    <>
      {rowsPerGroup.map((rows, groupIndex) => (
        <section key={groupIndex} aria-hidden className="flex flex-col">
          {/* Верх 24 и гэп 8 под заголовком — паритет PaymentsHeading и
           * контейнера группы (канон Heading, Figma 1036-40829). */}
          <Skeleton className="mx-6 mt-6 h-6 w-24" />
          <div className="pt-2">
            {/* px-6 — плоские 24 до контента, как у реального ряда (12+12). */}
            {skeletonRowWidths(rows).map((rowWidths, index) => (
              <SkeletonListRow key={index} value widths={rowWidths} className="px-6 py-3" />
            ))}
          </div>
        </section>
      ))}
    </>
  );
}

/**
 * Скелетоны визардов и формы правки (#607, паритет — §7 DESIGN.md):
 * композиции примитивов под первый шаг каждого мастера и под форму правки
 * платежа. Хром шага (TopNav с кнопками и чипом, заголовок шага) рендерится
 * вне фазы загрузки — скелетон закрывает только зависящий от данных
 * контент.
 */

/** Скелетон шага «Выберите категорию платежа» (#607): каркас списка
 * CategoryStep — строки ListRow (иконка 44, название, кружок выбора) с
 * вставкой строк py-1.5 и зазором 4. Число строк — типовой экран Cold
 * entry (8). */
export function CategoryRowsSkeleton({ rows = 8 }: { readonly rows?: number }): JSX.Element {
  const widths = skeletonRowWidths(rows);
  return (
    <div aria-hidden className="flex flex-col pt-6">
      {widths.map((rowWidths, index) => (
        <SkeletonListRow
          key={index}
          subtitle={false}
          trailing
          widths={rowWidths}
          className="gap-4 py-1.5"
        />
      ))}
    </div>
  );
}

/**
 * Скелетон формы правки платежа (#607, паритет — §7 DESIGN.md): каркас
 * PaymentEditForm по макету 1127-32083 (#1197) — «Сумма» с сегментом
 * направления, «Название платежа» (с нижней строкой счётчика 13/15, как
 * у TextField с maxLength), поля-кнопки (категория, регулярность,
 * окончание с подсказкой, напоминание/уведомление), ряд email-уведомлений
 * и danger-кнопка удаления. Роль в загрузке неизвестна — строка удаления
 * рисуется всегда: владелец (сценарий по умолчанию) получает точный
 * паритет, а смотрящему форму всё равно замещает карточка «Правка
 * недоступна». Sticky-панель «Сохранить» и хедер вне потока — не зеркалятся.
 */
export function PaymentEditFormSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col gap-8 px-6">
      <span className="flex flex-col gap-2">
        <SkeletonFormField labelWidth="w-16" />
        <Skeleton className="h-11 w-full rounded-2xl" />
      </span>
      <span className="flex flex-col gap-2">
        <SkeletonFormField labelWidth="w-36" />
        <Skeleton className="h-[15px] w-28" />
      </span>
      <SkeletonFormField labelWidth="w-24" />
      <SkeletonFormField labelWidth="w-44" />
      <span className="flex flex-col gap-2">
        <SkeletonFormField labelWidth="w-40" />
        <Skeleton className="h-[15px] w-32" />
      </span>
      <SkeletonFormField labelWidth="w-44" />
      {/* Ряд email-уведомлений (NotifyEmailRow): заголовок + подпись
          слева, тумблер справа. */}
      <span className="flex items-center justify-between gap-4 py-3">
        <span className="flex min-w-0 flex-col gap-1">
          <Skeleton className="h-[18px] w-48" />
          <Skeleton className="h-4 w-64 max-w-full" />
        </span>
        <Skeleton className="h-7 w-12 rounded-pill" />
      </span>
      <Skeleton className="h-14 w-full rounded-button" />
    </div>
  );
}

/**
 * Скелетон шага «Добавить операцию» (#607): каркас OperationAmountStep —
 * денежное поле и сегмент «Расход/Доход» шага — единый ярус (макет
 * 2913:69553, решение владельца 2026-10-22): бокс «Сумма» 56px и сегмент
 * во всю колонку, pt-24/gap-24. Нижняя панель
 * «Продолжить» — постоянная часть шага, её приносит экран реальной
 * кнопкой.
 */
export function OperationAmountStepSkeleton(): JSX.Element {
  return (
    <div aria-hidden className="flex flex-col items-center gap-6 px-6 pt-6 pb-16">
      <Skeleton className="h-14 w-full rounded-button" />
      {/* h-11 — реальная высота сегмента 44 (min-h-10 пилюль + p-[2px]
          трека), скелетон не дрейфует относительно шага. */}
      <Skeleton className="h-11 w-full rounded-2xl" />
    </div>
  );
}
