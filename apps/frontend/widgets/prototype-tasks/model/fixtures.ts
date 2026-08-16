// ПРОТОТИП (throwaway): демо-данные. Даты считаются от «сегодня», чтобы демо всегда было живым.

import type { PrototypeTask } from './types';
import { addDaysISO, todayISO, weekdayOf } from '../lib/recurrence';

export const PROTOTYPE_PROPERTY = {
    id: 'demo-property',
    name: 'Квартира на Ленина, 15',
    address: '2-к квартира, ул. Ленина, 15, кв. 48',
} as const;

function monthShift(delta: number, day: number): string {
    const now = new Date();
    const d = new Date(now.getFullYear(), now.getMonth() + delta, day);
    const y = d.getFullYear();
    const m = String(d.getMonth() + 1).padStart(2, '0');
    return `${y}-${m}-${String(day).padStart(2, '0')}`;
}

export function buildFixtureTasks(): PrototypeTask[] {
    const today = todayISO();
    const saturday = addDaysISO(today, -((weekdayOf(today) - 5 + 7) % 7));
    return [
        {
            id: 't1',
            title: 'Передать показания счётчиков',
            description: 'Электро и вода, через личный кабинет УК',
            dueDate: monthShift(-1, 25),
            dueTime: '10:00',
            recurrence: { freq: 'monthly', interval: 1, byMonthDays: [25] },
            completedDates: [monthShift(-1, 25)],
        },
        {
            id: 't2',
            title: 'Проверить фильтры воды',
            description: 'Магистральные фильтры под раковиной',
            dueDate: monthShift(-1, 11),
            recurrence: { freq: 'monthly', interval: 1, byMonthDays: [11, 14, 27] },
            completedDates: [monthShift(-1, 27), monthShift(0, 11)],
        },
        {
            id: 't3',
            title: 'Встретить клининг',
            dueDate: addDaysISO(saturday, -7),
            dueTime: '12:00',
            recurrence: { freq: 'weekly', interval: 1, byWeekdays: [5] },
            completedDates: [],
        },
        {
            id: 't4',
            title: 'Обслуживание котла',
            description: 'Гарантийное ТО, мастер из «ТеплоСервис»',
            dueDate: monthShift(2, 3),
            recurrence: { freq: 'yearly', interval: 1, count: 5 },
            completedDates: [],
        },
        {
            id: 't5',
            title: 'Позвонить по страховке',
            description: 'Продлить полис до конца месяца',
            dueDate: addDaysISO(today, 1),
            dueTime: '15:00',
            completedDates: [],
        },
        {
            id: 't6',
            title: 'Заменить батарейки в домофоне',
            dueDate: addDaysISO(today, -1),
            completedDates: [],
        },
        {
            id: 't7',
            title: 'Подписать акт с клинингом',
            dueDate: today,
            completedDates: [today],
        },
    ];
}
