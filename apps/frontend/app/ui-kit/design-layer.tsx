'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import {
    ArrowDown,
    ArrowLeft,
    BoldHome,
    BoldPerson,
    BoldSofa,
    ChangeHorizontal,
    Edit,
    Home,
    Move,
    Search,
    SortingDown,
    Star,
    StarOff,
} from '@/shared/assets/icons';
import {
    AmountField,
    Button,
    CalendarButton,
    CalendarDatePicker,
    CalendarMonth,
    Checkbox,
    ChipButton,
    ConfirmDialog,
    EmptyState,
    IconButton,
    ListRow,
    Modal,
    ModalClose,
    ModalContent,
    ModalTrigger,
    MonthDaysGrid,
    MonthYearPicker,
    PageContent,
    PickerMenu,
    PickerField,
    Skeleton,
    RadioGroup,
    RadioGroupItem,
    SearchField,
    StatusIcon,
    UserButton,
    StepsChip,
    StickyBottomBar,
    Switch,
    Textarea,
    TextField,
    TopNav,
    TopNavTitle,
    WheelPicker,
    WheelPickerSheet,
    amountKopecks,
    monthTitle,
    type PickerOption,
    type WheelPickerItem,
} from '@/shared/ui/design';
import { dateToIso } from '@/shared/lib/calendar';
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
                        <Button variant="primary" leadingIcon={<Home />} trailingIcon={<Edit />}>
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
                            leadingIcon={<SortingDown />}
                            trailingIcon={<ArrowDown />}
                            selected={chip === 'name'}
                            onClick={() => setChip('name')}
                        >
                            По названию
                        </ChipButton>
                        <ChipButton
                            leadingIcon={<SortingDown />}
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
                                trailingIcon={<ChangeHorizontal />}
                                aria-label={`Форма оплаты: ${paymentForm === 'transfer' ? 'Перевод' : 'Наличные'}. Нажмите, чтобы переключить`}
                                onClick={() => setPaymentForm((prev) => (prev === 'transfer' ? 'cash' : 'transfer'))}
                            >
                                {paymentForm === 'transfer' ? 'Перевод' : 'Наличные'}
                            </ChipButton>
                            <ChipButton
                                className="w-40"
                                trailingIcon={<ChangeHorizontal />}
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
                </div>

                <div className={styles.group}>
                    <h3 className={styles.groupTitle}>StickyBottomBar</h3>
                    <p className={styles.groupTitle}>
                        Живой образец закреплён внизу окна — вариант из визарда (кнопка «Далее»).
                    </p>
                </div>
            </section>

            <StickyBottomBar>
                <Button>Далее</Button>
            </StickyBottomBar>
        </>
    );
}
