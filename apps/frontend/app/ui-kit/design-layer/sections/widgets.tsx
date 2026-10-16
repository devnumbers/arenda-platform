'use client';

import type { JSX } from 'react';
import { ArrowDown, BoldSofa, BoldUser, Move, Star, StarOff } from '@/shared/assets/icons';
import {
    CalendarButton,
    ChipButton,
    CircleIcon,
    circleIconRing,
    ErrorCard,
    IconButton,
    ListRow,
    PickerMenu,
    StatusIcon,
    StepsChip,
    UserButton,
} from '@/shared/ui/design';
import { cn } from '@/shared/lib/cn';
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
            <p className={styles.groupTitle}>
                Слева направо: имя + аватар; pending — useMe в полёте, вместо
                имени скелетон h-4 w-10 (доступное имя «Профиль» даёт сам
                компонент). Компонент — настоящий Link на /profile: имя видно
                на всех ярусах, переход работает и средней кнопкой (решение
                владельца 02.10, макет 2329-148674 — отмена аватар-only
                аудита #876). Вживую ссылка живёт в крыльях TopNav
                (shared/ui/design/top-nav.tsx).
            </p>
            <div className={styles.links}>
                <UserButton name="Даниил" />
                <UserButton pending />
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
                        <span className={cn('flex h-11 w-11 items-center justify-center rounded-pill bg-[#FF8904] text-white', circleIconRing.white)}>
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

/** Канон «круг 44 с кольцом 2.5px» (#846): слот иконки/аватара строк,
 * вариант — подложка (muted — серая, white — белая), кант красится её
 * цветом. Размер 96 — классом потребителя; круг под фото (overflow-hidden
 * rounded-full) — в экранах, живой пример — ObjectAvatarGlyph
 * (widgets/participants/ui/participant-fragments.tsx:26). */
export function CircleIconSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>CircleIcon</h3>
            <div className={styles.links}>
                <div className="flex items-center gap-4">
                    <CircleIcon variant="white" aria-hidden>
                        <BoldUser className="h-6 w-6" />
                    </CircleIcon>
                    <CircleIcon variant="muted" aria-hidden>
                        <BoldUser className="h-6 w-6 text-content" />
                    </CircleIcon>
                    <CircleIcon variant="white" aria-hidden className="h-24 w-24">
                        <BoldUser className="h-[52px] w-[52px]" />
                    </CircleIcon>
                </div>
            </div>
        </div>
    );
}
