// ПРОТОТИП (throwaway), вариант A — «Секция в объекте».
// Задачи живут секцией на странице объекта (в духе Apple Reminders): круг-чекбокс,
// быстрое добавление, редактор в нижнем шите, редактор повтора — drill-down экран.

'use client';

import { useMemo, useState, type JSX } from 'react';
import clsx from 'clsx';
import type { PrototypeTask, TaskDraft } from '../model/types';
import { PROTOTYPE_PROPERTY } from '../model/fixtures';
import type { TasksAction } from '../model/store';
import { addDaysISO, formatDateHuman, formatRecurrence, todayISO } from '../lib/recurrence';
import {
    activeRows,
    completedRows,
    isDone,
    occurrenceCounts,
    occurrencesInWindow,
    type Occurrence,
} from './view-model';
import { PrototypeMonthCalendar } from './PrototypeMonthCalendar';
import { ScopeDialog } from './ScopeDialog';
import { MonthDayGrid, WeekdayChips } from './controls';
import type { EditorRequest } from './editor-request';
import {
    repeatStateFromRule,
    repeatStateToRule,
    repeatSummary,
    type RepeatEditorState,
} from './repeat-state';
import styles from './VariantA.module.css';

export type VariantProps = {
    readonly tasks: readonly PrototypeTask[];
    readonly dispatch: (action: TasksAction) => void;
};

type ScopeRequest = {
    readonly task: PrototypeTask;
    readonly occurrenceDate: string;
};

function formatRowDate(dateISO: string, todayISO_: string): string {
    if (dateISO === todayISO_) return 'Сегодня';
    if (dateISO === addDaysISO(todayISO_, 1)) return 'Завтра';
    if (dateISO === addDaysISO(todayISO_, -1)) return 'Вчера';
    return formatDateHuman(dateISO);
}

export function VariantA({ tasks, dispatch }: VariantProps): JSX.Element {
    const today = todayISO();
    const [editor, setEditor] = useState<EditorRequest | null>(null);
    const [scopeRequest, setScopeRequest] = useState<ScopeRequest | null>(null);
    const [completedOpen, setCompletedOpen] = useState(false);
    const [quickTitle, setQuickTitle] = useState('');
    const [selectedDate, setSelectedDate] = useState(today);

    const active = useMemo(() => activeRows(tasks, today), [tasks, today]);
    const completed = useMemo(() => completedRows(tasks), [tasks]);
    const counts = useMemo(
        () => occurrenceCounts(tasks, addDaysISO(today, -400), addDaysISO(today, 800)),
        [tasks, today],
    );
    const dayOccurrences = useMemo(
        () => occurrencesInWindow(tasks, selectedDate, addDaysISO(selectedDate, 1)),
        [tasks, selectedDate],
    );

    const openOccurrence = (occ: Occurrence): void => {
        if (occ.task.recurrence) {
            setScopeRequest({ task: occ.task, occurrenceDate: occ.date });
        } else {
            setEditor({ kind: 'edit', task: occ.task, scope: 'all' });
        }
    };

    const quickAdd = (): void => {
        const title = quickTitle.trim();
        if (!title) return;
        dispatch({ type: 'add', draft: { title, dueDate: today } });
        setQuickTitle('');
    };

    return (
        <div className={styles.root}>
            <div className={styles.propertyCard}>
                <span className={styles.propertyName}>{PROTOTYPE_PROPERTY.name}</span>
                <span className={styles.propertyAddress}>{PROTOTYPE_PROPERTY.address}</span>
                <div className={styles.skeletonRow} />
                <div className={styles.skeletonRowShort} />
            </div>

            <section className={styles.card} aria-label="Задачи объекта">
                <header className={styles.cardHeader}>
                    <h2 className={styles.cardTitle}>Задачи</h2>
                    <span className={styles.counter}>{active.length}</span>
                </header>

                <div className={styles.quickAdd}>
                    <button
                        type="button"
                        className={styles.quickAddPlus}
                        aria-label="Новая задача с полной формой"
                        title="Открыть полную форму"
                        onClick={() => setEditor({ kind: 'new', defaultDate: today })}
                    >
                        +
                    </button>
                    <input
                        className={styles.quickAddInput}
                        placeholder="Новая задача"
                        value={quickTitle}
                        onChange={(e) => setQuickTitle(e.currentTarget.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') quickAdd();
                        }}
                    />
                    <button
                        type="button"
                        className={styles.quickAddButton}
                        onClick={quickAdd}
                        disabled={!quickTitle.trim()}
                    >
                        Добавить
                    </button>
                </div>

                <ul className={styles.list}>
                    {active.map((row) => (
                        <li key={row.task.id} className={styles.row}>
                            <button
                                type="button"
                                className={styles.circle}
                                aria-label="Выполнить"
                                onClick={() =>
                                    dispatch({
                                        type: 'toggle',
                                        id: row.task.id,
                                        occurrenceDate: row.date,
                                    })
                                }
                            />
                            <button
                                type="button"
                                className={styles.rowBody}
                                onClick={() =>
                                    row.task.recurrence
                                        ? setScopeRequest({ task: row.task, occurrenceDate: row.date })
                                        : setEditor({ kind: 'edit', task: row.task, scope: 'all' })
                                }
                            >
                                <span className={styles.rowTitle}>{row.task.title}</span>
                                <span className={styles.rowMeta}>
                                    <span className={clsx(styles.dateChip, row.overdue && styles.dateChipOverdue)}>
                                        {formatRowDate(row.date, today)}
                                        {row.task.dueTime ? `, ${row.task.dueTime}` : ''}
                                    </span>
                                    {row.task.recurrence && (
                                        <span className={styles.repeatChip}>
                                            ↻ {formatRecurrence(row.task.recurrence, row.task.dueDate)}
                                        </span>
                                    )}
                                </span>
                                {row.task.description && (
                                    <span className={styles.rowNote}>{row.task.description}</span>
                                )}
                            </button>
                        </li>
                    ))}
                    {active.length === 0 && (
                        <li className={styles.empty}>Задач нет — добавьте первую</li>
                    )}
                </ul>

                <button
                    type="button"
                    className={styles.completedToggle}
                    aria-expanded={completedOpen}
                    onClick={() => setCompletedOpen((v) => !v)}
                >
                    Выполненные ({completed.length}) {completedOpen ? '▾' : '▸'}
                </button>
                {completedOpen && (
                    <ul className={clsx(styles.list, styles.listCompleted)}>
                        {completed.map((row) => (
                            <li key={`${row.task.id}-${row.date}`} className={styles.row}>
                                <button
                                    type="button"
                                    className={clsx(styles.circle, styles.circleDone)}
                                    aria-label="Вернуть в активные"
                                    onClick={() =>
                                        dispatch({
                                            type: 'toggle',
                                            id: row.task.id,
                                            occurrenceDate: row.date,
                                        })
                                    }
                                >
                                    ✓
                                </button>
                                <div className={styles.rowBody}>
                                    <span className={clsx(styles.rowTitle, styles.rowTitleDone)}>
                                        {row.task.title}
                                    </span>
                                    <span className={styles.rowMeta}>
                                        <span className={styles.dateChip}>{formatRowDate(row.date, today)}</span>
                                    </span>
                                </div>
                            </li>
                        ))}
                    </ul>
                )}
            </section>

            <section className={styles.card} aria-label="Календарь задач">
                <header className={styles.cardHeader}>
                    <h2 className={styles.cardTitle}>Календарь</h2>
                </header>
                <PrototypeMonthCalendar
                    counts={counts}
                    selectedDate={selectedDate}
                    onSelectDate={setSelectedDate}
                />
                <ul className={styles.dayList}>
                    {dayOccurrences.map((occ) => {
                        const done = isDone(occ.task, occ.date);
                        return (
                            <li key={`${occ.task.id}-${occ.date}`} className={styles.row}>
                                <button
                                    type="button"
                                    className={clsx(styles.circle, done && styles.circleDone)}
                                    aria-label={done ? 'Вернуть' : 'Выполнить'}
                                    onClick={() =>
                                        dispatch({
                                            type: 'toggle',
                                            id: occ.task.id,
                                            occurrenceDate: occ.date,
                                        })
                                    }
                                >
                                    {done ? '✓' : ''}
                                </button>
                                <button
                                    type="button"
                                    className={styles.rowBody}
                                    onClick={() => openOccurrence(occ)}
                                >
                                    <span className={clsx(styles.rowTitle, done && styles.rowTitleDone)}>
                                        {occ.task.recurrence && <span aria-hidden="true">↻ </span>}
                                        {occ.task.title}
                                    </span>
                                    <span className={styles.rowMeta}>
                                        <span className={styles.dateChip}>
                                            {occ.task.dueTime ?? 'весь день'}
                                        </span>
                                    </span>
                                </button>
                            </li>
                        );
                    })}
                    {dayOccurrences.length === 0 && (
                        <li className={styles.empty}>На этот день задач нет</li>
                    )}
                </ul>
            </section>

            {scopeRequest && (
                <ScopeDialog
                    actionLabel="Изменить задачу"
                    onClose={() => setScopeRequest(null)}
                    onChoose={(scope) => {
                        setEditor({
                            kind: 'edit',
                            task: scopeRequest.task,
                            scope,
                            occurrenceDate: scopeRequest.occurrenceDate,
                        });
                        setScopeRequest(null);
                    }}
                />
            )}

            {editor && (
                <SheetEditor
                    request={editor}
                    onClose={() => setEditor(null)}
                    onSave={(draft) => {
                        if (editor.kind === 'new') {
                            dispatch({ type: 'add', draft });
                        } else {
                            dispatch({
                                type: 'update',
                                id: editor.task.id,
                                draft,
                                scope: editor.scope,
                                occurrenceDate: editor.occurrenceDate,
                            });
                        }
                        setEditor(null);
                    }}
                    onDelete={() => {
                        if (editor.kind === 'edit') {
                            dispatch({
                                type: 'remove',
                                id: editor.task.id,
                                scope: editor.scope,
                                occurrenceDate: editor.occurrenceDate,
                            });
                        }
                        setEditor(null);
                    }}
                />
            )}
        </div>
    );
}

const FREQ_LABELS = { daily: 'День', weekly: 'Неделя', monthly: 'Месяц', yearly: 'Год' } as const;

type PresetKey = 'none' | 'daily' | 'weekly' | 'monthly' | 'yearly' | 'custom';

function presetOf(repeat: RepeatEditorState): PresetKey {
    if (!repeat.enabled) return 'none';
    const simple = repeat.interval === 1 && repeat.end === 'never';
    if (simple && repeat.freq === 'daily') return 'daily';
    if (simple && repeat.freq === 'weekly' && repeat.weekdays.length <= 1) return 'weekly';
    if (simple && repeat.freq === 'monthly' && repeat.monthDays.length <= 1) return 'monthly';
    if (simple && repeat.freq === 'yearly') return 'yearly';
    return 'custom';
}

function SheetEditor({
    request,
    onClose,
    onSave,
    onDelete,
}: {
    readonly request: EditorRequest;
    readonly onClose: () => void;
    readonly onSave: (draft: TaskDraft) => void;
    readonly onDelete: () => void;
}): JSX.Element {
    const task = request.kind === 'edit' ? request.task : undefined;
    const initialDate =
        request.kind === 'edit'
            ? (request.occurrenceDate ?? request.task.dueDate)
            : request.defaultDate;

    const [screen, setScreen] = useState<'main' | 'repeat'>('main');
    const [title, setTitle] = useState(task?.title ?? '');
    const [description, setDescription] = useState(task?.description ?? '');
    const [date, setDate] = useState(initialDate);
    const [time, setTime] = useState(task?.dueTime ?? '');
    const [repeat, setRepeat] = useState<RepeatEditorState>(() =>
        repeatStateFromRule(task?.recurrence, initialDate),
    );
    const [titleError, setTitleError] = useState(false);

    const scheduleLocked =
        request.kind === 'edit' && Boolean(task?.recurrence) && request.scope === 'all';

    const scopeBanner =
        request.kind === 'edit' && task?.recurrence
            ? request.scope === 'this'
                ? 'Правится одно вхождение: серия разорвётся, это вхождение станет отдельной задачей'
                : request.scope === 'this_and_future'
                  ? 'Правится эта и следующие: прошлые вхождения останутся как были'
                  : 'Правится вся серия: доступны только название и описание'
            : null;

    const save = (): void => {
        if (!title.trim()) {
            setTitleError(true);
            return;
        }
        onSave({
            title: title.trim(),
            description: description.trim() || undefined,
            dueDate: date,
            dueTime: time || undefined,
            recurrence: scheduleLocked ? task?.recurrence : repeatStateToRule(repeat),
        });
    };

    return (
        <div className={styles.overlay} onClick={onClose} role="presentation">
            <div
                className={styles.sheet}
                role="dialog"
                aria-modal="true"
                aria-label={request.kind === 'new' ? 'Новая задача' : 'Изменить задачу'}
                onClick={(e) => e.stopPropagation()}
            >
                {screen === 'main' ? (
                    <>
                        <header className={styles.sheetHeader}>
                            <button type="button" className={styles.sheetCancel} onClick={onClose}>
                                Отмена
                            </button>
                            <h3 className={styles.sheetTitle}>
                                {request.kind === 'new' ? 'Новая задача' : 'Изменить'}
                            </h3>
                            <button type="button" className={styles.sheetSave} onClick={save}>
                                Сохранить
                            </button>
                        </header>

                        {scopeBanner && <p className={styles.scopeBanner}>{scopeBanner}</p>}

                        <div className={styles.fields}>
                            <input
                                className={clsx(styles.titleInput, titleError && styles.titleInputError)}
                                placeholder="Название"
                                value={title}
                                onChange={(e) => {
                                    setTitle(e.currentTarget.value);
                                    setTitleError(false);
                                }}
                            />
                            <textarea
                                className={styles.noteInput}
                                placeholder="Описание"
                                rows={2}
                                value={description}
                                onChange={(e) => setDescription(e.currentTarget.value)}
                            />

                            <div className={styles.fieldRow}>
                                <span className={styles.fieldLabel}>Дата</span>
                                <input
                                    type="date"
                                    className={styles.fieldInput}
                                    value={date}
                                    disabled={scheduleLocked}
                                    onChange={(e) => setDate(e.currentTarget.value)}
                                />
                            </div>
                            <div className={styles.fieldRow}>
                                <span className={styles.fieldLabel}>Время</span>
                                <input
                                    type="time"
                                    className={styles.fieldInput}
                                    value={time}
                                    disabled={scheduleLocked}
                                    onChange={(e) => setTime(e.currentTarget.value)}
                                />
                                {time && !scheduleLocked && (
                                    <button
                                        type="button"
                                        className={styles.clearTime}
                                        onClick={() => setTime('')}
                                    >
                                        ✕
                                    </button>
                                )}
                            </div>
                            <button
                                type="button"
                                className={clsx(styles.fieldRow, styles.repeatRow)}
                                disabled={scheduleLocked}
                                onClick={() => setScreen('repeat')}
                            >
                                <span className={styles.fieldLabel}>Повтор</span>
                                <span className={styles.repeatSummary}>
                                    {scheduleLocked
                                        ? formatRecurrence(task!.recurrence!, task!.dueDate)
                                        : repeatSummary(repeat, date)}
                                </span>
                                {!scheduleLocked && <span aria-hidden="true">›</span>}
                            </button>
                        </div>

                        {request.kind === 'edit' && (
                            <button type="button" className={styles.deleteButton} onClick={onDelete}>
                                {request.scope === 'this'
                                    ? 'Удалить это вхождение'
                                    : request.scope === 'this_and_future'
                                      ? 'Удалить это и следующие'
                                      : 'Удалить задачу'}
                            </button>
                        )}
                    </>
                ) : (
                    <>
                        <header className={styles.sheetHeader}>
                            <button
                                type="button"
                                className={styles.sheetCancel}
                                onClick={() => setScreen('main')}
                            >
                                ‹ Назад
                            </button>
                            <h3 className={styles.sheetTitle}>Повтор</h3>
                            <span className={styles.sheetSave} />
                        </header>

                        <div className={styles.fields}>
                            <div className={styles.presetList} role="radiogroup" aria-label="Пресеты повтора">
                                {(
                                    [
                                        ['none', 'Не повторяется'],
                                        ['daily', 'Каждый день'],
                                        ['weekly', 'Каждую неделю'],
                                        ['monthly', 'Каждый месяц'],
                                        ['yearly', 'Каждый год'],
                                        ['custom', 'Свой вариант'],
                                    ] as const
                                ).map(([key, label]) => {
                                    const selected = presetOf(repeat) === key;
                                    return (
                                        <button
                                            key={key}
                                            type="button"
                                            role="radio"
                                            aria-checked={selected}
                                            className={clsx(styles.presetRow, selected && styles.presetRowSelected)}
                                            onClick={() => {
                                                if (key === 'none') {
                                                    setRepeat({ ...repeat, enabled: false });
                                                } else if (key === 'custom') {
                                                    setRepeat({ ...repeat, enabled: true });
                                                } else {
                                                    setRepeat({
                                                        ...repeat,
                                                        enabled: true,
                                                        freq: key,
                                                        interval: 1,
                                                        end: 'never',
                                                    });
                                                }
                                            }}
                                        >
                                            <span>{label}</span>
                                            {selected && <span aria-hidden="true">✓</span>}
                                        </button>
                                    );
                                })}
                            </div>

                            {repeat.enabled && (
                                <div className={styles.customBlock}>
                                    <div className={styles.customRow}>
                                        <span className={styles.fieldLabel}>Частота</span>
                                        <div className={styles.freqChips}>
                                            {(Object.keys(FREQ_LABELS) as (keyof typeof FREQ_LABELS)[]).map(
                                                (freq) => (
                                                    <button
                                                        key={freq}
                                                        type="button"
                                                        className={clsx(
                                                            styles.freqChip,
                                                            repeat.freq === freq && styles.freqChipSelected,
                                                        )}
                                                        onClick={() => setRepeat({ ...repeat, freq })}
                                                    >
                                                        {FREQ_LABELS[freq]}
                                                    </button>
                                                ),
                                            )}
                                        </div>
                                    </div>

                                    <div className={styles.customRow}>
                                        <span className={styles.fieldLabel}>Интервал</span>
                                        <div className={styles.stepper}>
                                            <button
                                                type="button"
                                                className={styles.stepperButton}
                                                aria-label="Меньше"
                                                onClick={() =>
                                                    setRepeat({
                                                        ...repeat,
                                                        interval: Math.max(1, repeat.interval - 1),
                                                    })
                                                }
                                            >
                                                −
                                            </button>
                                            <span className={styles.stepperValue}>{repeat.interval}</span>
                                            <button
                                                type="button"
                                                className={styles.stepperButton}
                                                aria-label="Больше"
                                                onClick={() =>
                                                    setRepeat({ ...repeat, interval: repeat.interval + 1 })
                                                }
                                            >
                                                +
                                            </button>
                                        </div>
                                    </div>

                                    {repeat.freq === 'weekly' && (
                                        <div className={styles.customRowColumn}>
                                            <span className={styles.fieldLabel}>Дни недели</span>
                                            <WeekdayChips
                                                value={repeat.weekdays}
                                                onChange={(weekdays) => setRepeat({ ...repeat, weekdays })}
                                            />
                                        </div>
                                    )}

                                    {repeat.freq === 'monthly' && (
                                        <div className={styles.customRowColumn}>
                                            <span className={styles.fieldLabel}>Числа месяца</span>
                                            <MonthDayGrid
                                                value={repeat.monthDays}
                                                onChange={(monthDays) => setRepeat({ ...repeat, monthDays })}
                                            />
                                        </div>
                                    )}

                                    <div className={styles.customRowColumn}>
                                        <span className={styles.fieldLabel}>Окончание</span>
                                        <label className={styles.endOption}>
                                            <input
                                                type="radio"
                                                name="repeat-end-a"
                                                checked={repeat.end === 'never'}
                                                onChange={() => setRepeat({ ...repeat, end: 'never' })}
                                            />
                                            Никогда
                                        </label>
                                        <label className={styles.endOption}>
                                            <input
                                                type="radio"
                                                name="repeat-end-a"
                                                checked={repeat.end === 'until'}
                                                onChange={() => setRepeat({ ...repeat, end: 'until' })}
                                            />
                                            До даты
                                            <input
                                                type="date"
                                                className={styles.fieldInput}
                                                value={repeat.untilDate}
                                                disabled={repeat.end !== 'until'}
                                                onChange={(e) =>
                                                    setRepeat({ ...repeat, untilDate: e.currentTarget.value })
                                                }
                                            />
                                        </label>
                                        <label className={styles.endOption}>
                                            <input
                                                type="radio"
                                                name="repeat-end-a"
                                                checked={repeat.end === 'count'}
                                                onChange={() => setRepeat({ ...repeat, end: 'count' })}
                                            />
                                            После
                                            <input
                                                type="number"
                                                min={1}
                                                className={styles.countInput}
                                                value={repeat.countNum}
                                                disabled={repeat.end !== 'count'}
                                                onChange={(e) =>
                                                    setRepeat({
                                                        ...repeat,
                                                        countNum: Math.max(1, Number(e.currentTarget.value) || 1),
                                                    })
                                                }
                                            />
                                            повторов
                                        </label>
                                    </div>

                                    <p className={styles.repeatPreview}>{repeatSummary(repeat, date)}</p>
                                </div>
                            )}

                            <button
                                type="button"
                                className={styles.doneButton}
                                onClick={() => setScreen('main')}
                            >
                                Готово
                            </button>
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
