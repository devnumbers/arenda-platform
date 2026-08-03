// PROTOTYPE — throwaway, issue #105

export type PrototypeProperty = {
  readonly id: string;
  readonly name: string;
};

export const prototypeProperties: readonly PrototypeProperty[] = [
  { id: 'lenina15', name: '2-к квартира, Ленина 15' },
  { id: 'sovetskaya8', name: 'Студия, Советская 8' },
  { id: 'zavodskaya12', name: 'Гараж, Заводская 12' },
];

export type PrototypeReminderFrequency = 'once' | 'daily' | 'weekly' | 'monthly' | 'yearly';

export const prototypeFrequencyOptions: readonly {
  readonly value: PrototypeReminderFrequency;
  readonly label: string;
}[] = [
  { value: 'once', label: 'Разовое' },
  { value: 'daily', label: 'Каждый день' },
  { value: 'weekly', label: 'Каждую неделю' },
  { value: 'monthly', label: 'Каждый месяц' },
  { value: 'yearly', label: 'Каждый год' },
];

export const prototypeFrequencyLabels: Record<PrototypeReminderFrequency, string> = {
  once: 'Разовое',
  daily: 'Каждый день',
  weekly: 'Каждую неделю',
  monthly: 'Каждый месяц',
  yearly: 'Каждый год',
};

export type PrototypeReminder = {
  readonly id: string;
  readonly title: string;
  readonly propertyId: string;
  readonly date: string; // 'YYYY-MM-DD'
  readonly time: string; // 'HH:MM'
  readonly frequency: PrototypeReminderFrequency;
};

export const PROTOTYPE_OBJECT_ID = 'lenina15';

export const prototypeReminders: readonly PrototypeReminder[] = [
  {
    id: 'r1',
    title: 'Оплатить интернет',
    propertyId: 'lenina15',
    date: '2026-03-12',
    time: '10:00',
    frequency: 'monthly',
  },
  {
    id: 'r2',
    title: 'Передать показания счётчиков',
    propertyId: 'lenina15',
    date: '2026-03-20',
    time: '09:30',
    frequency: 'monthly',
  },
  {
    id: 'r3',
    title: 'Встретить жильцов с ключами',
    propertyId: 'sovetskaya8',
    date: '2026-03-14',
    time: '18:00',
    frequency: 'once',
  },
  {
    id: 'r4',
    title: 'Продлить страховку квартиры',
    propertyId: 'lenina15',
    date: '2026-11-01',
    time: '12:00',
    frequency: 'yearly',
  },
];

// Напоминание, которое показывает страница «Просмотр/правка».
export const prototypeItemReminder: PrototypeReminder = prototypeReminders[0];

// Напоминания текущего объекта (Ленина 15), ближайшие первыми.
export const prototypeObjectReminders: readonly PrototypeReminder[] = prototypeReminders
  .filter((reminder) => reminder.propertyId === PROTOTYPE_OBJECT_ID)
  .sort((a, b) => `${a.date}T${a.time}`.localeCompare(`${b.date}T${b.time}`))
  .slice(0, 3);

export function prototypePropertyName(propertyId: string): string {
  return prototypeProperties.find((property) => property.id === propertyId)?.name ?? 'Без объекта';
}

export type PrototypeReminderDraft = {
  readonly title: string;
  readonly date: string; // 'YYYY-MM-DD' или ''
  readonly time: string; // 'HH:MM' или ''
  readonly frequency: PrototypeReminderFrequency;
  readonly propertyId: string | undefined;
};

export const prototypeEmptyDraft: PrototypeReminderDraft = {
  title: '',
  date: '',
  time: '',
  frequency: 'monthly',
  propertyId: undefined,
};

export function prototypeDraftFromReminder(reminder: PrototypeReminder): PrototypeReminderDraft {
  return {
    title: reminder.title,
    date: reminder.date,
    time: reminder.time,
    frequency: reminder.frequency,
    propertyId: reminder.propertyId,
  };
}

export function formatPrototypeDate(date: string): string {
  const parsed = new Date(`${date}T00:00:00`);
  if (Number.isNaN(parsed.getTime())) {
    return date;
  }
  return parsed.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
}

export function formatPrototypeDateTime(reminder: Pick<PrototypeReminder, 'date' | 'time'>): string {
  return `${formatPrototypeDate(reminder.date)}, ${reminder.time}`;
}
