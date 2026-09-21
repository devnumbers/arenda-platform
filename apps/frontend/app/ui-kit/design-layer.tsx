'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import {
    ArrowDown,
    ArrowLeft,
    BoldHome,
    BoldPerson,
    BoldSofa,
    ChangeVertical,
    Edit,
    Move,
    Search,
    SortingBigSmall,
    SortingSmallBig,
    Star,
    StarOff,
} from '@/shared/assets/icons';
import type { IsoRange } from '@/shared/lib/calendar';
import {
    AmountField,
    Button,
    CalendarButton,
    CalendarDatePicker,
    CalendarMonth,
    CalendarRangePicker,
    Checkbox,
    ChipButton,
    ConfirmDialog,
    DesktopMenuButton,
    EmptyState,
    HubTitle,
    SubScreenShell,
    IconButton,
    InfiniteQueryTail,
    ListRow,
    Modal,
    ModalClose,
    ModalContent,
    ModalTrigger,
    MoreSheet,
    MonthDaysGrid,
    MonthYearPicker,
    PageContent,
    PickerMenu,
    PickerField,
    Skeleton,
    SkeletonButton,
    SkeletonCard,
    SkeletonFormField,
    SkeletonListRow,
    SkeletonMedia,
    SkeletonSection,
    RadioGroup,
    RadioGroupItem,
    ResendCodeTile,
    SearchField,
    StatusIcon,
    SuccessPopup,
    SupportModal,
    UserButton,
    StepsChip,
    StickyBottomBar,
    Switch,
    Textarea,
    TextField,
    TopNav,
    TopNavBackButton,
    TopNavTitle,
    WheelPicker,
    WheelPickerSheet,
    amountKopecks,
    monthTitle,
    type PickerOption,
    type WheelPickerItem,
} from '@/shared/ui/design';
import { navSectionById, supportNavSection } from '@/shared/config/navigation';
import { addDays, dateToIso } from '@/shared/lib/calendar';
import {
    CategoryIcon,
    categoryStyle,
    type PaymentCategoryEntry,
    paymentCategories,
} from '@/features/payment-categories';
import {
    PaymentCardButton,
    PaymentRowButton,
    formatOverdueDays,
} from '@/entities/payment';
import styles from './page.module.css';

const dlButtonVariants = ['primary', 'secondary', 'danger', 'clear', 'white'] as const;
const dlIconVariants = ['primary', 'secondary', 'danger'] as const;
const statusGradations = ['danger', 'warning', 'good', 'check', 'info'] as const;

/** Колёса демо-шита WheelPickerSheet: часы 00–23 и минуты 00–59. */
const sheetHourItems: ReadonlyArray<WheelPickerItem> = Array.from({ length: 24 }, (_, value) => ({
    value: String(value),
    label: String(value).padStart(2, '0'),
}));
const sheetMinuteItems: ReadonlyArray<WheelPickerItem> = Array.from({ length: 60 }, (_, value) => ({
    value: String(value),
    label: String(value).padStart(2, '0'),
}));

/** Витринные категории из сгенерированного каталога #447. */
const showcaseCategorySlugs = [
    'rent',
    'utilities',
    'mortgage',
    'insurance',
    'damage-compensation',
    'late-fees',
] as const;

const showcaseCategories = showcaseCategorySlugs
    .map((slug) => paymentCategories.find((entry) => entry.slug === slug))
    .filter((entry) => entry !== undefined);

const showcaseBySlug = new Map<string, PaymentCategoryEntry>(
    showcaseCategories.map((entry) => [entry.slug, entry]),
);

/** Витрина демо-данными каталога: слаг гарантированно входит в набор выше. */
function showcaseEntry(slug: string): PaymentCategoryEntry {
    const entry = showcaseBySlug.get(slug);
    if (entry === undefined) {
        throw new Error(`showcase: в каталоге нет категории ${slug}`);
    }
    return entry;
}

/** Витринные объекты пикера (#505): с домашней иконкой в сером круге,
 * как строка «Объект» на карточке контакта. */
function ObjectAvatar(): JSX.Element {
    return (
        <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted text-content">
            <BoldHome className="h-6 w-6" />
        </span>
    );
}

const showcaseObjects: ReadonlyArray<PickerOption> = [
    { value: 'kv-1', label: 'Моя квартира', hint: 'Новаторов, 8', icon: <ObjectAvatar /> },
    { value: 'kv-2', label: 'Квартира на набережной', hint: 'Набережная, 15', icon: <ObjectAvatar /> },
    { value: 'kv-3', label: 'Дача', hint: 'Приозёрная, 2', icon: <ObjectAvatar /> },
];

/** Заглушка запроса для витрины хвоста: фаза «едет следующая порция» —
 * индикатор виден постоянно, fetchNextPage никуда не ходит. */
const STUB_FETCHING_QUERY = {
    hasNextPage: true,
    isFetchingNextPage: true,
    fetchNextPage: () => Promise.resolve(),
} as const;

/** Холостая фаза той же ленты: продолжение есть (sentinel в DOM), порция
 * не едет — хвост безмолвен, пока sentinel не войдёт во вьюпорт. */
const STUB_IDLE_QUERY = {
    hasNextPage: true,
    isFetchingNextPage: false,
    fetchNextPage: () => Promise.resolve(),
} as const;

/** Витрина дизайн-слоя (ADR 0050, тикет #455): шадкн/ui поверх Radix,
 * Tailwind на токенах, шрифт Onest. Внешний вид сверен с экспортами
 * Figma-фреймов «Рентли. Новые экраны сервиса» (node-id — резолюция #449). */
export function DesignLayerShowcase(): JSX.Element {
    const [nameValue, setNameValue] = useState('');
    const [nameInValue, setNameInValue] = useState('');
    const [titleInValue, setTitleInValue] = useState('');
    const [titleInFilled, setTitleInFilled] = useState('Аренда, январь');
    const [titleInError, setTitleInError] = useState('Страховка');
    const [limitedValue, setLimitedValue] = useState('Страхование квартиры');
    const [searchValue, setSearchValue] = useState('');
    const [searchFilled, setSearchFilled] = useState('аренд');
    const [checked, setChecked] = useState(true);
    const [unchecked, setUnchecked] = useState(false);
    const [radio, setRadio] = useState('payments');
    const [autoPay, setAutoPay] = useState(true);
    const [paused, setPaused] = useState(false);
    const [chip, setChip] = useState('name');
    const [calendarYear, setCalendarYear] = useState(2026);
    const [calendarMonth, setCalendarMonth] = useState(7);
    const [selectedDate, setSelectedDate] = useState<Date | undefined>(new Date(2026, 7, 17));
    const [wheelOpen, setWheelOpen] = useState(false);
    const [wheelSheetOpen, setWheelSheetOpen] = useState(false);
    const [wheelSheetValue, setWheelSheetValue] = useState('09:00');
    const [sheetHour, setSheetHour] = useState('09');
    const [sheetMinute, setSheetMinute] = useState('00');
    const [datePickerOpen, setDatePickerOpen] = useState(false);
    const [datePickerValue, setDatePickerValue] = useState<string | null>(null);
    const [datePickerRequiredOpen, setDatePickerRequiredOpen] = useState(false);
    const [datePickerRequiredValue, setDatePickerRequiredValue] = useState<string | null>(null);
    const [datePickerMinOpen, setDatePickerMinOpen] = useState(false);
    const [datePickerMinValue, setDatePickerMinValue] = useState<string | null>(null);
    const [datePickerMaxOpen, setDatePickerMaxOpen] = useState(false);
    const [datePickerMaxValue, setDatePickerMaxValue] = useState<string | null>(null);
    const [rangePickerOpen, setRangePickerOpen] = useState(false);
    const [rangePickerValue, setRangePickerValue] = useState<IsoRange | null>(null);
    const [rangePickerEmptyOpen, setRangePickerEmptyOpen] = useState(false);
    const [rangePickerEmptyValue, setRangePickerEmptyValue] = useState<IsoRange | null>(null);
    const [monthDays, setMonthDays] = useState<ReadonlySet<number>>(new Set([10]));
    const [lastDayOfMonth, setLastDayOfMonth] = useState(false);
    const [amount, setAmount] = useState('');
    const [paymentForm, setPaymentForm] = useState<'transfer' | 'cash'>('transfer');
    const [direction, setDirection] = useState<'income' | 'expense'>('income');
    const [note, setNote] = useState('');
    const [noteError, setNoteError] = useState('Страховка не оформлена');
    const [pickerValue, setPickerValue] = useState<string | null>('kv-1');
    const [pickerEmpty, setPickerEmpty] = useState<string | null>(null);
    const [confirmOpen, setConfirmOpen] = useState(false);
    const [deleteOpen, setDeleteOpen] = useState(false);
    const [confirmLargeOpen, setConfirmLargeOpen] = useState(false);
    const [deletePropertyOpen, setDeletePropertyOpen] = useState(false);
    const [successPopupOpen, setSuccessPopupOpen] = useState(false);
    const [moreSheetOpen, setMoreSheetOpen] = useState(false);
    const [supportModalOpen, setSupportModalOpen] = useState(false);

    return (
        <>
            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Новые компоненты — дизайн-слой (ADR 0050)</h2>
                <p className={styles.groupTitle}>
                    shadcn/ui поверх Radix · Tailwind на токенах · шрифт Onest — источник правды для новых экранов
                </p>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>Button</h3>
                    <div className={styles.grid}>
                        {dlButtonVariants.map((variant) => (
                            <Button key={variant} variant={variant}>
                                {variant}
                            </Button>
                        ))}
                        <Button loading>loading</Button>
                        <Button disabled>disabled</Button>
                        <Button variant="primary" leadingIcon={<BoldHome />} trailingIcon={<Edit />}>
                            with icons
                        </Button>
                    </div>
    <div className={styles.grid}>
        {dlButtonVariants.map((variant) => (
            <Button key={variant} variant={variant} size="small">
                small
            </Button>
        ))}
        <Button variant="secondary" size="small" selected>
            selected
        </Button>
        <Button size="small" loading>
            loading
        </Button>
        <Button size="small" disabled>
            disabled
        </Button>
        {/* Радиус m (12px, токен Figma radius/m) — CTA пустых состояний
         * секций объекта (#588, решение владельца 11.09). */}
        {dlButtonVariants.map((variant) => (
            <Button key={`m-${variant}`} variant={variant} size="small" radius="m">
                small · m
            </Button>
        ))}
        <Button variant="primary" radius="m">
            default · m
        </Button>
    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>IconButton</h3>
                    <div className={styles.grid}>
                        {dlIconVariants.map((variant) => (
                            <IconButton key={variant} variant={variant} icon={<ArrowLeft />} label={`Назад ${variant}`} />
                        ))}
                        <IconButton variant="primary" icon={<Edit />} label="Редактировать" disabled />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>TextField · Title Out</h3>
                    <div className={styles.textFields}>
                        <TextField
                            title="Название платежа"
                            required
                            placeholder="Название платежа"
                            description="Необязательно"
                            maxLength={256}
                            value={nameValue}
                            onChange={(event) => setNameValue(event.target.value)}
                            onClear={() => setNameValue('')}
                        />
                        <TextField
                            title="Название платежа"
                            error="Ошибка"
                            maxLength={256}
                            value={nameInValue}
                            onChange={(event) => setNameInValue(event.target.value)}
                            onClear={() => setNameInValue('')}
                        />
                        <TextField
                            title="Название платежа"
                            maxLength={32}
                            value={limitedValue}
                            onChange={(event) => setLimitedValue(event.target.value)}
                            onClear={() => setLimitedValue('')}
                        />
                        <TextField title="Название платежа" placeholder="Название платежа" disabled />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>TextField · Title In (плавающий лейбл)</h3>
                    <div className={styles.textFields}>
                        <TextField
                            variant="titleIn"
                            title="Название платежа"
                            required
                            maxLength={256}
                            value={titleInValue}
                            onChange={(event) => setTitleInValue(event.target.value)}
                            onClear={() => setTitleInValue('')}
                        />
                        <TextField
                            variant="titleIn"
                            title="Название платежа"
                            maxLength={256}
                            value={titleInFilled}
                            onChange={(event) => setTitleInFilled(event.target.value)}
                            onClear={() => setTitleInFilled('')}
                        />
                        <TextField
                            variant="titleIn"
                            title="Название платежа"
                            error="Ошибка"
                            maxLength={256}
                            value={titleInError}
                            onChange={(event) => setTitleInError(event.target.value)}
                            onClear={() => setTitleInError('')}
                        />
                        <TextField variant="titleIn" title="Название платежа" disabled />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>Checkbox · Radio · Switch</h3>
                    <div className={styles.grid}>
                        <div className={styles.links}>
                            <Checkbox
                                id="dl-checkbox-on"
                                checked={checked}
                                onCheckedChange={(value) => setChecked(value === true)}
                            />
                            <label htmlFor="dl-checkbox-on">Автоплатёж включён</label>
                        </div>
                        <div className={styles.links}>
                            <Checkbox
                                id="dl-checkbox-off"
                                checked={unchecked}
                                onCheckedChange={(value) => setUnchecked(value === true)}
                            />
                            <label htmlFor="dl-checkbox-off">Не выбран</label>
                        </div>
                        <RadioGroup value={radio} onValueChange={setRadio}>
                            <div className={styles.links}>
                                <RadioGroupItem id="dl-radio-payments" value="payments" />
                                <label htmlFor="dl-radio-payments">Платежи</label>
                            </div>
                            <div className={styles.links}>
                                <RadioGroupItem id="dl-radio-auto" value="auto" />
                                <label htmlFor="dl-radio-auto">Автоплатежи</label>
                            </div>
                        </RadioGroup>
                        <div className={styles.links}>
                            <Switch id="dl-switch-auto" checked={autoPay} onCheckedChange={setAutoPay} />
                            <label htmlFor="dl-switch-auto">Автоплатёж</label>
                        </div>
                        <div className={styles.links}>
                            <Switch id="dl-switch-pause" checked={paused} onCheckedChange={setPaused} />
                            <label htmlFor="dl-switch-pause">Пауза</label>
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>SearchField</h3>
                    <div className={styles.column} style={{ maxWidth: 420 }}>
                        <SearchField
                            placeholder="Поиск операций"
                            value={searchValue}
                            onChange={(event) => setSearchValue(event.target.value)}
                            onClear={() => setSearchValue('')}
                        />
                        <SearchField
                            placeholder="Поиск операций"
                            value={searchFilled}
                            onChange={(event) => setSearchFilled(event.target.value)}
                            onClear={() => setSearchFilled('')}
                        />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>ChipButton</h3>
                    <div className={styles.grid}>
                        <ChipButton
                            leadingIcon={<SortingSmallBig />}
                            trailingIcon={<ArrowDown />}
                            selected={chip === 'name'}
                            onClick={() => setChip('name')}
                        >
                            По названию
                        </ChipButton>
                        <ChipButton
                            leadingIcon={<SortingBigSmall />}
                            trailingIcon={<ArrowDown />}
                            selected={chip === 'amount'}
                            onClick={() => setChip('amount')}
                        >
                            По сумме
                        </ChipButton>
                        <ChipButton trailingIcon={<ArrowDown />} disabled>
                            disabled
                        </ChipButton>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>PickerMenu · меню / шит опций</h3>
                    <p className={styles.groupTitle}>
                        Адаптивный выбор опций: на десктопе — меню-карточка, на мобиле — нижний шит
                        (Figma 1603-94487 / 1535-76225). Триггер — любая кнопка (children).
                    </p>
                    <div className={styles.grid}>
                        <PickerMenu
                            title="Сортировать"
                            groups={[
                                {
                                    options: [
                                        { label: 'По дате создания', selected: true, onSelect: () => {} },
                                        { label: 'По названию', selected: false, onSelect: () => {} },
                                    ],
                                },
                                {
                                    options: [
                                        { label: 'Возрастание', selected: false, onSelect: () => {} },
                                        { label: 'Убывание', selected: false, onSelect: () => {} },
                                    ],
                                },
                            ]}
                        >
                            <ChipButton trailingIcon={<ArrowDown />}>Сортировать</ChipButton>
                        </PickerMenu>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>StepsChip</h3>
                    <div className={styles.grid}>
                        <StepsChip step={1} total={5} />
                        <StepsChip step={2} total={6} size="m" />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>StatusIcon</h3>
                    <div className={styles.grid}>
                        {statusGradations.map((status) => (
                            <StatusIcon key={status} status={status} />
                        ))}
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>UserButton</h3>
                    <div className={styles.links}>
                        <UserButton name="Даниил" />
                        <UserButton name="Профиль" disabled />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>CalendarButton</h3>
                    <div className={styles.links}>
                        {[26, 27, 28, 29, 30, 31, 1].map((day, i) => (
                            <CalendarButton
                                key={day}
                                state={i === 2 ? 'today' : i === 4 ? 'selected' : 'default'}
                                disabled={i === 6}
                            >
                                {day}
                            </CalendarButton>
                        ))}
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>CalendarMonth · выбор даты + крутилка месяц/год</h3>
                    <p className={styles.groupTitle}>
                        Один выбранный месяц, без бесконечной сетки; месяц и год — колёсами в шите, как в
                        таймере Apple (Figma 835:20007, 848:8720).
                    </p>
                    <div className={styles.column}>
                        <div className="flex gap-1.5 px-6 pb-2">
                            <ChipButton trailingIcon={<ArrowDown />} onClick={() => setWheelOpen(true)}>
                                {monthTitle(calendarYear, calendarMonth)}
                            </ChipButton>
                        </div>
                        <CalendarMonth
                            year={calendarYear}
                            month={calendarMonth}
                            value={selectedDate}
                            today={new Date()}
                            onDateSelect={setSelectedDate}
                        />
                    </div>
                    <MonthYearPicker
                        open={wheelOpen}
                        onOpenChange={setWheelOpen}
                        month={calendarMonth}
                        year={calendarYear}
                        onConfirm={(month, year) => {
                            setCalendarMonth(month);
                            setCalendarYear(year);
                            // выбранная дата жила в старом месяце —
                            // в новом блоке ничего не выбрано
                            if (
                                selectedDate !== undefined &&
                                (selectedDate.getMonth() !== month || selectedDate.getFullYear() !== year)
                            ) {
                                setSelectedDate(undefined);
                            }
                            setWheelOpen(false);
                        }}
                    />
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>WheelPickerSheet · универсальный шит колёс</h3>
                    <p className={styles.groupTitle}>
                        Любое число колёс + передаваемые кнопки (Figma 1539-82659); на мобиле —
                        выезжающий шит (vaul), на десктопе — карточка.
                    </p>
                    <div className={styles.column}>
                        <Button onClick={() => setWheelSheetOpen(true)}>Открыть шит колёс</Button>
                        <p className="px-6 text-base text-content-secondary">
                            Выбрано: {wheelSheetValue}
                        </p>
                    </div>
                    <WheelPickerSheet
                        title="Время"
                        open={wheelSheetOpen}
                        onOpenChange={setWheelSheetOpen}
                        actions={[
                            {
                                label: 'Отменить',
                                variant: 'secondary',
                                onSelect: () => setWheelSheetOpen(false),
                            },
                            {
                                label: 'Выбрать',
                                onSelect: () => {
                                    setWheelSheetValue(`${sheetHour}:${sheetMinute}`);
                                    setWheelSheetOpen(false);
                                },
                            },
                        ]}
                        columns={[
                            <WheelPicker
                                key="hour"
                                label="Часы"
                                strip={false}
                                items={sheetHourItems}
                                value={sheetHour}
                                onValueChange={setSheetHour}
                            />,
                            <WheelPicker
                                key="minute"
                                label="Минуты"
                                strip={false}
                                items={sheetMinuteItems}
                                value={sheetMinute}
                                onValueChange={setSheetMinute}
                            />,
                        ]}
                    />
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>CalendarDatePicker · бесконечный пикер даты</h3>
                    <p className={styles.groupTitle}>
                        Полноэкранный: чип месяца и дни недели закреплены над прокруткой, лента месяцев
                        бесконечно вперёд без подвисаний (общий компонент из пикера задач #500).
                        «Выбрать» скрыта, пока нечего подтвердить, проп minDate гасит дни ≤ минимума —
                        решение владельца 2026-09-05.
                    </p>
                    <div className={styles.column}>
                        <Button onClick={() => setDatePickerOpen(true)}>Открыть пикер даты</Button>
                        {datePickerValue !== null && (
                            <p className="px-6 text-base text-content-secondary">
                                Выбрано: {datePickerValue}
                            </p>
                        )}
                    </div>
                    {datePickerOpen && (
                        <CalendarDatePicker
                            today={dateToIso(new Date())}
                            value={datePickerValue}
                            onClose={() => setDatePickerOpen(false)}
                            onConfirm={(date) => {
                                setDatePickerValue(date);
                                setDatePickerOpen(false);
                            }}
                        />
                    )}
                    <div className={styles.column}>
                        <Button onClick={() => setDatePickerRequiredOpen(true)}>
                            Открыть пикер (required)
                        </Button>
                        {datePickerRequiredValue !== null && (
                            <p className="px-6 text-base text-content-secondary">
                                Выбрано: {datePickerRequiredValue}
                            </p>
                        )}
                    </div>
                    {datePickerRequiredOpen && (
                        <CalendarDatePicker
                            required
                            today={dateToIso(new Date())}
                            value={datePickerRequiredValue}
                            onClose={() => setDatePickerRequiredOpen(false)}
                            onConfirm={(date) => {
                                setDatePickerRequiredValue(date);
                                setDatePickerRequiredOpen(false);
                            }}
                        />
                    )}
                    <div className={styles.column}>
                        <Button onClick={() => setDatePickerMinOpen(true)}>
                            Открыть пикер (minDate: +10 дней)
                        </Button>
                        {datePickerMinValue !== null && (
                            <p className="px-6 text-base text-content-secondary">
                                Выбрано: {datePickerMinValue}
                            </p>
                        )}
                    </div>
                    {datePickerMinOpen && (
                        <CalendarDatePicker
                            minDate={addDays(dateToIso(new Date()), 10)}
                            today={dateToIso(new Date())}
                            value={datePickerMinValue}
                            onClose={() => setDatePickerMinOpen(false)}
                            onConfirm={(date) => {
                                setDatePickerMinValue(date);
                                setDatePickerMinOpen(false);
                            }}
                        />
                    )}
                    <div className={styles.column}>
                        <Button onClick={() => setDatePickerMaxOpen(true)}>
                            Открыть пикер (maxDate: −10 дней)
                        </Button>
                        {datePickerMaxValue !== null && (
                            <p className="px-6 text-base text-content-secondary">
                                Выбрано: {datePickerMaxValue}
                            </p>
                        )}
                    </div>
                    {datePickerMaxOpen && (
                        <CalendarDatePicker
                            maxDate={addDays(dateToIso(new Date()), -10)}
                            today={dateToIso(new Date())}
                            value={datePickerMaxValue}
                            onClose={() => setDatePickerMaxOpen(false)}
                            onConfirm={(date) => {
                                setDatePickerMaxValue(date);
                                setDatePickerMaxOpen(false);
                            }}
                        />
                    )}
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>CalendarRangePicker · пикер периода</h3>
                    <p className={styles.groupTitle}>
                        Диапазон дат: лента назад без предела (дорисовка при прокрутке вверх), будущее
                        закрыто; поля «с …/по …» следуют за тапами. Чип «Месяц Год ⌄» — опциональный
                        проп monthJump (по умолчанию показан); на фильтре периода операций скрыт
                        (решение владельца 2026-09-05), здесь — как в продукте, без него. Решение
                        владельца 2026-09-04.
                    </p>
                    <div className={styles.column}>
                        <Button onClick={() => setRangePickerOpen(true)}>Открыть пикер периода</Button>
                        {rangePickerValue !== null && (
                            <p className="px-6 text-base text-content-secondary">
                                Выбрано: с {rangePickerValue.from} по {rangePickerValue.to}
                            </p>
                        )}
                    </div>
                    {rangePickerOpen && (
                        <CalendarRangePicker
                            today={dateToIso(new Date())}
                            monthJump={false}
                            value={
                                rangePickerValue ?? {
                                    from: dateToIso(new Date()),
                                    to: dateToIso(new Date()),
                                }
                            }
                            onClose={() => setRangePickerOpen(false)}
                            onConfirm={(range) => {
                                setRangePickerValue(range);
                                setRangePickerOpen(false);
                            }}
                        />
                    )}
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>CalendarRangePicker · пустой старт и сброс</h3>
                    <p className={styles.groupTitle}>
                        Без применённого периода (#670, решение владельца 2026-09-14): value не
                        задан — ничего не предвыбрано, поля «с …/по …» — плейсхолдеры, «Выбрать»
                        активна только при выборе; опциональный onReset рисует «Сбросить» рядом
                        с «Выбрать» — потребитель решает, что сброс означает.
                    </p>
                    <div className={styles.column}>
                        <Button onClick={() => setRangePickerEmptyOpen(true)}>
                            Открыть пикер (пустой старт)
                        </Button>
                        {rangePickerEmptyValue !== null && (
                            <p className="px-6 text-base text-content-secondary">
                                Выбрано: с {rangePickerEmptyValue.from} по {rangePickerEmptyValue.to}
                            </p>
                        )}
                    </div>
                    {rangePickerEmptyOpen && (
                        <CalendarRangePicker
                            today={dateToIso(new Date())}
                            monthJump={false}
                            value={rangePickerEmptyValue}
                            onClose={() => setRangePickerEmptyOpen(false)}
                            onConfirm={(range) => {
                                setRangePickerEmptyValue(range);
                                setRangePickerEmptyOpen(false);
                            }}
                            onReset={() => {
                                setRangePickerEmptyValue(null);
                                setRangePickerEmptyOpen(false);
                            }}
                        />
                    )}
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>MonthDaysGrid · «Каждый месяц»</h3>
                    <p className={styles.groupTitle}>
                        Мини-грид дней 1..N с нескольких выбранными + опция «Последний день месяца»
                        (Figma 823:11422).
                    </p>
                    <div className={styles.column}>
                        <MonthDaysGrid
                            days={30}
                            selectedDays={monthDays}
                            onDayToggle={(day) =>
                                setMonthDays((prev) => {
                                    const next = new Set(prev);
                                    if (next.has(day)) {
                                        next.delete(day);
                                    } else {
                                        next.add(day);
                                    }
                                    return next;
                                })
                            }
                        />
                        <ListRow
                            title="Последний день месяца"
                            trailing={
                                <Checkbox
                                    id="dl-last-day"
                                    checked={lastDayOfMonth}
                                    onCheckedChange={(value) => setLastDayOfMonth(value === true)}
                                />
                            }
                        />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>AmountField · ввод суммы</h3>
                    <p className={styles.groupTitle}>
                        Ввод с клавиатуры, только цифры и запятая (маска до 9 999 999,99 ₽); под суммой
                        — две кнопки: клик меняет их значение, без всплывающих окон (Figma 834:19662
                        «Перевод/Доход» → 835:19795 «Наличные/Расход»). Кнопка заблокирована, пока
                        сумма не введена.
                    </p>
                    <div className={styles.column} style={{ maxWidth: 560 }}>
                        <AmountField value={amount} onChange={setAmount} label="Сумма" />
                        {/* одинаковая ширина обеих кнопок: при переключении
                            значений (Перевод↔Наличные, Доход↔Расход) вёрстка
                            не дёргается */}
                        <div className="flex justify-center gap-2">
                            <ChipButton
                                className="w-40"
                                trailingIcon={<ChangeVertical />}
                                aria-label={`Форма оплаты: ${paymentForm === 'transfer' ? 'Перевод' : 'Наличные'}. Нажмите, чтобы переключить`}
                                onClick={() => setPaymentForm((prev) => (prev === 'transfer' ? 'cash' : 'transfer'))}
                            >
                                {paymentForm === 'transfer' ? 'Перевод' : 'Наличные'}
                            </ChipButton>
                            <ChipButton
                                className="w-40"
                                trailingIcon={<ChangeVertical />}
                                aria-label={`Направление: ${direction === 'income' ? 'Доход' : 'Расход'}. Нажмите, чтобы переключить`}
                                onClick={() => setDirection((prev) => (prev === 'income' ? 'expense' : 'income'))}
                            >
                                {direction === 'income' ? 'Доход' : 'Расход'}
                            </ChipButton>
                        </div>
                        <Button disabled={amountKopecks(amount) === 0}>Создать платеж</Button>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>ListRow</h3>
                    <div className={styles.column} style={{ maxWidth: 480 }}>
                        <ListRow
                            leading={
                                <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-[#FF8904] text-white shadow-[0_0_0_2.5px_#ffffff]">
                                    <BoldSofa className="h-6 w-6" />
                                </span>
                            }
                            title="Аренда стола"
                            subtitle="Моя квартира"
                            subtitleIcon={<Star />}
                            value="2 500 ₽"
                            description="11 сентября"
                            onSelect={() => undefined}
                        />
                        <ListRow
                            leading={
                                <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted text-content">
                                    <BoldSofa className="h-6 w-6" />
                                </span>
                            }
                            title="Аренда стола"
                            subtitle="Моя квартира"
                            subtitleIcon={<StarOff />}
                            value="2 500 ₽"
                            description="11 сентября"
                            trailing={<IconButton icon={<Move />} label="Переставить" />}
                        />
                        <ListRow
                            leading={<StatusIcon status="danger" />}
                            title="Просроченный платёж"
                            subtitle="2 дня"
                            value="32 000 ₽"
                        />
                        <ListRow
                            title="Екатеринбург (UTC+5)"
                            titleClassName="text-primary"
                            onSelect={() => undefined}
                        />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>CategoryIcon — доменный композит категорий (#447)</h3>
                    <p className={styles.groupTitle}>
                        Круг 44×44 с цветом подложки каталога, кантом по поверхности и bold-иконкой;
                        бейдж danger — просрочка на верхнем левом углу (Figma 651:5925).
                    </p>
                    <div className={styles.grid}>
                        {showcaseCategories.map((entry) => (
                            <CategoryIcon key={entry.slug} icon={entry.icon} color={entry.color} />
                        ))}
                        <CategoryIcon
                            icon={showcaseEntry('late-fees').icon}
                            color={showcaseEntry('late-fees').color}
                            badge="danger"
                        />
                        <CategoryIcon {...categoryStyle('custom')} surface="muted" />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>PaymentRowButton · PaymentCardButton — композиты платежей (#462)</h3>
                    <p className={styles.groupTitle}>
                        Строка операции (White/Gray) и карточка просроченного 168.5px; danger красит
                        сумму и срок (#452). Слева — CategoryIcon с серым кантом на серой плитке.
                    </p>
                    <div className={styles.column} style={{ maxWidth: 480 }}>
                        <PaymentRowButton
                            categoryIcon={
                                <CategoryIcon
                                    icon={showcaseEntry('rent').icon}
                                    color={showcaseEntry('rent').color}
                                />
                            }
                            title="Арендная плата"
                            subtitle="Моя квартира"
                            subtitleIcon={<Star />}
                            amountKopecks={5600000}
                            description="11 сентября"
                            onSelect={() => undefined}
                        />
                        <PaymentRowButton
                            variant="gray"
                            leading={<IconButton icon={<StarOff />} label="Избранное" />}
                            categoryIcon={
                                <CategoryIcon
                                    icon={showcaseEntry('damage-compensation').icon}
                                    color={showcaseEntry('damage-compensation').color}
                                    badge="danger"
                                    surface="muted"
                                />
                            }
                            title="Возмещение ущерба"
                            subtitle={formatOverdueDays(4)}
                            amountKopecks={320000}
                            description="17 августа"
                            danger
                            onSelect={() => undefined}
                        />
                        <div className="flex gap-2 overflow-x-auto py-1">
                            <PaymentCardButton
                                leading={
                                    <CategoryIcon
                                        icon={showcaseEntry('insurance').icon}
                                        color={showcaseEntry('insurance').color}
                                        surface="muted"
                                    />
                                }
                                title="Страхование квартиры в центре города"
                                subtitle="Моя квартира в центре"
                                amountKopecks={3200000}
                                description="17 августа"
                                onSelect={() => undefined}
                            />
                            <PaymentCardButton
                                leading={
                                    <CategoryIcon
                                        icon={showcaseEntry('late-fees').icon}
                                        color={showcaseEntry('late-fees').color}
                                        badge="danger"
                                        surface="muted"
                                    />
                                }
                                title="Пени по аренде"
                                subtitle="Моя квартира"
                                amountKopecks={850000}
                                description={formatOverdueDays(3)}
                                danger
                                onSelect={() => undefined}
                            />
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>TopNav</h3>
                    <div className={styles.column} style={{ maxWidth: 480 }}>
                        <TopNav
                            leading={
                                <span className="px-5 text-lg font-bold text-primary">Рентли</span>
                            }
                            trailing={
                                <span className="flex items-center gap-3 pr-2">
                                    <span className="text-sm font-medium text-content">Даниил</span>
                                    <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted text-content">
                                        <BoldPerson className="h-6 w-6" />
                                    </span>
                                </span>
                            }
                        />
                        <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
                            <TopNavTitle title="Избранные платежи" subtitle="На паузе" />
                        </TopNav>
                        <TopNav
                            leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
                            trailing={<IconButton icon={<Search />} label="Поиск" />}
                        >
                            <StepsChip step={1} total={5} size="m" />
                        </TopNav>
                        <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
                            <div className="w-full">
                                <SearchField
                                    placeholder="Поиск операций"
                                    value={searchValue}
                                    onChange={(event) => setSearchValue(event.target.value)}
                                    onClear={() => setSearchValue('')}
                                />
                            </div>
                        </TopNav>
                        {/* Хаб-вариант (#523): «крылья» и на мобайле; вживую
                         * смотрится на экране «Задачи» (/tasks). У подэкранов
                         * с leading-кнопкой крылья на планшете (561–1023)
                         * скрываются — наложение на колонку 560 (аудит #563),
                         * на ПК ≥1024 — всегда. */}
                        <TopNav mobileWings />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>TopNavBackButton · HubTitle · анатомия подэкрана и хаба (#566)</h3>
                    <p className={styles.groupTitle}>
                        Канонические куски хаб-шапки и подэкранного хедера: TopNavBackButton —
                        ведущая кнопка «Назад» (history-first goBack с фолбэком, сериализуемые
                        пропы — страницу-серверный компонент можно не делать клиентской);
                        HubTitle — заголовок раздела хаба 28/32 со своим паддингом 24
                        (PageContent горизонталей не вкладывает). Вживую: любой подэкран
                        дерева профиля и хаб «Уведомления» (/profile/notifications).
                    </p>
                    <div className={styles.column} style={{ maxWidth: 480 }}>
                        <TopNav leading={<TopNavBackButton fallbackHref="/ui-kit" />}>
                            <TopNavTitle title="Профиль" />
                        </TopNav>
                        <div className="rounded-button bg-surface-muted p-4">
                            <HubTitle>Уведомления</HubTitle>
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>SubScreenShell · каркас подэкрана (#568)</h3>
                    <p className={styles.groupTitle}>
                        Один компонент вместо ручной сборки подэкранной шапки: TopNav с
                        ведущим «Назад» (TopNavBackButton), TopNavTitle в центре и
                        PageContent с боковым паддингом 24. trailing — действия экрана
                        (кебаб объекта), subtitle — серый подзаголовок под заголовком
                        (#587, счётчик архива), contentClassName переопределяет паддинг
                        контента. Шапка в этой сборке — fixed на планшете и ПК, поэтому
                        вживую она видна на любом подэкране (дерево профиля, детализация
                        и правка объекта); ниже — контентная колонка каркаса.
                    </p>
                    <div className={styles.column} style={{ maxWidth: 480 }}>
                        <SubScreenShell
                            title="Информация об объекте"
                            subtitle="7 объектов"
                            fallbackHref="/ui-kit"
                            contentClassName="pt-4 pb-4"
                        >
                            <p className="text-sm text-content-secondary">
                                Контент подэкрана в колонке 560.
                            </p>
                        </SubScreenShell>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>PageContent</h3>
                    <div className="rounded-button bg-surface-muted p-4">
                        <PageContent className="pt-4 pb-4">
                            <p className="text-sm text-content-secondary">
                                Центрированная колонка max-width 560 — контейнер новых экранов
                                (отступы 72/136 уменьшены для витрины).
                            </p>
                        </PageContent>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>Modal</h3>
                    <div className={styles.grid}>
                        <Modal>
                            <ModalTrigger asChild>
                                <Button>Открыть модалку</Button>
                            </ModalTrigger>
                            <ModalContent title="Что создать?" description="Выберите тип регулярного платежа">
                                <div className={styles.column} style={{ alignItems: 'stretch' }}>
                                    <Button variant="secondary">Платеж</Button>
                                    <Button variant="secondary">Автоплатеж</Button>
                                    <ModalClose asChild>
                                        <Button variant="clear">Отмена</Button>
                                    </ModalClose>
                                </div>
                            </ModalContent>
                        </Modal>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>SupportModal · «Связаться с нами» (#766)</h3>
                    <p className={styles.groupTitle}>
                        Единая поверхность поддержки вместо страницы /support (карта #761):
                        канва Modal — карточка ≥768 / vaul-шит ниже; лого HeaderLogo 112×28,
                        заголовок H1 28/32 + подпись 16/18, Primary «Написать в Телеграм»
                        (внешняя ссылка в новой вкладке) и Secondary-строка почты с копированием
                        в буфер (Figma 2355:52709 — десктоп, 2355:52684 — планшет, 2355:52746 —
                        мобайл). Карточка макета уже канона — max-w-400. Триггеры в продукте:
                        пилюля «Поддержка» десктопа, шестая ячейка шита «Еще», кнопка экрана
                        «Тариф», пилюли «Написать в поддержку» шагов входа.
                    </p>
                    <div className={styles.grid}>
                        <Button onClick={() => setSupportModalOpen(true)}>
                            Открыть «Связаться с нами»
                        </Button>
                    </div>
                    <SupportModal open={supportModalOpen} onOpenChange={setSupportModalOpen} />
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>MoreSheet · шит «Еще» мобильного TabBar</h3>
                    <p className={styles.groupTitle}>
                        Выезжающий снизу шит навигации: ручка 48×4, два ряда разделов из нав-модели,
                        шестая ячейка — «Поддержка»-действие (TabNavAction, открывает SupportModal —
                        #766), нижний ряд — сам TabBar с активным «Еще» (Figma 1721:57140, #560). Выезд
                        400ms на кривой vaul, оверлей — fade 250ms; закрытие — оверлей, свайп вниз,
                        повторный тап «Еще». В продукте живёт в TabBar (мобайл/планшет ≤768), здесь —
                        с ручным триггером.
                    </p>
                    <div className={styles.grid}>
                        <Button onClick={() => setMoreSheetOpen(true)}>Открыть шит «Еще»</Button>
                    </div>
                    <MoreSheet open={moreSheetOpen} onOpenChange={setMoreSheetOpen} />
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>DesktopMenuButton · десктопная навигация (#561)</h3>
                    <p className={styles.groupTitle}>
                        Кнопка десктопного хрома (Figma 1675:54051): 200×44, radius 16, иконка 24 +
                        подпись 14/16; активная — серая плашка bg-surface-muted, hover-фона нет.
                        В продукте ScreenLayout рендерит из них DesktopSidebar (6 разделов слева под
                        хедером, Figma 1675:54050) и DesktopNavPills («Уведомления» — левый-низ 200,
                        «Поддержка» — правый-низ авто, Figma 1675:54098/54096) — только на ПК ≥1024
                        (561–1023 — планшетный хром с TabBar); пилюли глушатся вместе с TabBar,
                        пока открыт StickyBottomBar. С onClick рендерится кнопкой-действием
                        вместо ссылки — так живёт пилюля «Поддержка»: открывает SupportModal (#766).
                        Живой вид — на любом экране продукта при ширине ≥1024.
                    </p>
                    <div className={styles.grid}>
                        <div className="flex flex-col gap-0.5">
                            <DesktopMenuButton section={navSectionById('properties')} active />
                            <DesktopMenuButton section={navSectionById('payments')} />
                            <DesktopMenuButton section={navSectionById('operations')} />
                        </div>
                        <div className="flex flex-col items-start gap-2">
                            <DesktopMenuButton section={navSectionById('notifications')} active />
                            <DesktopMenuButton
                                section={supportNavSection}
                                onClick={() => setSupportModalOpen(true)}
                                className="w-fit"
                            />
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>Textarea · многострочное поле (#505)</h3>
                    <p className={styles.groupTitle}>
                        Shell как у TextField (Title Out), бокс 92px — четыре строки 16/18 с
                        прокруткой сверх; счётчик «длина/лимит» краснеет на лимите (Figma
                        1281:48439, «Заметка» формы контакта).
                    </p>
                    <div className={styles.textFields}>
                        <Textarea
                            title="Заметка"
                            placeholder="Заметка о контакте"
                            description="Необязательно"
                            maxLength={1024}
                            value={note}
                            onChange={(event) => setNote(event.target.value)}
                        />
                        <Textarea
                            title="Заметка"
                            error="Ошибка"
                            maxLength={1024}
                            value={noteError}
                            onChange={(event) => setNoteError(event.target.value)}
                        />
                        <Textarea title="Заметка" placeholder="Заметка о контакте" disabled />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>PickerField · пикер с поиском (#505)</h3>
                    <p className={styles.groupTitle}>
                        Триггер — бокс поля с шевроном; список — в адаптивном Modal (шит на
                        мобильном, карточка на десктопе), поиск по названию и подсказке,
                        строка «Без объекта» очищает значение. Первый потребитель —
                        «Привязанный объект» формы контакта.
                    </p>
                    <div className={styles.textFields}>
                        <PickerField
                            title="Привязанный объект"
                            placeholder="Выберите объект"
                            options={showcaseObjects}
                            clearable
                            value={pickerValue}
                            onValueChange={setPickerValue}
                        />
                        <PickerField
                            title="Привязанный объект"
                            placeholder="Выберите объект"
                            options={showcaseObjects}
                            clearable
                            value={pickerEmpty}
                            onValueChange={setPickerEmpty}
                        />
                        <PickerField
                            title="Привязанный объект"
                            placeholder="Выберите объект"
                            options={showcaseObjects}
                            disabled
                        />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>EmptyState · пустое состояние</h3>
                    <p className={styles.groupTitle}>
                        Иллюстрация 128 + заголовок + серое описание (максимум 360) и действие.
                        Полноэкранные «пусто» — задачи, контакты, операции (Figma 1535-75363,
                        1527:74479, 1510-77308).
                    </p>
                    <div className={styles.grid}>
                        <EmptyState
                            imageSrc="/images/contacts/empty-contacts.png"
                            title="Контактов нет"
                            description="Добавьте контакты арендатора, мастеров и других специалистов"
                        />
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>ResendCodeTile · повторная отправка кода</h3>
                    <p className={styles.groupTitle}>
                        Resend-канон шага кода (#733, Figma 1869-68137/2343-51004): Secondary-кнопка
                        «Отправить новый код», на таймере disabled с подписью ММ:СС (Roboto Mono),
                        по истечении подпись скрыта. Таймер-хранение — за фичей (useCountdown).
                    </p>
                    <div className={styles.grid}>
                        <div className="flex w-full flex-col gap-6">
                            <ResendCodeTile remainingSeconds={42} onResend={() => {}} />
                            <ResendCodeTile remainingSeconds={0} onResend={() => {}} />
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>Skeleton · скелетон загрузки</h3>
                    <p className={styles.groupTitle}>
                        Пульс на bg-surface-muted, размер и форма — через className; внутри
                        серой карточки — bg-surface-muted-hover.
                    </p>
                    <div className={styles.grid}>
                        <div className="flex w-full flex-col gap-2">
                            <Skeleton className="h-11 w-3/5 bg-surface-muted-hover" />
                            <Skeleton className="h-11 w-4/5 bg-surface-muted-hover" />
                            <Skeleton className="h-11 w-2/5 bg-surface-muted-hover" />
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>
                        Skeleton-примитивы · составные заглушки загрузки
                    </h3>
                    <p className={styles.groupTitle}>
                        Композиции канона Skeleton под анатомию реальных блоков (#604,
                        паритет — §7 DESIGN.md): контент занимает место скелетона без
                        сдвига. API финализирован на хабах карты #603 (#605).
                    </p>
                    <div className={styles.grid}>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>SkeletonListRow · строка списка</h4>
                            <div className="rounded-card border border-dashed border-content-tertiary">
                                <SkeletonListRow widths={{title: 'w-2/5', subtitle: 'w-3/5'}} />
                                <SkeletonListRow
                                    value
                                    description
                                    widths={{title: 'w-1/2', subtitle: 'w-2/5'}}
                                />
                                <SkeletonListRow value widths={{title: 'w-3/5', subtitle: 'w-1/2'}} />
                                <SkeletonListRow
                                    value
                                    trailing
                                    leading={false}
                                    widths={{title: 'w-2/5', subtitle: 'w-3/5'}}
                                />
                                <SkeletonListRow
                                    subtitle={false}
                                    value
                                    trailing
                                    widths={{title: 'w-2/5', subtitle: 'w-3/5'}}
                                />
                            </div>
                        </div>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>SkeletonSection · серая секция</h4>
                            <SkeletonSection rows={3} />
                            <SkeletonSection rows={2} className="mx-6" />
                        </div>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>SkeletonCard · карточка-плитка</h4>
                            <div className="flex gap-2 overflow-hidden">
                                <SkeletonCard />
                                <SkeletonCard className="w-40" />
                            </div>
                        </div>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>SkeletonMedia · фото/баннер</h4>
                            <SkeletonMedia />
                            <div className="flex items-center gap-2">
                                <SkeletonMedia className="h-20 w-20 rounded-full" />
                                <SkeletonMedia className="h-20 flex-1" />
                            </div>
                        </div>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>SkeletonButton · CTA-кнопка</h4>
                            <SkeletonButton />
                            <SkeletonButton className="w-2/3" />
                        </div>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>SkeletonFormField · поле формы</h4>
                            <div className="flex flex-col gap-6 rounded-card border border-dashed border-content-tertiary p-6">
                                <SkeletonFormField />
                                <SkeletonFormField labelWidth="w-36" />
                            </div>
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>InfiniteQueryTail · хвост бесконечной ленты</h3>
                    <p className={styles.groupTitle}>
                        Sentinel дозагрузки и индикатор «Загружаем еще» одной строкой на
                        экран (#633): компонент сам держит sentinel, подключает
                        useInfiniteScroll и жив при тёплом кэше (#631). Ниже — оба тона
                        индикатора догрузки (LoadingMoreIndicator) на статичной заглушке
                        запроса; в приложении хвост ставится последним элементом ленты.
                    </p>
                    <div className={styles.grid}>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>Хвост без догрузки · sentinel ждёт</h4>
                            <div className="rounded-card border border-dashed border-content-tertiary p-4 text-center text-sm text-content-secondary">
                                <InfiniteQueryTail query={STUB_IDLE_QUERY} />
                                ↑ sentinel (пустой div) — невидим, продолжение появится при подходе к краю
                            </div>
                        </div>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>LoadingMoreIndicator · тон base</h4>
                            <div className="rounded-card border border-dashed border-content-tertiary">
                                <InfiniteQueryTail query={STUB_FETCHING_QUERY} />
                            </div>
                        </div>
                        <div className="flex w-full flex-col gap-2">
                            <h4 className={styles.groupTitle}>LoadingMoreIndicator · тон muted</h4>
                            <div className="flex flex-col gap-2 rounded-card bg-surface-muted p-3">
                                <InfiniteQueryTail query={STUB_FETCHING_QUERY} tone="muted" />
                            </div>
                        </div>
                    </div>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>ConfirmDialog · подтверждение</h3>
                    <p className={styles.groupTitle}>
                        Карточка на десктопе, нижний шит на мобиле: отмена secondary +
                        подтверждение primary; разрушительное — confirmVariant=&quot;danger&quot;.
                        Закрытие — на потребителе.
                    </p>
                    <div className={styles.grid}>
                        <Button onClick={() => setConfirmOpen(true)}>Подтвердить действие</Button>
                        <Button variant="danger" onClick={() => setDeleteOpen(true)}>
                            Удалить (danger)
                        </Button>
                        <Button onClick={() => setConfirmLargeOpen(true)}>
                            Подтвердить (описание 16)
                        </Button>
                        <Button variant="danger" onClick={() => setDeletePropertyOpen(true)}>
                            Удалить объект (столбиком)
                        </Button>
                    </div>
                    <ConfirmDialog
                        open={confirmOpen}
                        onOpenChange={setConfirmOpen}
                        title="Сменить тариф?"
                        description="Новые условия применятся со следующего периода"
                        confirmLabel="Сменить"
                        onConfirm={() => setConfirmOpen(false)}
                    />
                    <ConfirmDialog
                        open={deleteOpen}
                        onOpenChange={setDeleteOpen}
                        title="Удалить контакт?"
                        description="Контакт исчезнет из книги контактов объекта"
                        confirmLabel="Удалить"
                        confirmVariant="danger"
                        onConfirm={() => setDeleteOpen(false)}
                    />
                    {/* #627: подпись макета R/400 16/18 — descriptionClassName
                     * поверх каноничных 14px. */}
                    <ConfirmDialog
                        open={confirmLargeOpen}
                        onOpenChange={setConfirmLargeOpen}
                        title="Завершить аренду?"
                        description="Объект станет свободным, арендный платеж завершится. Данные аренды сохранятся в разделе «Прошлые аренды»"
                        descriptionClassName="text-base leading-[18px]"
                        confirmLabel="Завершить"
                        cancelLabel="Отменить"
                        onConfirm={() => setConfirmLargeOpen(false)}
                    />
                    {/* #629: stacked + children + titleClassName — кнопки
                     * столбиком (danger сверху, решение владельца 12.09),
                     * заголовок H1 28/32, красное предупреждение о
                     * последствиях (danger-soft) между описанием и кнопками.
                     * Figma 1583:56558. */}
                    <ConfirmDialog
                        open={deletePropertyOpen}
                        onOpenChange={setDeletePropertyOpen}
                        title="Удалить объект?"
                        titleClassName="text-[28px] leading-8"
                        description="Объект будет удален. Вместо удаления объект можно перевести в архив"
                        descriptionClassName="text-base leading-[18px]"
                        confirmLabel="Удалить"
                        cancelLabel="Отменить"
                        confirmVariant="danger"
                        stacked
                        onConfirm={() => setDeletePropertyOpen(false)}
                    >
                        <p className="text-sm leading-4 text-danger-soft">
                            Будут удалены данные аренд объекта, все операции объекта, платежи, контакты и задачи, связанные с объектом. Это действие нельзя отменить
                        </p>
                    </ConfirmDialog>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>StickyBottomBar</h3>
                    <p className={styles.groupTitle}>
                        Живой образец закреплён внизу окна — вариант из визарда (кнопка «Далее»).
                    </p>
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>SuccessPopup · успех-попап</h3>
                    <p className={styles.groupTitle}>
                        Центрированная карточка без затемнения на любой ширине (#744, Figma
                        2329:148661): иконка Icon/Color/GoodWhite 48, зелёное сообщение 16/18
                        Medium (#00A63E из макета), крестик закрытия. Потребители — действия
                        центра уведомлений.
                    </p>
                    <div className={styles.grid}>
                        <Button onClick={() => setSuccessPopupOpen(true)}>Показать попап</Button>
                    </div>
                </div>
            </section>

            <SuccessPopup
                open={successPopupOpen}
                onOpenChange={setSuccessPopupOpen}
                message="Все уведомления прочитаны"
            />

            <StickyBottomBar>
                <Button>Далее</Button>
            </StickyBottomBar>
        </>
    );
}
