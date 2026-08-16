// ПРОТОТИП (throwaway): три варианта UI задач объекта, переключаемые через ?variant=
// на throwaway-маршруте /prototype-tasks. Отвечает на вопрос тикета #279:
// список задач, создание/редактирование, редактор повтора (вкл. «каждый месяц 11, 14, 27»),
// выполнено↔невыполнено, правка одиночного вхождения из календаря.

'use client';

import { useMemo, useReducer, type JSX } from 'react';
import { useSearchParams } from 'next/navigation';
import { PrototypeSwitcher, type PrototypeVariant } from '@/shared/ui/prototype-switcher';
import { buildFixtureTasks } from '../model/fixtures';
import { tasksReducer } from '../model/store';
import { VariantA } from './VariantA';
import { VariantB } from './VariantB';
import { VariantC } from './VariantC';
import styles from './TasksPrototypePage.module.css';

const VARIANTS: readonly PrototypeVariant[] = [
    { key: 'A', label: 'Секция в объекте' },
    { key: 'B', label: 'Отдельный экран' },
    { key: 'C', label: 'Повестка и календарь' },
];

const VARIANT_HINTS: Record<string, string> = {
    A: 'Задачи — секция на странице объекта: быстрое добавление, шит редактора, drill-down повтора, мини-календарь ниже.',
    B: 'Отдельный экран задач объекта: сегмент-фильтр, список + календарь рядом, редактор — боковая панель с повтором-«предложением».',
    C: 'Календарь-центрично: неделя-лента и повестка дня, добавление под датой, редактор — полный экран.',
};

export function TasksPrototypePage(): JSX.Element {
    const searchParams = useSearchParams();
    const variant = searchParams.get('variant') ?? 'A';
    const [tasks, dispatch] = useReducer(tasksReducer, undefined, buildFixtureTasks);

    const hint = useMemo(() => VARIANT_HINTS[variant] ?? VARIANT_HINTS.A, [variant]);

    return (
        <div className={styles.root}>
            <div className={styles.prototypeBanner}>
                <span className={styles.prototypeBadge}>Прототип</span>
                <span className={styles.prototypeText}>{hint}</span>
            </div>

            {variant === 'A' && <VariantA tasks={tasks} dispatch={dispatch} />}
            {variant === 'B' && <VariantB tasks={tasks} dispatch={dispatch} />}
            {variant === 'C' && <VariantC tasks={tasks} dispatch={dispatch} />}
            {variant !== 'A' && variant !== 'B' && variant !== 'C' && (
                <VariantA tasks={tasks} dispatch={dispatch} />
            )}

            <PrototypeSwitcher variants={VARIANTS} current={variant} />
        </div>
    );
}
