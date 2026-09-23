'use client';

import { useState } from 'react';
import type { JSX, ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Calendar, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import type { IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import {
  currentRentalOf,
  rentalExtendMinDate,
  rentalExtendSuccessCopy,
  useRentals,
  useUpdateRental,
} from '@/features/rentals';
import type { Rental } from '@/entities/rental';
import {
  Button,
  CalendarDatePicker,
  IconButton,
  PageContent,
  Skeleton,
  StickyBottomBar,
  TopNav,
} from '@/shared/ui/design';
import { PickerTriggerBox } from './wizard-chrome';

/**
 * Экран «Продление аренды» (#533, Figma 1428:58218): вопрос «На сколько
 * продлить аренду?» с единственным полем «Новая дата» — канонический
 * календарь. Нижняя граница — правило продления (ADR 0053 §3/§5): у срочной
 * «строго позже текущего окончания», у needs_attention (окончание в прошлом)
 * и у бессрочной — today (контракт PATCH: даты не раньше сегодняшнего).
 * Условия (сумма/день оплаты) в продлении не меняются — PATCH несёт только
 * plannedEndDate, сервер синхронно переставляет платёж аренды. Шапка —
 * крестик без заголовка (как шаг 1 визарда #530), панель «Отменить /
 * Продлить»: продление притушено, пока дата не выбрана (макет — Disabled).
 * Успех — тост «Аренда продлена еще на N месяцев до ДД.ММ.ГГГГ» на
 * детализации (1550:93664, решение #802 23.09; прежний попап 1550:93723
 * заменён); детализация пересчитывается инвалидацией useUpdateRental.
 *
 * Точка входа — круглая «Продлить аренду» на детализации (там и живёт
 * доступность: Full Access). Продление — сценарий любой незавершённой
 * аренды; бессрочной задаёт первую дату окончания (домен «Продление»,
 * rentals/CONTEXT.md; бэк-контракт PATCH это же и держит — seam-тест
 * бессрочной в rentals_seam_integration_test.go).
 */
export function RentalExtendScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const rentalsQuery = useRentals(propertyId);
  const rental = currentRentalOf(rentalsQuery.data ?? []);
  const close = (): void => goBack(router, ROUTES.propertyRental(propertyId));

  if (rentalsQuery.isPending) {
    return (
      <ExtendShell onClose={close}>
        <div className="flex flex-col gap-4 pt-6">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      </ExtendShell>
    );
  }

  if (rentalsQuery.isError) {
    return (
      <ExtendShell onClose={close}>
        <div className="flex flex-col items-center gap-4 pt-6">
          <p className="text-center text-base leading-[18px] text-content-secondary">
            Не удалось загрузить аренду
          </p>
          <Button variant="secondary" size="small" onClick={() => void rentalsQuery.refetch()}>
            Повторить
          </Button>
        </div>
      </ExtendShell>
    );
  }

  // Точка входа — детализация текущей аренды; без неё продлевать нечего.
  if (rental === undefined) {
    return (
      <ExtendShell onClose={close}>
        <div className="pt-6">
          <p className="text-center text-base leading-[18px] text-content-secondary">
            Аренда не найдена
          </p>
        </div>
      </ExtendShell>
    );
  }

  return <RentalExtendForm key={rental.id} rental={rental} onClose={close} />;
}

/** Оболочка состояний без формы: шапка с крестиком + контент страницы. */
function ExtendShell({
  onClose,
  children,
}: {
  readonly onClose: () => void;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <>
      <ExtendTopNav onClose={onClose} />
      <PageContent>{children}</PageContent>
    </>
  );
}

/** Шапка экрана (Figma 1428:58218): крестик «Отменить» без заголовка —
 * как шаг 1 визарда создания (#530). */
function ExtendTopNav({ onClose }: { readonly onClose: () => void }): JSX.Element {
  return (
    <TopNav
      leading={
        <IconButton icon={<Cancel />} label="Отменить продление" onClick={onClose} />
      }
    />
  );
}

/** Форма продления: черновик даты живёт здесь; key по id
 * аренды пересоздаёт форму при смене данных (прецедент правки условий). */
function RentalExtendForm({
  rental,
  onClose,
}: {
  readonly rental: Rental;
  readonly onClose: () => void;
}): JSX.Element {
  const updateRental = useUpdateRental(rental.propertyId, rental.id);

  // today — серверный (TZ собственника): граница «не в прошлое» и форматы.
  const today: IsoDate = rental.today;
  const currentEnd = rental.plannedEndDate;

  const [newEnd, setNewEnd] = useState<IsoDate | undefined>(undefined);
  const [pickerOpen, setPickerOpen] = useState(false);

  // «Строго позже текущего окончания» (ADR 0053 §5); у needs_attention
  // окончание в прошлом, а у бессрочной его нет вовсе — границу ведёт
  // контракт PATCH (не в прошлое и не раньше дня после начала).
  const minDate = rentalExtendMinDate(currentEnd, rental.startDate, today);

  const extend = async (): Promise<void> => {
    // Гарантировано UI (кнопка живёт только с выбранной датой).
    if (newEnd === undefined) {
      return;
    }
    try {
      await updateRental.mutateAsync({ plannedEndDate: newEnd });
      // Успех — тост на детализации (1550:93664): текст считает либа
      // success-copy, старое окончание снимком до перечитания аренды
      // (у бессрочной его нет — тост без счётчика месяцев).
      notify.success(rentalExtendSuccessCopy({ previousEnd: currentEnd, newEnd }));
      onClose();
    } catch (error) {
      notify.scenarios.rentals.updateError(error);
    }
  };

  return (
    <>
      <ExtendTopNav onClose={onClose} />

      <PageContent>
        <div className="flex flex-col gap-8 px-6 pt-6">
          <div className="flex flex-col gap-2">
            <h1 className="m-0 font-sans text-xl font-semibold leading-6 text-content">
              На сколько продлить аренду?
            </h1>
            <p className="text-base leading-[18px] text-content-secondary">
              Выберите новую дату окончания аренды
            </p>
          </div>

          <PickerTriggerBox
            title="Новая дата"
            value={newEnd === undefined ? undefined : formatDayMonthWithYear(newEnd, today)}
            placeholder="Выбрать дату"
            icon={<Calendar className="h-6 w-6" />}
            onClick={() => setPickerOpen(true)}
          />
        </div>
      </PageContent>

      {/* Панель макета: «Отменить» и «Продлить» рядом равными долями;
          «Продлить» — Disabled, пока дата не выбрана. */}
      <StickyBottomBar>
        <div className="flex gap-2">
          <Button variant="secondary" className="flex-1" onClick={onClose}>
            Отменить
          </Button>
          <Button
            className="flex-1"
            disabled={newEnd === undefined}
            loading={updateRental.isPending}
            onClick={() => void extend()}
          >
            Продлить
          </Button>
        </div>
      </StickyBottomBar>

      {pickerOpen && (
        <CalendarDatePicker
          title="Новая дата"
          today={today}
          value={newEnd ?? null}
          minDate={minDate}
          onClose={() => setPickerOpen(false)}
          onConfirm={(date) => {
            if (date !== null) {
              setNewEnd(date);
            }
            setPickerOpen(false);
          }}
        />
      )}
    </>
  );
}
