// ПРОТОТИП (throwaway): маршрут для UI-прототипа задач объекта (тикет #279).
// Не часть продукта; живёт только на ветке прототипа.

import type { Metadata } from 'next';
import { Suspense } from 'react';
import type { JSX } from 'react';
import { TasksPrototypePage } from '@/widgets/prototype-tasks';

export const metadata: Metadata = {
    title: 'Прототип: задачи объекта',
    robots: { index: false },
};

export default function PrototypeTasksRoutePage(): JSX.Element {
    return (
        <Suspense>
            <TasksPrototypePage />
        </Suspense>
    );
}
