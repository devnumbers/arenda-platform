// ПРОТОТИП (throwaway), вариант B — «Отдельный экран задач».
// Задачи объекта как полноценный экран: сегмент-фильтр, список слева, календарь справа,
// редактор — боковая панель (drawer) со структурированной формой и повтором-«предложением».

'use client';

import { useMemo, useState, type JSX } from 'react';
import clsx from 'clsx';
import type { TaskDraft } from '../model/types';
import { PROTOTYPE_PROPERTY } from '../model/fixtures';
import {
    addDaysISO,
    formatDateHuman,
    formatRecurrence,
    todayISO,
} from '../lib/recurrence';
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
import type { VariantProps } from './VariantA';
import styles from './VariantB.module.css';

const MONTH_SHORT: readonly string[] = [
    'янв', 'фев', 'мар', 'апр', 'мая', 'июн',
    'июл', 'авг', 'сен', 'окт', 'ноя', 'дек',
];

function dateBlock(dateISO: string): { day: string; month: string } {
    return {
        day: String(Number(dateISO.slice(8, 10))),
        month: MONTH_SHORT[Number(dateISO.slice(5, 7)) - 1],
    };
}

export function VariantB({ tasks, dispatch }: VariantProps): JSX.Element {
    const today = todayISO();
    const [tab, setTab] = useState<'active' | 'completed'>('active');
    const [editor, setEditor] = useState<EditorRequest | null>(null);
    const [scopeTarget, setScopeTarget] = useState<Occurrence | null>(null);
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
            setScopeTarget(occ);
        } else {
            setEditor({ kind: 'edit', task: occ.task, scope: 'all' });
        }
    };

    return (
        <div className={styles.root}>
            <header className={styles.header}>
                <div className={styles.headerMain}>
                    <h1 className={styles.title}>Задачи</h1>
                    <span className={styles.propertyChip}>{PROTOTYPE_PROPERTY.name}</span>
                </div>
                <button
                    type="button"
                    className={styles.newButton}
                    onClick={() => setEditor({ kind: 'new', defaultDate: today })}
                >
                    + Новая задача
                </button>
            </header>

            <div className={styles.tabs} role="tablist">
                <button
                    type="button"
                    role="tab"
                    aria-selected={tab === 'active'}
                    className={clsx(styles.tab, tab === 'active' && styles.tabSelected)}
                    onClick={() => setTab('active')}
                >
                    Активные · {active.length}
                </button>
                <button
                    type="button"
                    role="tab"
                    aria-selected={tab === 'completed'}
                    className={clsx(styles.tab, tab === 'completed' && styles.tabSelected)}
                    onClick={() => setTab('completed')}
                >
                    Выполненные · {completed.length}
                </button>
            </div>

            <div className={styles.columns}>
                <div className={styles.listPane}>
                    {tab === 'active' ? (
                        <ul className={styles.rows}>
                            {active.map((row) => {
                                const block = dateBlock(row.date);
                                return (
                                    <li key={row.task.id} className={styles.rowCard}>
                                        <button
                                            type="button"
                                            className={styles.rowMain}
                                            onClick={() =>
                                                row.task.recurrence
                                                    ? setScopeTarget({ task: row.task, date: row.date })
                                                    : setEditor({ kind: 'edit', task: row.task, scope: 'all' })
                                            }
                                        >
                                            <span
                                                className={clsx(
                                                    styles.dateBlock,
                                                    row.overdue && styles.dateBlockOverdue,
                                                )}
                                            >
                                                <span className={styles.dateDay}>{block.day}</span>
                                                <span className={styles.dateMonth}>{block.month}</span>
                                            </span>
                                            <span className={styles.rowText}>
                                                <span className={styles.rowTitle}>{row.task.title}</span>
                                                <span className={styles.rowSub}>
                                                    {row.overdue && (
                                                        <span className={styles.overdueLabel}>просрочено · </span>
                                                    )}
                                                    {row.task.dueTime ? `${row.task.dueTime} · ` : ''}
                                                    {row.task.recurrence
                                                        ? `↻ ${formatRecurrence(row.task.recurrence, row.task.dueDate)}`
                                                        : 'одноразовая'}
                                                </span>
                                                {row.task.description && (
                                                    <span className={styles.rowNote}>{row.task.description}</span>
                                                )}
                                            </span>
                                        </button>
                                        <button
                                            type="button"
                                            className={styles.doneAction}
                                            onClick={() =>
                                                dispatch({
                                                    type: 'toggle',
                                                    id: row.task.id,
                                                    occurrenceDate: row.date,
                                                })
                                            }
                                        >
                                            ✓ Выполнить
                                        </button>
                                    </li>
                                );
                            })}
                            {active.length === 0 && <li className={styles.empty}>Все задачи выполнены</li>}
                        </ul>
                    ) : (
                        <ul className={styles.rows}>
                            {completed.map((row) => (
                                <li key={`${row.task.id}-${row.date}`} className={styles.rowCard}>
                                    <div className={styles.rowMain}>
                                        <span className={styles.dateBlockDone}>
                                            ✓
                                        </span>
                                        <span className={styles.rowText}>
                                            <span className={clsx(styles.rowTitle, styles.rowTitleDone)}>
                                                {row.task.title}
                                            </span>
                                            <span className={styles.rowSub}>
                                                {formatDateHuman(row.date)}
                                                {row.task.dueTime ? `, ${row.task.dueTime}` : ''}
                                            </span>
                                        </span>
                                    </div>
                                    <button
                                        type="button"
                                        className={styles.undoAction}
                                        onClick={() =>
                                            dispatch({
                                                type: 'toggle',
                                                id: row.task.id,
                                                occurrenceDate: row.date,
                                            })
                                        }
                                    >
                                        Вернуть
                                    </button>
                                </li>
                            ))}
                            {completed.length === 0 && (
                                <li className={styles.empty}>Выполненных пока нет</li>
                            )}
                        </ul>
                    )}
                </div>

                <aside className={styles.calendarPane}>
                    <PrototypeMonthCalendar
                        counts={counts}
                        selectedDate={selectedDate}
                        onSelectDate={setSelectedDate}
                    />
                    <div className={styles.dayList}>
                        <h3 className={styles.dayTitle}>{formatDateHuman(selectedDate)}</h3>
                        {dayOccurrences.length === 0 && (
                            <p className={styles.empty}>Нет задач на этот день</p>
                        )}
                        {dayOccurrences.map((occ) => {
                            const done = isDone(occ.task, occ.date);
                            return (
                                <div key={`${occ.task.id}-${occ.date}`} className={styles.dayRow}>
                                    <button
                                        type="button"
                                        className={clsx(styles.miniCircle, done && styles.miniCircleDone)}
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
                                        className={styles.dayRowBody}
                                        onClick={() => openOccurrence(occ)}
                                    >
                                        <span className={clsx(done && styles.rowTitleDone)}>
                                            {occ.task.dueTime ? `${occ.task.dueTime} · ` : ''}
                                            {occ.task.title}
                                        </span>
                                    </button>
                                </div>
                            );
                        })}
                    </div>
                </aside>
            </div>

            {scopeTarget && (
                <ScopeDialog
                    actionLabel="Изменить задачу"
                    onClose={() => setScopeTarget(null)}
                    onChoose={(scope) => {
                        setEditor({
                            kind: 'edit',
                            task: scopeTarget.task,
                            scope,
                            occurrenceDate: scopeTarget.date,
                        });
                        setScopeTarget(null);
                    }}
                />
            )}

            {editor && (
                <DrawerEditor
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

const FREQ_UNITS = {
    daily: ['день', 'дня', 'дней'],
    weekly: ['неделю', 'недели', 'недель'],
    monthly: ['месяц', 'месяца', 'месяцев'],
    yearly: ['год', 'года', 'лет'],
} as const;

function plural(n: number, forms: readonly [string, string, string]): string {
    const mod10 = n % 10;
    const mod100 = n % 100;
    if (mod10 === 1 && mod100 !== 11) return forms[0];
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return forms[1];
    return forms[2];
}

function DrawerEditor({
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
                className={styles.drawer}
                role="dialog"
                aria-modal="true"
                aria-label={request.kind === 'new' ? 'Новая задача' : 'Изменить задачу'}
                onClick={(e) => e.stopPropagation()}
            >
                <header className={styles.drawerHeader}>
                    <h3 className={styles.drawerTitle}>
                        {request.kind === 'new' ? 'Новая задача' : 'Изменить задачу'}
                    </h3>
                    <button type="button" className={styles.drawerClose} aria-label="Закрыть" onClick={onClose}>
                        ✕
                    </button>
                </header>

                <div className={styles.drawerBody}>
                    {request.kind === 'edit' && task?.recurrence && (
                        <p className={styles.scopeNote}>
                            {request.scope === 'this' &&
                                'Одно вхождение: после сохранения серия разорвётся, это вхождение станет отдельной задачей.'}
                            {request.scope === 'this_and_future' &&
                                'Эта и следующие: серия обрежется до этой даты и продолжится с новыми параметрами.'}
                            {request.scope === 'all' &&
                                'Вся серия: меняются только название и описание, расписание заблокировано.'}
                        </p>
                    )}

                    <label className={styles.field}>
                        <span className={styles.fieldLabel}>Название</span>
                        <input
                            className={clsx(styles.input, titleError && styles.inputError)}
                            value={title}
                            placeholder="Например, передать показания"
                            onChange={(e) => {
                                setTitle(e.currentTarget.value);
                                setTitleError(false);
                            }}
                        />
                        {titleError && <span className={styles.fieldError}>Введите название</span>}
                    </label>

                    <label className={styles.field}>
                        <span className={styles.fieldLabel}>Описание</span>
                        <textarea
                            className={styles.input}
                            rows={3}
                            value={description}
                            onChange={(e) => setDescription(e.currentTarget.value)}
                        />
                    </label>

                    <div className={styles.fieldGrid}>
                        <label className={styles.field}>
                            <span className={styles.fieldLabel}>Дата</span>
                            <input
                                type="date"
                                className={styles.input}
                                value={date}
                                disabled={scheduleLocked}
                                onChange={(e) => setDate(e.currentTarget.value)}
                            />
                        </label>
                        <label className={styles.field}>
                            <span className={styles.fieldLabel}>Время (необязательно)</span>
                            <input
                                type="time"
                                className={styles.input}
                                value={time}
                                disabled={scheduleLocked}
                                onChange={(e) => setTime(e.currentTarget.value)}
                            />
                        </label>
                    </div>

                    <fieldset className={styles.repeatBlock} disabled={scheduleLocked}>
                        <legend className={styles.fieldLabel}>Повтор</legend>
                        <div className={styles.sentence}>
                            <span>Каждый</span>
                            <input
                                type="number"
                                min={1}
                                className={styles.sentenceNumber}
                                value={repeat.interval}
                                onChange={(e) =>
                                    setRepeat({
                                        ...repeat,
                                        enabled: true,
                                        interval: Math.max(1, Number(e.currentTarget.value) || 1),
                                    })
                                }
                            />
                            <select
                                className={styles.sentenceSelect}
                                value={repeat.enabled ? repeat.freq : 'none'}
                                onChange={(e) => {
                                    const value = e.currentTarget.value;
                                    if (value === 'none') {
                                        setRepeat({ ...repeat, enabled: false });
                                    } else {
                                        setRepeat({
                                            ...repeat,
                                            enabled: true,
                                            freq: value as RepeatEditorState['freq'],
                                        });
                                    }
                                }}
                            >
                                <option value="none">— не повторяется —</option>
                                <option value="daily">
                                    {plural(repeat.interval, FREQ_UNITS.daily)}
                                </option>
                                <option value="weekly">
                                    {plural(repeat.interval, FREQ_UNITS.weekly)}
                                </option>
                                <option value="monthly">
                                    {plural(repeat.interval, FREQ_UNITS.monthly)}
                                </option>
                                <option value="yearly">
                                    {plural(repeat.interval, FREQ_UNITS.yearly)}
                                </option>
                            </select>
                        </div>

                        {repeat.enabled && repeat.freq === 'weekly' && (
                            <div className={styles.repeatDetail}>
                                <span className={styles.detailLabel}>по дням</span>
                                <WeekdayChips
                                    value={repeat.weekdays}
                                    onChange={(weekdays) => setRepeat({ ...repeat, weekdays })}
                                />
                            </div>
                        )}

                        {repeat.enabled && repeat.freq === 'monthly' && (
                            <div className={styles.repeatDetail}>
                                <span className={styles.detailLabel}>по числам месяца</span>
                                <MonthDayGrid
                                    value={repeat.monthDays}
                                    onChange={(monthDays) => setRepeat({ ...repeat, monthDays })}
                                />
                            </div>
                        )}

                        {repeat.enabled && (
                            <div className={styles.repeatDetail}>
                                <span className={styles.detailLabel}>окончание</span>
                                <div className={styles.sentence}>
                                    <select
                                        className={styles.sentenceSelect}
                                        value={repeat.end}
                                        onChange={(e) =>
                                            setRepeat({
                                                ...repeat,
                                                end: e.currentTarget.value as RepeatEditorState['end'],
                                            })
                                        }
                                    >
                                        <option value="never">без окончания</option>
                                        <option value="until">до даты</option>
                                        <option value="count">после числа повторов</option>
                                    </select>
                                    {repeat.end === 'until' && (
                                        <input
                                            type="date"
                                            className={styles.input}
                                            value={repeat.untilDate}
                                            onChange={(e) =>
                                                setRepeat({ ...repeat, untilDate: e.currentTarget.value })
                                            }
                                        />
                                    )}
                                    {repeat.end === 'count' && (
                                        <input
                                            type="number"
                                            min={1}
                                            className={styles.sentenceNumber}
                                            value={repeat.countNum}
                                            onChange={(e) =>
                                                setRepeat({
                                                    ...repeat,
                                                    countNum: Math.max(1, Number(e.currentTarget.value) || 1),
                                                })
                                            }
                                        />
                                    )}
                                </div>
                            </div>
                        )}

                        {repeat.enabled && (
                            <p className={styles.preview}>{repeatSummary(repeat, date)}</p>
                        )}
                    </fieldset>
                </div>

                <footer className={styles.drawerFooter}>
                    {request.kind === 'edit' && (
                        <button type="button" className={styles.deleteButton} onClick={onDelete}>
                            Удалить
                        </button>
                    )}
                    <span className={styles.footerSpacer} />
                    <button type="button" className={styles.cancelButton} onClick={onClose}>
                        Отмена
                    </button>
                    <button type="button" className={styles.saveButton} onClick={save}>
                        Сохранить
                    </button>
                </footer>
            </div>
        </div>
    );
}
