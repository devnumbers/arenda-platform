// ПРОТОТИП (throwaway), вариант C — «Повестка и календарь».
// Календарь-центричный сценарий: неделя-лента + повестка дня; задача создаётся прямо
// под датой; правка одиночного вхождения — сразу из повестки; редактор — полный экран.

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
    weekdayOf,
} from '../lib/recurrence';
import { isDone, occurrencesInWindow, type Occurrence } from './view-model';
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
import styles from './VariantC.module.css';

const WEEKDAY_SHORT: readonly string[] = ['пн', 'вт', 'ср', 'чт', 'пт', 'сб', 'вс'];

export function VariantC({ tasks, dispatch }: VariantProps): JSX.Element {
    const today = todayISO();
    const [weekAnchor, setWeekAnchor] = useState(today);
    const [selectedDate, setSelectedDate] = useState(today);
    const [quickTitle, setQuickTitle] = useState('');
    const [editor, setEditor] = useState<EditorRequest | null>(null);
    const [scopeTarget, setScopeTarget] = useState<Occurrence | null>(null);

    const days = useMemo(() => {
        const start = addDaysISO(weekAnchor, -weekdayOf(weekAnchor));
        return Array.from({ length: 7 }, (_, i) => addDaysISO(start, i));
    }, [weekAnchor]);

    const weekOccurrences = useMemo(
        () => occurrencesInWindow(tasks, days[0], addDaysISO(days[6], 1)),
        [tasks, days],
    );
    const dayOccurrences = useMemo(
        () => weekOccurrences.filter((occ) => occ.date === selectedDate),
        [weekOccurrences, selectedDate],
    );

    const quickAdd = (): void => {
        const title = quickTitle.trim();
        if (!title) return;
        dispatch({ type: 'add', draft: { title, dueDate: selectedDate } });
        setQuickTitle('');
    };

    const openOccurrence = (occ: Occurrence): void => {
        if (occ.task.recurrence) {
            setScopeTarget(occ);
        } else {
            setEditor({ kind: 'edit', task: occ.task, scope: 'all' });
        }
    };

    if (editor) {
        return (
            <FullScreenEditor
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
        );
    }

    return (
        <div className={styles.root}>
            <header className={styles.header}>
                <h1 className={styles.title}>Задачи</h1>
                <span className={styles.propertyChip}>{PROTOTYPE_PROPERTY.name}</span>
                <button
                    type="button"
                    className={styles.newButton}
                    onClick={() => setEditor({ kind: 'new', defaultDate: selectedDate })}
                >
                    + Новая задача
                </button>
            </header>

            <div className={styles.weekNav}>
                <button
                    type="button"
                    className={styles.navButton}
                    aria-label="Предыдущая неделя"
                    onClick={() => setWeekAnchor(addDaysISO(weekAnchor, -7))}
                >
                    ‹
                </button>
                <button
                    type="button"
                    className={styles.navButton}
                    aria-label="Следующая неделя"
                    onClick={() => setWeekAnchor(addDaysISO(weekAnchor, 7))}
                >
                    ›
                </button>
                <button
                    type="button"
                    className={styles.todayButton}
                    onClick={() => {
                        setWeekAnchor(today);
                        setSelectedDate(today);
                    }}
                >
                    Сегодня
                </button>
            </div>

            <div className={styles.weekStrip}>
                {days.map((date) => {
                    const count = weekOccurrences.filter((occ) => occ.date === date).length;
                    return (
                        <button
                            key={date}
                            type="button"
                            aria-pressed={date === selectedDate}
                            className={clsx(
                                styles.dayCard,
                                date === selectedDate && styles.dayCardSelected,
                                date === today && styles.dayCardToday,
                            )}
                            onClick={() => setSelectedDate(date)}
                        >
                            <span className={styles.dayWeekday}>{WEEKDAY_SHORT[weekdayOf(date)]}</span>
                            <span className={styles.dayNumber}>{Number(date.slice(8, 10))}</span>
                            <span className={styles.dayDots}>
                                {Array.from({ length: Math.min(3, count) }, (_, i) => (
                                    <span key={i} className={styles.dot} />
                                ))}
                                {count > 0 && <span className={styles.dayCount}>{count}</span>}
                            </span>
                        </button>
                    );
                })}
            </div>

            <section className={styles.agenda} aria-label="Повестка дня">
                <h2 className={styles.agendaTitle}>{formatDateHuman(selectedDate)}</h2>

                <ul className={styles.agendaList}>
                    {dayOccurrences.map((occ) => {
                        const done = isDone(occ.task, occ.date);
                        return (
                            <li key={`${occ.task.id}-${occ.date}`} className={styles.agendaRow}>
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
                                    className={styles.agendaBody}
                                    onClick={() => openOccurrence(occ)}
                                >
                                    <span className={styles.agendaTime}>
                                        {occ.task.dueTime ?? 'весь день'}
                                    </span>
                                    <span className={styles.agendaText}>
                                        <span
                                            className={clsx(
                                                styles.agendaName,
                                                done && styles.agendaNameDone,
                                            )}
                                        >
                                            {occ.task.title}
                                        </span>
                                        {occ.task.recurrence && (
                                            <span className={styles.agendaRepeat}>
                                                ↻ {formatRecurrence(occ.task.recurrence, occ.task.dueDate)}
                                            </span>
                                        )}
                                        {occ.task.description && (
                                            <span className={styles.agendaNote}>{occ.task.description}</span>
                                        )}
                                    </span>
                                </button>
                            </li>
                        );
                    })}
                    {dayOccurrences.length === 0 && (
                        <li className={styles.empty}>На этот день задач нет</li>
                    )}
                </ul>

                <div className={styles.quickAdd}>
                    <span className={styles.quickAddPlus} aria-hidden="true">+</span>
                    <input
                        className={styles.quickAddInput}
                        placeholder={`Задача на ${formatDateHuman(selectedDate)}`}
                        value={quickTitle}
                        onChange={(e) => setQuickTitle(e.currentTarget.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') quickAdd();
                        }}
                    />
                    <button
                        type="button"
                        className={styles.quickAddButton}
                        disabled={!quickTitle.trim()}
                        onClick={quickAdd}
                    >
                        Добавить
                    </button>
                </div>
            </section>

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
        </div>
    );
}

const FREQ_CARDS = [
    { freq: 'daily', title: 'День', hint: 'каждый день или раз в несколько дней' },
    { freq: 'weekly', title: 'Неделя', hint: 'по выбранным дням недели' },
    { freq: 'monthly', title: 'Месяц', hint: 'по выбранным числам, например 11, 14 и 27' },
    { freq: 'yearly', title: 'Год', hint: 'в ту же дату каждый год' },
] as const;

function FullScreenEditor({
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
        <div className={styles.editorRoot}>
            <header className={styles.editorHeader}>
                <button type="button" className={styles.editorBack} onClick={onClose}>
                    ‹ Отмена
                </button>
                <h2 className={styles.editorTitle}>
                    {request.kind === 'new' ? 'Новая задача' : 'Изменить задачу'}
                </h2>
                <span className={styles.editorBackSpacer} />
            </header>

            {request.kind === 'edit' && task?.recurrence && (
                <p className={styles.scopeBanner}>
                    {request.scope === 'this' &&
                        'Одно вхождение: серия разорвётся, это вхождение станет отдельной задачей.'}
                    {request.scope === 'this_and_future' &&
                        'Эта и следующие: прошлые вхождения останутся без изменений.'}
                    {request.scope === 'all' &&
                        'Вся серия: меняются только название и описание.'}
                </p>
            )}

            <section className={styles.editorSection}>
                <h3 className={styles.sectionTitle}>Основное</h3>
                <input
                    className={clsx(styles.input, titleError && styles.inputError)}
                    placeholder="Название"
                    value={title}
                    onChange={(e) => {
                        setTitle(e.currentTarget.value);
                        setTitleError(false);
                    }}
                />
                {titleError && <span className={styles.fieldError}>Введите название</span>}
                <textarea
                    className={styles.input}
                    placeholder="Описание"
                    rows={3}
                    value={description}
                    onChange={(e) => setDescription(e.currentTarget.value)}
                />
            </section>

            <section className={styles.editorSection}>
                <h3 className={styles.sectionTitle}>Дата и время</h3>
                <div className={styles.datetimeRow}>
                    <input
                        type="date"
                        className={styles.input}
                        value={date}
                        disabled={scheduleLocked}
                        onChange={(e) => setDate(e.currentTarget.value)}
                    />
                    <input
                        type="time"
                        className={styles.input}
                        value={time}
                        disabled={scheduleLocked}
                        onChange={(e) => setTime(e.currentTarget.value)}
                    />
                </div>
            </section>

            <section className={styles.editorSection}>
                <h3 className={styles.sectionTitle}>Повтор</h3>
                <div className={styles.freqCards} role="radiogroup" aria-label="Частота повтора">
                    <button
                        type="button"
                        role="radio"
                        aria-checked={!repeat.enabled}
                        className={clsx(styles.freqCard, !repeat.enabled && styles.freqCardSelected)}
                        disabled={scheduleLocked}
                        onClick={() => setRepeat({ ...repeat, enabled: false })}
                    >
                        <span className={styles.freqCardTitle}>Нет</span>
                        <span className={styles.freqCardHint}>одноразовая задача</span>
                    </button>
                    {FREQ_CARDS.map((card) => {
                        const selected = repeat.enabled && repeat.freq === card.freq;
                        return (
                            <button
                                key={card.freq}
                                type="button"
                                role="radio"
                                aria-checked={selected}
                                className={clsx(styles.freqCard, selected && styles.freqCardSelected)}
                                disabled={scheduleLocked}
                                onClick={() => setRepeat({ ...repeat, enabled: true, freq: card.freq })}
                            >
                                <span className={styles.freqCardTitle}>{card.title}</span>
                                <span className={styles.freqCardHint}>{card.hint}</span>
                            </button>
                        );
                    })}
                </div>

                {repeat.enabled && !scheduleLocked && (
                    <div className={styles.repeatOptions}>
                        <label className={styles.optionRow}>
                            <span className={styles.optionLabel}>Интервал</span>
                            <input
                                type="number"
                                min={1}
                                className={styles.numberInput}
                                value={repeat.interval}
                                onChange={(e) =>
                                    setRepeat({
                                        ...repeat,
                                        interval: Math.max(1, Number(e.currentTarget.value) || 1),
                                    })
                                }
                            />
                        </label>

                        {repeat.freq === 'weekly' && (
                            <div className={styles.optionColumn}>
                                <span className={styles.optionLabel}>Дни недели</span>
                                <WeekdayChips
                                    value={repeat.weekdays}
                                    onChange={(weekdays) => setRepeat({ ...repeat, weekdays })}
                                />
                            </div>
                        )}

                        {repeat.freq === 'monthly' && (
                            <div className={styles.optionColumn}>
                                <span className={styles.optionLabel}>Числа месяца</span>
                                <MonthDayGrid
                                    value={repeat.monthDays}
                                    onChange={(monthDays) => setRepeat({ ...repeat, monthDays })}
                                />
                            </div>
                        )}

                        <div className={styles.optionColumn}>
                            <span className={styles.optionLabel}>Окончание</span>
                            <label className={styles.endOption}>
                                <input
                                    type="radio"
                                    name="repeat-end-c"
                                    checked={repeat.end === 'never'}
                                    onChange={() => setRepeat({ ...repeat, end: 'never' })}
                                />
                                Без окончания
                            </label>
                            <label className={styles.endOption}>
                                <input
                                    type="radio"
                                    name="repeat-end-c"
                                    checked={repeat.end === 'until'}
                                    onChange={() => setRepeat({ ...repeat, end: 'until' })}
                                />
                                До
                                <input
                                    type="date"
                                    className={styles.inputInline}
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
                                    name="repeat-end-c"
                                    checked={repeat.end === 'count'}
                                    onChange={() => setRepeat({ ...repeat, end: 'count' })}
                                />
                                После
                                <input
                                    type="number"
                                    min={1}
                                    className={styles.numberInput}
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

                        <p className={styles.preview}>{repeatSummary(repeat, date)}</p>
                    </div>
                )}
                {repeat.enabled && scheduleLocked && (
                    <p className={styles.preview}>
                        {formatRecurrence(task!.recurrence!, task!.dueDate)}
                    </p>
                )}
            </section>

            <div className={styles.editorActions}>
                {request.kind === 'edit' && (
                    <button type="button" className={styles.deleteButton} onClick={onDelete}>
                        Удалить
                    </button>
                )}
                <button type="button" className={styles.saveButton} onClick={save}>
                    {request.kind === 'new' ? 'Создать задачу' : 'Сохранить'}
                </button>
            </div>
        </div>
    );
}
