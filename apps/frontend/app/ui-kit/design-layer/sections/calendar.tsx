'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowDown } from '@/shared/assets/icons';
import type { IsoRange } from '@/shared/lib/calendar';
import { addDays, dateToIso } from '@/shared/lib/calendar';
import {
    Button,
    CalendarDatePicker,
    CalendarMonth,
    CalendarRangePicker,
    Checkbox,
    ChipButton,
    ListRow,
    MonthDaysGrid,
    MonthYearPicker,
    monthTitle,
} from '@/shared/ui/design';
import styles from '../../page.module.css';

export function CalendarMonthSection(): JSX.Element {
    const [calendarYear, setCalendarYear] = useState(2026);
    const [calendarMonth, setCalendarMonth] = useState(7);
    const [selectedDate, setSelectedDate] = useState<Date | undefined>(new Date(2026, 7, 17));
    const [wheelOpen, setWheelOpen] = useState(false);

    return (
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
    );
}

export function MonthYearPickerSection(): JSX.Element {
    const [boundedWheelOpen, setBoundedWheelOpen] = useState(false);
    const [boundedMonth, setBoundedMonth] = useState(8);
    const [boundedYear, setBoundedYear] = useState(2026);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>MonthYearPicker · кольцо месяцев при min/max</h3>
            <p className={styles.groupTitle}>
                Колесо месяцев замкнуто (#810), но при границах кольцо идёт по дуге
                разрешённых: здесь март–сентябрь 2026 — сентябрь докручивается в март,
                закрытые месяцы не открываются. Годы линейны: за 2026 ничего нет.
            </p>
            <div className={styles.column}>
                <ChipButton trailingIcon={<ArrowDown />} onClick={() => setBoundedWheelOpen(true)}>
                    {monthTitle(boundedYear, boundedMonth)}
                </ChipButton>
            </div>
            <MonthYearPicker
                open={boundedWheelOpen}
                onOpenChange={setBoundedWheelOpen}
                month={boundedMonth}
                year={boundedYear}
                min={{ year: 2026, month0: 2 }}
                max={{ year: 2026, month0: 8 }}
                onConfirm={(month, year) => {
                    setBoundedMonth(month);
                    setBoundedYear(year);
                    setBoundedWheelOpen(false);
                }}
            />
        </div>
    );
}

export function CalendarDatePickerSection(): JSX.Element {
    const [datePickerOpen, setDatePickerOpen] = useState(false);
    const [datePickerValue, setDatePickerValue] = useState<string | null>(null);
    const [datePickerRequiredOpen, setDatePickerRequiredOpen] = useState(false);
    const [datePickerRequiredValue, setDatePickerRequiredValue] = useState<string | null>(null);
    const [datePickerMinOpen, setDatePickerMinOpen] = useState(false);
    const [datePickerMinValue, setDatePickerMinValue] = useState<string | null>(null);
    const [datePickerMaxOpen, setDatePickerMaxOpen] = useState(false);
    const [datePickerMaxValue, setDatePickerMaxValue] = useState<string | null>(null);
    const [datePickerAllowPastOpen, setDatePickerAllowPastOpen] = useState(false);
    const [datePickerAllowPastValue, setDatePickerAllowPastValue] = useState<string | null>(null);

    return (
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
            <div className={styles.column}>
                <Button onClick={() => setDatePickerAllowPastOpen(true)}>
                    Открыть пикер (allowPast: minDate в прошлом)
                </Button>
                {datePickerAllowPastValue !== null && (
                    <p className="px-6 text-base text-content-secondary">
                        Выбрано: {datePickerAllowPastValue}
                    </p>
                )}
            </div>
            {datePickerAllowPastOpen && (
                <CalendarDatePicker
                    allowPast
                    minDate={addDays(dateToIso(new Date()), -10)}
                    today={dateToIso(new Date())}
                    value={datePickerAllowPastValue}
                    onClose={() => setDatePickerAllowPastOpen(false)}
                    onConfirm={(date) => {
                        setDatePickerAllowPastValue(date);
                        setDatePickerAllowPastOpen(false);
                    }}
                />
            )}
        </div>
    );
}

export function CalendarRangePickerSection(): JSX.Element {
    const [rangePickerOpen, setRangePickerOpen] = useState(false);
    const [rangePickerValue, setRangePickerValue] = useState<IsoRange | null>(null);

    return (
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
    );
}

export function CalendarRangePickerEmptySection(): JSX.Element {
    const [rangePickerEmptyOpen, setRangePickerEmptyOpen] = useState(false);
    const [rangePickerEmptyValue, setRangePickerEmptyValue] = useState<IsoRange | null>(null);

    return (
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
    );
}

export function MonthDaysGridSection(): JSX.Element {
    const [monthDays, setMonthDays] = useState<ReadonlySet<number>>(new Set([10]));
    const [lastDayOfMonth, setLastDayOfMonth] = useState(false);

    return (
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
    );
}
