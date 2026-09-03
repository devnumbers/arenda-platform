/**
 * Доменная модель контекста «Задачи» на фронте (словарь — tasks/CONTEXT.md,
 * ADR 0051): правило порождает вхождения-задачи, задачу выполняют вручную;
 * просрочка и «без срока» — серверно-вычисляемые статусы на чтении. Даты —
 * строки 'YYYY-MM-DD' без времени и зон: «сегодня» для интерфейса считает
 * сервер по TZ собственника объекта (ADR 0048), клиент зоны не знает —
 * граница секций «Сегодня»/«Завтра» приходит в ответе листинга.
 */

/** Дата-строка 'YYYY-MM-DD' (как у платежей — см. entities/payment). */
export type IsoDate = string;

/** Повтор правила (TEXT-enum схемы; параметров повтора в v1 нет). */
export type TaskRepeat = 'once' | 'daily' | 'weekly' | 'monthly' | 'yearly';

/** Серверно-вычисляемый бакет задачи на чтение. */
export type TaskStatus = 'active' | 'overdue' | 'undated' | 'completed';

/**
 * Задача — вхождение правила: единица выполнения. Название, комментарий,
 * срок и время — снимки правила на момент материализации; repeat —
 * read-проекция живого правила для ↻ строки (null после удаления правила).
 */
export type Task = {
  readonly id: string;
  readonly propertyId: string;
  /** Правило-источник; null — строка журнала удалённого правила. */
  readonly ruleId: string | null;
  readonly dueDate: IsoDate | null;
  /** HH:MM; null — задача на весь день (время требует даты). */
  readonly dueTime: string | null;
  readonly title: string;
  readonly comment: string | null;
  readonly repeat: TaskRepeat | null;
  /** Дата выполнения — факт; null у активных. */
  readonly completedDate: IsoDate | null;
  readonly status: TaskStatus;
  readonly createdAt: string;
  readonly updatedAt: string;
};

/**
 * Правило — источник задач (словарь #494): настройка, порождающая
 * вхождения; её и правит экран «Изменить задачу» (#502). Состояний
 * «просрочено/выполнено» у правила нет — это состояния его задач.
 */
export type TaskRule = {
  readonly id: string;
  readonly propertyId: string;
  readonly title: string;
  readonly comment: string | null;
  /** Якорь — дата первого вхождения; null — правило без даты (только once). */
  readonly dueDate: IsoDate | null;
  /** HH:MM; null — на весь день. */
  readonly dueTime: string | null;
  readonly repeat: TaskRepeat;
  readonly createdAt: string;
  readonly updatedAt: string;
};

/** Страница листинга задач (активные или журнал) с серверным today. */
export type TasksPage = {
  readonly items: ReadonlyArray<Task>;
  readonly total: number;
  readonly today: IsoDate;
};
