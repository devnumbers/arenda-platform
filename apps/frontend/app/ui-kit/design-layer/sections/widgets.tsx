'use client';

import type { JSX } from 'react';
import { ArrowDown, BoldSofa, Move, Star, StarOff } from '@/shared/assets/icons';
import {
    CalendarButton,
    ChipButton,
    ErrorCard,
    IconButton,
    ListRow,
    PickerMenu,
    StatusIcon,
    StepsChip,
    UserButton,
} from '@/shared/ui/design';
import styles from '../../page.module.css';

const statusGradations = ['danger', 'warning', 'good', 'check', 'info'] as const;

export function StepsChipSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>StepsChip</h3>
            <div className={styles.grid}>
                <StepsChip step={1} total={5} />
                <StepsChip step={2} total={6} size="m" />
            </div>
        </div>
    );
}

export function ErrorCardSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>ErrorCard</h3>
            <div className={styles.links}>
                <ErrorCard
                    title="Не удалось загрузить раздел"
                    onRetry={() => {}}
                    className="mx-0 max-w-[360px]"
                />
            </div>
        </div>
    );
}

export function StatusIconSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>StatusIcon</h3>
            <div className={styles.grid}>
                {statusGradations.map((status) => (
                    <StatusIcon key={status} status={status} />
                ))}
            </div>
        </div>
    );
}

export function UserButtonSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>UserButton</h3>
            <div className={styles.links}>
                <UserButton name="Даниил" />
                <UserButton name="Профиль" disabled />
            </div>
        </div>
    );
}

export function CalendarButtonSection(): JSX.Element {
    return (
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
    );
}

export function PickerMenuSection(): JSX.Element {
    return (
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
    );
}

export function ListRowSection(): JSX.Element {
    return (
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
                    // Интерактивная строка с фокусируемым вложенным
                    // кнопкой: Enter/Space на «Переставить» активируют
                    // кнопку, а не строку (гвард вложенного keydown,
                    // #831) — без onSelect геометрия непрогоняема.
                    onSelect={() => undefined}
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
    );
}
