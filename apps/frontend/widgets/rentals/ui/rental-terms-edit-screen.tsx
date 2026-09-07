'use client';

import { useState } from 'react';
import type { JSX, ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Calendar, Cancel, Check } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { addDays, type IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { kopecksToAmountInputString, parseRublesToKopecks } from '@/shared/lib/format-money';
import {
  buildRentalUpdateCommand,
  currentRentalOf,
  paymentDayLabel,
  rentalEditFormFromRental,
  rentalPlannedEndDateEditError,
  RENTAL_COMMENT_MAX,
  useRentals,
  useUpdateRental,
  type RentalEditForm,
} from '@/features/rentals';
import type { Rental } from '@/entities/rental';
import {
  Button,
  CalendarDatePicker,
  IconButton,
  PageContent,
  Skeleton,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  AutoPayRow,
  FieldTitle,
  MoneyField,
  PaymentDayPicker,
  PickerTriggerBox,
  UtilitiesPickerField,
} from './wizard-chrome';

/**
 * Экран «Изменить условия» (#532, Figma 1302:53055): форма, не визард —
 * все поля на одной странице, значения предзаполнены арендой (анатомия
 * правки платежа #467: шапка «Отмена | Заголовок | Check» — быстрая
 * клавиша сохранения наравне со sticky-кнопкой; без изменений и при
 * невалидной форме обе гаснут). Поля в порядке макета: арендная плата*,
 * день оплаты*, начало аренды, окончание, залог, комиссия, коммунальные
 * платежи, тумблер автоплатежа, комментарий 0/2000.
 *
 * «Начало аренды» — read-only (ADR 0053 §3: «Начало не правится», PATCH
 * startDate не принимает; иконки календаря у поля нет). «За сколько
 * напоминать», тумблеры email-уведомлений макета и арендатор вырезаны
 * (тело #532; в контракте аренды их нет). Окончание — канонический
 * бесконечный календарь: снятая дата — бессрочная (tri-state null),
 * будущие дни открыты, дни ≤ начала погашены (minDate). Сумма, день
 * оплаты, автоплатёж и окончание сервер синхронно переносит на Платёж
 * арендной платы — фронт шлёт дифф формы (edit-model) одним PATCH.
 * Сохранение — тост и возврат на «Условия аренды».
 */
export function RentalTermsEditScreen({
  propertyId,
}: {
  readonly propertyId: string;
}): JSX.Element {
  const router = useRouter();
  const rentalsQuery = useRentals(propertyId);
  const rental = currentRentalOf(rentalsQuery.data ?? []);
  const close = (): void => goBack(router, ROUTES.propertyRentalTerms(propertyId));

  if (rentalsQuery.isPending) {
    return (
      <EditShell onClose={close}>
        <div className="flex flex-col gap-4 pt-6">
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
          <Skeleton className="h-14 w-full" />
        </div>
      </EditShell>
    );
  }

  if (rentalsQuery.isError) {
    return (
      <EditShell onClose={close}>
        <div className="flex flex-col items-center gap-4 pt-6">
          <p className="text-center text-base leading-[18px] text-content-secondary">
            Не удалось загрузить аренду
          </p>
          <Button variant="secondary" size="small" onClick={() => void rentalsQuery.refetch()}>
            Повторить
          </Button>
        </div>
      </EditShell>
    );
  }

  // Точка входа — карандаш в шапке «Условий аренды», текущая аренда там
  // уже на экране; сюда попадаем только с непустой арендой.
  if (rental === undefined) {
    return (
      <EditShell onClose={close}>
        <div className="pt-6">
          <p className="text-center text-base leading-[18px] text-content-secondary">
            Аренда не найдена
          </p>
        </div>
      </EditShell>
    );
  }

  return <RentalTermsEditForm key={rental.id} rental={rental} onClose={close} />;
}

/** Оболочка состояний без формы: шапка с крестиком + контент страницы. */
function EditShell({
  onClose,
  children,
}: {
  readonly onClose: () => void;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <>
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Отменить правку" onClick={onClose} />
        }
      />
      <PageContent>{children}</PageContent>
    </>
  );
}

/** Форма правки: рабочее состояние и пикеры живут здесь; key по id аренды
 * пересоздаёт форму при смене данных (прецедент правки платежа). */
function RentalTermsEditForm({
  rental,
  onClose,
}: {
  readonly rental: Rental;
  readonly onClose: () => void;
}): JSX.Element {
  const updateRental = useUpdateRental(rental.propertyId, rental.id);

  // today — серверный (TZ собственника): границы валидации и форматы дат.
  const today: IsoDate = rental.today;

  const [form, setForm] = useState<RentalEditForm>(() => rentalEditFormFromRental(rental));
  // «Сырые» набранные значения денег — источник отображения (группировка
  // не сбрасывает каретку); копейки едут в форму для диффа и валидации.
  const [amountRaw, setAmountRaw] = useState(() =>
    kopecksToAmountInputString(rental.rentPayment.amountKopecks),
  );
  const [depositRaw, setDepositRaw] = useState(() =>
    rental.depositKopecks === null ? '' : kopecksToAmountInputString(rental.depositKopecks),
  );
  const [commissionRaw, setCommissionRaw] = useState(() =>
    rental.commissionKopecks === null ? '' : kopecksToAmountInputString(rental.commissionKopecks),
  );

  const [dayPickerOpen, setDayPickerOpen] = useState(false);
  const [endPickerOpen, setEndPickerOpen] = useState(false);

  const update = <K extends keyof RentalEditForm>(key: K, value: RentalEditForm[K]): void =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const command = buildRentalUpdateCommand(rental, form);
  const canSave = command !== undefined;
  // Ошибка видна только у изменённого значения: предзаполненное окончание
  // «needs_attention»-аренды может быть в прошлом — сервер его принял, и
  // правка прочих полей не обязана чинить дату (ADR 0053 §3).
  const endError =
    form.plannedEndDate !== rental.plannedEndDate
      ? rentalPlannedEndDateEditError(form.plannedEndDate, rental.startDate, today)
      : undefined;

  const save = async (): Promise<void> => {
    if (command === undefined) {
      return;
    }
    try {
      await updateRental.mutateAsync(command);
      notify.scenarios.rentals.updated();
      onClose();
    } catch (error) {
      notify.scenarios.rentals.updateError(error);
    }
  };

  return (
    <>
      {/* Шапка (Figma 1302:53055): Отмена | «Изменить условия» | Check. */}
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Отменить правку" onClick={onClose} />
        }
        trailing={
          <IconButton
            icon={<Check />}
            label="Сохранить"
            disabled={!canSave || updateRental.isPending}
            onClick={() => void save()}
          />
        }
      >
        <TopNavTitle title="Изменить условия" />
      </TopNav>

      <PageContent>
        <div className="flex flex-col gap-8 px-6 pt-6">
          <MoneyField
            title="Арендная плата"
            required
            raw={amountRaw}
            onRawChange={(raw) => {
              setAmountRaw(raw);
              update(
                'amountKopecks',
                raw === '' ? undefined : parseRublesToKopecks(raw, { positive: true }),
              );
            }}
            onClear={() => {
              setAmountRaw('');
              update('amountKopecks', undefined);
            }}
            ariaLabel="Арендная плата, рублей"
          />

          <PickerTriggerBox
            title="День оплаты"
            required
            value={form.paymentDay === undefined ? undefined : paymentDayLabel(form.paymentDay)}
            placeholder="Выбрать день"
            icon={<Calendar className="h-6 w-6" />}
            onClick={() => setDayPickerOpen(true)}
          />

          {/* Начало не правится (ADR 0053 §3) — read-only бокс без иконки. */}
          <div className="flex w-full flex-col gap-2 font-sans">
            <FieldTitle title="Начало аренды" />
            <div className="flex h-14 w-full items-center rounded-button bg-surface-muted py-0 pl-[18px] pr-2">
              <span className="min-w-0 flex-1 truncate text-base leading-[18px] text-content">
                {formatDayMonthWithYear(rental.startDate, today)}
              </span>
            </div>
          </div>

          <PickerTriggerBox
            title="Окончание аренды"
            value={
              form.plannedEndDate === null
                ? 'Бессрочно'
                : formatDayMonthWithYear(form.plannedEndDate, today)
            }
            placeholder="Выбрать дату"
            icon={<Calendar className="h-6 w-6" />}
            onClick={() => setEndPickerOpen(true)}
            error={endError}
          />

          <MoneyField
            title="Залог"
            raw={depositRaw}
            onRawChange={(raw) => {
              setDepositRaw(raw);
              update('depositKopecks', raw === '' ? null : parseRublesToKopecks(raw) ?? null);
            }}
            onClear={() => {
              setDepositRaw('');
              update('depositKopecks', null);
            }}
            ariaLabel="Залог, рублей"
          />

          <MoneyField
            title="Комиссия"
            raw={commissionRaw}
            onRawChange={(raw) => {
              setCommissionRaw(raw);
              update('commissionKopecks', raw === '' ? null : parseRublesToKopecks(raw) ?? null);
            }}
            onClear={() => {
              setCommissionRaw('');
              update('commissionKopecks', null);
            }}
            ariaLabel="Комиссия, рублей"
          />

          <UtilitiesPickerField
            value={form.utilities}
            onChange={(utilities) => update('utilities', utilities)}
          />

          <AutoPayRow
            checked={form.autoPay}
            onCheckedChange={(checked) => update('autoPay', checked)}
          />

          <TextField
            variant="titleOut"
            title="Комментарий"
            multiline
            autoGrow
            maxLength={RENTAL_COMMENT_MAX}
            value={form.comment}
            onChange={(event) => update('comment', event.target.value)}
            aria-label="Комментарий"
          />
        </div>
      </PageContent>

      {/* Паддинг 24 даёт сама панель; дополнительная обёртка давала бы
          двойной отступ (канон правки платежа). */}
      <StickyBottomBar>
        <Button
          className="w-full"
          disabled={!canSave}
          loading={updateRental.isPending}
          onClick={() => void save()}
        >
          Сохранить изменения
        </Button>
      </StickyBottomBar>

      {/* Рендер только в открытом состоянии — лента и черновик живут, пока
          пикер смонтирован (конвенция канона). */}
      {dayPickerOpen && (
        <PaymentDayPicker
          initial={form.paymentDay}
          onClose={() => setDayPickerOpen(false)}
          onConfirm={(day) => {
            if (day !== undefined) {
              update('paymentDay', day);
            }
            setDayPickerOpen(false);
          }}
        />
      )}
      {endPickerOpen && (
        <CalendarDatePicker
          title="Окончание аренды"
          today={today}
          value={form.plannedEndDate}
          minDate={addDays(rental.startDate, 1)}
          onClose={() => setEndPickerOpen(false)}
          onConfirm={(date) => {
            update('plannedEndDate', date);
            setEndPickerOpen(false);
          }}
        />
      )}
    </>
  );
}
