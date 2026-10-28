'use client';

import { useEffect, useState } from 'react';
import type { JSX, ReactNode } from 'react';
import { useRouter } from 'next/navigation';
import { Calendar, Cancel, Check } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import { type IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import { kopecksToAmountInputString, parseRublesToKopecks } from '@/shared/lib/format-money';
import { useMe } from '@/features/auth';
import {
  emailNotificationsRowVisible,
  emailReminderCaption,
  NotifyEmailRow,
  RENTAL_EMAIL_CATEGORY,
  useEmailNotificationPreferences,
} from '@/features/notifications';
import {
  buildRentalUpdateCommand,
  clearRentalTermsEditSessionDraft,
  consumeRentalTermsEditSessionContact,
  currentRentalOf,
  formAfterPaymentDayChange,
  keepRentalTermsEditSessionAlive,
  openRentalTermsEditSession,
  paymentDayLabel,
  plannedEndDateMinDate,
  readRentalTermsEditSessionDraft,
  rentalEditFormFromRental,
  rentalPlannedEndDateEditError,
  RENTAL_COMMENT_MAX,
  setRentalTermsEditSessionForm,
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
  ReminderPickerField,
  UtilitiesPickerField,
} from './wizard-chrome';
import { RentalTermsEditFormSkeleton } from './rental-skeletons';
import { TermsContactSection } from './terms-contact-section';

/**
 * Экран «Изменить условия» (#532, Figma 1302:53055): форма, не визард —
 * все поля на одной странице, значения предзаполнены арендой (анатомия
 * правки платежа #467: шапка «Отмена | Заголовок | Check» — быстрая
 * клавиша сохранения наравне со sticky-кнопкой; без изменений и при
 * невалидной форме обе гаснут). Поля в порядке макета: арендная плата*,
 * день оплаты*, начало аренды, окончание, залог, комиссия, коммунальные
 * платежи, тумблер автоплатежа, селект «За сколько напоминать» (#1208,
 * канон шага создания), ряд «Включить уведомления об оплате на почту»
 * (после пикера напоминания — канон #1552), комментарий 0/2000.
 *
 * Секция «Контакт арендатора» возвращена (решение владельца 28.10.2026,
 * #1623; макеты 40000101:153003/153262) — TermsContactSection между рядом
 * почты и комментарием: выбор/создание/кебаб 1-в-1 с шагом 4 визарда,
 * смена едет общим диффом формы (tri-state PATCH contactId), значения
 * формы переживают под-маршруты секции в носителе сессии
 * (rental-terms-edit-session). «Начало аренды» — read-only (ADR 0053 §3:
 * «Начало не правится», PATCH startDate не принимает; иконки календаря
 * у поля нет). Email-ряд — NotifyEmailRow (карта #1551, тикет #1557):
 * свитч — персональная галочка почты аренды (`rentPayment.notifyEmail`,
 * сервер держит флаг на Платеже и переносит при правке, #1555),
 * предзаполнение из флага аренды; глобальный мастер категории «Аренда»
 * решает видимость ряда вовсе — общий предикат с компонентом ряда решает
 * и видимость, и «едет ли notifyEmail в PATCH» (при off и сбое — дифф
 * без поля, канон #1552); гейта от пикера напоминаний у аренды нет —
 * ряд независим (решение владельца 22.10.2026). Окончание —
 * канонический бесконечный календарь: снятая
 * дата — бессрочная (tri-state null), будущие дни открыты, дни ≤ начала
 * и дни до первого вхождения дня оплаты погашены (minDate, #1156 —
 * валидатор слитой пары на сервере, #1154); смена дня оплаты молча чистит
 * окончание, переставшее покрывать новое первое вхождение. Сумма, день
 * оплаты, автоплатёж, напоминание, галочка почты и окончание сервер
 * синхронно переносит на Платёж арендной платы (#1208, #1555) — фронт
 * шлёт дифф формы (edit-model) одним PATCH. Сохранение — тост и возврат
 * на «Условия аренды».
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
        <RentalTermsEditFormSkeleton />
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

/** Оболочка состояний без формы: шапка с крестиком и титулом + контент
 * страницы. Титул рендерится вне фазы загрузки (§7): состав шапки не
 * меняется при приходе данных — у формы добавляется только галочка
 * сохранения. */
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
      >
        <TopNavTitle title="Изменить условия" />
      </TopNav>
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

  // Ряд «Уведомления на почту» (карта #1551, тикет #1557): подпись с
  // почтой пользователя, канон шага создания; свитч — персональная
  // галочка этой аренды, предзаполнение из её флага. Мастер категории
  // здесь только читается: общий предикат с компонентом ряда решает и
  // видимость, и «едет ли notifyEmail в PATCH» (при off и сбое — дифф
  // без поля, канон #1552).
  const meQuery = useMe();
  const email = meQuery.data?.email ?? null;
  const emailMasterQuery = useEmailNotificationPreferences();
  const emailRowVisible = emailNotificationsRowVisible(
    emailMasterQuery.isPending,
    emailMasterQuery.data?.[RENTAL_EMAIL_CATEGORY],
  );

  // Форма переживает круговые маршруты секции «Контакт арендатора»
  // (решение владельца 28.10.2026, #1623): снимок хранится в носителе
  // сессии, при возврате восстанавливается, гостевой контакт (пикер,
  // ветвь создания) потребляется один раз. useState-инициализатор чистый
  // (StrictMode зовёт его дважды), ожидание снимает эффект ниже.
  const [form, setForm] = useState<RentalEditForm>(() => {
    const { form: saved, pendingContactId } = readRentalTermsEditSessionDraft(rental.propertyId);
    const base = saved ?? rentalEditFormFromRental(rental);
    return pendingContactId === null ? base : { ...base, contactId: pendingContactId };
  });
  const propertyId = rental.propertyId;
  useEffect(() => {
    consumeRentalTermsEditSessionContact(propertyId);
    return openRentalTermsEditSession(propertyId);
  }, [propertyId]);
  // Уход в под-маршрут секции: флаг живости и полный снимок формы —
  // синхронно в клике, до router.push (канон носителя визарда #1159).
  const openSubRoute = (): void => {
    keepRentalTermsEditSessionAlive(rental.propertyId);
    setRentalTermsEditSessionForm(rental.propertyId, form);
  };
  // «Сырые» набранные значения денег — источник отображения (группировка
  // не сбрасывает каретку); копейки едут в форму для диффа и валидации.
  // «Сырые» набранные значения денег — источник отображения; на старте
  // читают форму, а не аренду: восстановленный из носителя снимок (возврат
  // из под-маршрута секции) должен показаться целиком, иначе правка денег
  // жива в диффе, а на экране старое значение (находка приёмки #1623).
  const [amountRaw, setAmountRaw] = useState(() =>
    form.amountKopecks === undefined ? '' : kopecksToAmountInputString(form.amountKopecks),
  );
  const [depositRaw, setDepositRaw] = useState(() =>
    form.depositKopecks === null ? '' : kopecksToAmountInputString(form.depositKopecks),
  );
  const [commissionRaw, setCommissionRaw] = useState(() =>
    form.commissionKopecks === null ? '' : kopecksToAmountInputString(form.commissionKopecks),
  );

  const [dayPickerOpen, setDayPickerOpen] = useState(false);
  const [endPickerOpen, setEndPickerOpen] = useState(false);

  const update = <K extends keyof RentalEditForm>(key: K, value: RentalEditForm[K]): void =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const command = buildRentalUpdateCommand(rental, form, { notifyEmailRowVisible: emailRowVisible });
  const canSave = command !== undefined;
  // Эффективный день оплаты (#1156): выбранный в форме, иначе день платежа
  // аренды — от него считается минимум пикера окончания и сброс.
  const effectiveDay = form.paymentDay ?? rental.rentPayment.paymentDay;
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
      clearRentalTermsEditSessionDraft(rental.propertyId);
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

          <ReminderPickerField
            value={form.reminderOffsetDays}
            onChange={(offset) => update('reminderOffsetDays', offset)}
          />

          {/* Ряд почты — после пикера напоминания (канон #1552): свитч —
              персональная галочка этой аренды, гейта от пикера нет. */}
          <NotifyEmailRow
            category={RENTAL_EMAIL_CATEGORY}
            title="Включить уведомления об оплате на почту"
            caption={emailReminderCaption('об оплате', email)}
            checked={form.notifyEmail}
            onCheckedChange={(notifyEmail) => update('notifyEmail', notifyEmail)}
          />

          {/* Секция арендатора (решение владельца 28.10.2026, #1623):
              между рядом почты и комментарием — порядок макета
              40000101:153003; смена едет общим диффом формы. */}
          <TermsContactSection
            propertyId={rental.propertyId}
            contactId={form.contactId}
            onContactChange={(contactId) => update('contactId', contactId)}
            onSubRouteOpen={openSubRoute}
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
              // Смена дня молча чистит окончание, переставшее покрывать
              // первое вхождение (#1156) — валидатор слитой пары на
              // сервере иначе отклонил бы PATCH.
              setForm((prev) => formAfterPaymentDayChange(rental, prev, day));
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
          minDate={plannedEndDateMinDate(rental.startDate, effectiveDay)}
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
