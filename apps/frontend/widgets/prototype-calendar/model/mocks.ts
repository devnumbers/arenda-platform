// PROTOTYPE — throwaway, issue #106

// «Сегодня» зафиксировано, чтобы демо было стабильным.
export const PROTOTYPE_TODAY = '2026-08-03';

export type PrototypeCalendarKind = 'free' | 'operation' | 'system';
export type PrototypeCalendarOperationStatus = 'due_soon' | 'overdue';
export type PrototypeCalendarSystemType = 'lease_expiring' | 'lease_requires_action';
export type PrototypeCalendarFrequency = 'once' | 'daily' | 'weekly' | 'monthly' | 'yearly';

export type PrototypeCalendarReminder = {
  readonly id: string;
  readonly kind: PrototypeCalendarKind;
  readonly title: string;
  readonly date: string; // 'YYYY-MM-DD'
  readonly time?: string; // 'HH:MM', только свободные
  readonly frequency?: PrototypeCalendarFrequency; // только свободные; маркер повторяющегося вхождения
  readonly propertyId: string | null;
  readonly propertyName: string | null; // null → «Без объекта»
  readonly operationStatus?: PrototypeCalendarOperationStatus;
  readonly systemType?: PrototypeCalendarSystemType;
};

export const PROTOTYPE_ORPHAN_PROPERTY_NAME = 'Без объекта';

// Повторяющиеся свободные напоминания предразвёрнуты в вхождения заранее:
// реальный эндпоинт будет разворачивать рекуррентность на сервере.
export const prototypeCalendarReminders: readonly PrototypeCalendarReminder[] = [
  // Свободные разовые
  {
    id: 'f1',
    kind: 'free',
    title: 'Встретить жильцов с ключами',
    date: '2026-07-29',
    time: '18:00',
    frequency: 'once',
    propertyId: 'tverskaya',
    propertyName: 'Офис на Тверской',
  },
  {
    id: 'f2',
    kind: 'free',
    title: 'Оплатить интернет',
    date: '2026-08-05',
    time: '10:00',
    frequency: 'once',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  {
    id: 'f3',
    kind: 'free',
    title: 'Созвон с жильцами офиса',
    date: '2026-08-05',
    time: '16:00',
    frequency: 'once',
    propertyId: 'tverskaya',
    propertyName: 'Офис на Тверской',
  },
  {
    id: 'f4',
    kind: 'free',
    title: 'Передать показания счётчиков',
    date: '2026-08-20',
    time: '09:30',
    frequency: 'once',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  {
    id: 'f5',
    kind: 'free',
    title: 'Забрать документы у нотариуса',
    date: '2026-09-02',
    time: '11:00',
    frequency: 'once',
    propertyId: 'tverskaya',
    propertyName: 'Офис на Тверской',
  },
  // Свободное еженедельное, развёрнутое по понедельникам
  {
    id: 'w1',
    kind: 'free',
    title: 'Проветрить квартиру',
    date: '2026-08-03',
    time: '19:00',
    frequency: 'weekly',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  {
    id: 'w2',
    kind: 'free',
    title: 'Проветрить квартиру',
    date: '2026-08-10',
    time: '19:00',
    frequency: 'weekly',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  {
    id: 'w3',
    kind: 'free',
    title: 'Проветрить квартиру',
    date: '2026-08-17',
    time: '19:00',
    frequency: 'weekly',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  {
    id: 'w4',
    kind: 'free',
    title: 'Проветрить квартиру',
    date: '2026-08-24',
    time: '19:00',
    frequency: 'weekly',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  {
    id: 'w5',
    kind: 'free',
    title: 'Проветрить квартиру',
    date: '2026-08-31',
    time: '19:00',
    frequency: 'weekly',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  // Ежемесячное и ежегодное — по одному вхождению с маркером повтора
  {
    id: 'm1',
    kind: 'free',
    title: 'Оплатить квитанции ЖКХ',
    date: '2026-08-25',
    time: '12:00',
    frequency: 'monthly',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
  },
  {
    id: 'y1',
    kind: 'free',
    title: 'Продлить страховку квартиры',
    date: '2026-09-05',
    time: '12:00',
    frequency: 'yearly',
    propertyId: 'sokolniki',
    propertyName: 'Гараж в Сокольниках',
  },
  // Напоминания по операциям
  {
    id: 'o1',
    kind: 'operation',
    title: 'Поступление оплаты аренды',
    date: '2026-08-05',
    propertyId: 'tverskaya',
    propertyName: 'Офис на Тверской',
    operationStatus: 'due_soon',
  },
  {
    id: 'o2',
    kind: 'operation',
    title: 'Возврат залога',
    date: '2026-08-05',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
    operationStatus: 'due_soon',
  },
  {
    id: 'o3',
    kind: 'operation',
    title: 'Поступление оплаты аренды',
    date: '2026-08-03',
    propertyId: 'sokolniki',
    propertyName: 'Гараж в Сокольниках',
    operationStatus: 'overdue',
  },
  // Системные события аренды
  {
    id: 's1',
    kind: 'system',
    title: 'Заканчивается договор аренды',
    date: '2026-08-30',
    propertyId: 'tverskaya',
    propertyName: 'Офис на Тверской',
    systemType: 'lease_expiring',
  },
  {
    id: 's2',
    kind: 'system',
    title: 'Подтвердить продление договора',
    date: '2026-08-02',
    propertyId: 'lenina12',
    propertyName: 'Квартира на Ленина, 12',
    systemType: 'lease_requires_action',
  },
  // Сироты: напоминания переживают удаление объекта
  {
    id: 'x1',
    kind: 'free',
    title: 'Позвонить сантехнику',
    date: '2026-08-08',
    time: '12:30',
    frequency: 'once',
    propertyId: null,
    propertyName: null,
  },
  {
    id: 'x2',
    kind: 'operation',
    title: 'Поступление оплаты аренды',
    date: '2026-07-27',
    propertyId: null,
    propertyName: null,
    operationStatus: 'overdue',
  },
];

export const prototypeCalendarKindLabels: Record<PrototypeCalendarKind, string> = {
  free: 'Свободное',
  operation: 'По операции',
  system: 'Системное',
};

export type PrototypeCalendarStatus = PrototypeCalendarOperationStatus | PrototypeCalendarSystemType;

export const prototypeCalendarStatusLabels: Record<PrototypeCalendarStatus, string> = {
  due_soon: 'Скоро срок',
  overdue: 'Просрочено',
  lease_expiring: 'Окончание аренды',
  lease_requires_action: 'Требует действия',
};

export function prototypeReminderStatus(
  reminder: PrototypeCalendarReminder,
): PrototypeCalendarStatus | null {
  return reminder.operationStatus ?? reminder.systemType ?? null;
}

export function prototypePropertyDisplayName(reminder: PrototypeCalendarReminder): string {
  return reminder.propertyName ?? PROTOTYPE_ORPHAN_PROPERTY_NAME;
}

// Маркер «↻» показываем только на вхождениях повторяющихся свободных напоминаний.
export function prototypeReminderIsRecurring(reminder: PrototypeCalendarReminder): boolean {
  return reminder.frequency !== undefined && reminder.frequency !== 'once';
}

// --- Селекторы ---

// Сначала по дате, затем по времени; безвременные (операции, системные) — после тех, что со временем.
export function comparePrototypeReminders(
  a: PrototypeCalendarReminder,
  b: PrototypeCalendarReminder,
): number {
  if (a.date !== b.date) {
    return a.date.localeCompare(b.date);
  }
  const aTime = a.time ?? '';
  const bTime = b.time ?? '';
  if (aTime === '' && bTime !== '') {
    return 1;
  }
  if (aTime !== '' && bTime === '') {
    return -1;
  }
  if (aTime !== bTime) {
    return aTime.localeCompare(bTime);
  }
  return a.id.localeCompare(b.id);
}

// yearMonth — 'YYYY-MM'.
export function remindersForMonth(yearMonth: string): readonly PrototypeCalendarReminder[] {
  return prototypeCalendarReminders
    .filter((reminder) => reminder.date.startsWith(yearMonth))
    .sort(comparePrototypeReminders);
}

export function remindersForDate(isoDate: string): readonly PrototypeCalendarReminder[] {
  return prototypeCalendarReminders
    .filter((reminder) => reminder.date === isoDate)
    .sort(comparePrototypeReminders);
}

export type PrototypeCalendarPropertyGroup = {
  readonly propertyId: string | null;
  readonly propertyName: string;
  readonly reminders: readonly PrototypeCalendarReminder[];
};

// Группы в порядке первого появления, сироты (propertyId null) — последними.
export function groupRemindersByProperty(
  reminders: readonly PrototypeCalendarReminder[],
): readonly PrototypeCalendarPropertyGroup[] {
  const groups = new Map<string | null, PrototypeCalendarReminder[]>();
  reminders.forEach((reminder) => {
    const bucket = groups.get(reminder.propertyId);
    if (bucket) {
      bucket.push(reminder);
    } else {
      groups.set(reminder.propertyId, [reminder]);
    }
  });

  const result: PrototypeCalendarPropertyGroup[] = [];
  let orphans: PrototypeCalendarPropertyGroup | null = null;
  groups.forEach((items, propertyId) => {
    const group: PrototypeCalendarPropertyGroup = {
      propertyId,
      propertyName: propertyId === null ? PROTOTYPE_ORPHAN_PROPERTY_NAME : (items[0].propertyName ?? ''),
      reminders: items,
    };
    if (propertyId === null) {
      orphans = group;
    } else {
      result.push(group);
    }
  });
  if (orphans !== null) {
    result.push(orphans);
  }
  return result;
}

// --- Даты: ручная арифметика на строках 'YYYY-MM-DD', неделя с понедельника ---

export const PROTOTYPE_WEEKDAY_SHORTS: readonly string[] = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

const WEEKDAY_FULL_BY_GETDAY: readonly string[] = [
  'воскресенье',
  'понедельник',
  'вторник',
  'среда',
  'четверг',
  'пятница',
  'суббота',
];

const WEEKDAY_SHORT_BY_GETDAY: readonly string[] = ['вс', 'пн', 'вт', 'ср', 'чт', 'пт', 'сб'];

const MONTH_NOMINATIVE: readonly string[] = [
  'Январь',
  'Февраль',
  'Март',
  'Апрель',
  'Май',
  'Июнь',
  'Июль',
  'Август',
  'Сентябрь',
  'Октябрь',
  'Ноябрь',
  'Декабрь',
];

const MONTH_GENITIVE_SHORT: readonly string[] = [
  'янв',
  'фев',
  'мар',
  'апр',
  'мая',
  'июн',
  'июл',
  'авг',
  'сен',
  'окт',
  'ноя',
  'дек',
];

function parseISODate(iso: string): Date {
  return new Date(`${iso}T00:00:00`);
}

export function toPrototypeISODate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

export function addPrototypeDays(iso: string, days: number): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() + days);
  return toPrototypeISODate(date);
}

// Семь дат (пн–вс) недели, содержащей isoDate.
export function prototypeWeekDates(isoDate: string): readonly string[] {
  const date = parseISODate(isoDate);
  const mondayOffset = (date.getDay() + 6) % 7;
  const monday = addPrototypeDays(isoDate, -mondayOffset);
  return Array.from({ length: 7 }, (_, index) => addPrototypeDays(monday, index));
}

// «Август 2026». monthIndex — 0-based.
export function prototypeMonthTitle(year: number, monthIndex: number): string {
  return `${MONTH_NOMINATIVE[monthIndex]} ${year}`;
}

export type PrototypeCalendarDayCell = {
  readonly date: string; // 'YYYY-MM-DD'
  readonly day: number;
  readonly inMonth: boolean;
  readonly isToday: boolean;
};

// Сетка месяца: недели (пн–вс) × 7 дней, с добивкой днями соседних месяцев.
export function prototypeMonthGridWeeks(
  year: number,
  monthIndex: number,
): readonly (readonly PrototypeCalendarDayCell[])[] {
  const first = new Date(year, monthIndex, 1);
  const leading = (first.getDay() + 6) % 7;
  const daysInMonth = new Date(year, monthIndex + 1, 0).getDate();
  const last = new Date(year, monthIndex, daysInMonth);
  const trailing = 6 - ((last.getDay() + 6) % 7);

  const start = new Date(year, monthIndex, 1 - leading);
  const totalDays = leading + daysInMonth + trailing;
  const weeks: PrototypeCalendarDayCell[][] = [];
  for (let index = 0; index < totalDays; index += 1) {
    const current = new Date(start);
    current.setDate(start.getDate() + index);
    const cell: PrototypeCalendarDayCell = {
      date: toPrototypeISODate(current),
      day: current.getDate(),
      inMonth: current.getMonth() === monthIndex,
      isToday: toPrototypeISODate(current) === PROTOTYPE_TODAY,
    };
    const weekIndex = Math.floor(index / 7);
    if (!weeks[weekIndex]) {
      weeks[weekIndex] = [];
    }
    weeks[weekIndex].push(cell);
  }
  return weeks;
}

// --- Форматирование (ru-RU) ---

// «3 августа»
export function formatPrototypeDate(date: string): string {
  const parsed = parseISODate(date);
  if (Number.isNaN(parsed.getTime())) {
    return date;
  }
  return parsed.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long' });
}

// «3 августа, понедельник»
export function formatPrototypeDateWithWeekday(date: string): string {
  const parsed = parseISODate(date);
  if (Number.isNaN(parsed.getTime())) {
    return date;
  }
  return `${formatPrototypeDate(date)}, ${WEEKDAY_FULL_BY_GETDAY[parsed.getDay()]}`;
}

// «12 авг, ср»
export function formatPrototypeDateShort(date: string): string {
  const parsed = parseISODate(date);
  if (Number.isNaN(parsed.getTime())) {
    return date;
  }
  return `${parsed.getDate()} ${MONTH_GENITIVE_SHORT[parsed.getMonth()]}, ${WEEKDAY_SHORT_BY_GETDAY[parsed.getDay()]}`;
}

// Короткий день недели («Пн») для строковой даты.
export function prototypeWeekdayShort(date: string): string {
  const parsed = parseISODate(date);
  return PROTOTYPE_WEEKDAY_SHORTS[(parsed.getDay() + 6) % 7];
}

export function pluralizePrototypeReminders(count: number): string {
  if (count === 1) {
    return 'напоминание';
  }
  if (count >= 2 && count <= 4) {
    return 'напоминания';
  }
  return 'напоминаний';
}
