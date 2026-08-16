// ПРОТОТИП (throwaway): диалог выбора рамок правки/удаления вхождения серии.

'use client';

import type { JSX } from 'react';
import type { EditScope } from '../model/types';
import styles from './ScopeDialog.module.css';

const OPTIONS: readonly { readonly scope: EditScope; readonly title: string; readonly hint: string }[] = [
    {
        scope: 'this',
        title: 'Только это вхождение',
        hint: 'Серия разорвётся: это вхождение станет отдельной задачей, остальные не изменятся',
    },
    {
        scope: 'this_and_future',
        title: 'Это и следующие',
        hint: 'Серия обрежется до этой даты и продолжится уже с новыми параметрами',
    },
    {
        scope: 'all',
        title: 'Вся серия',
        hint: 'Только название и описание; расписание останется прежним',
    },
];

export type ScopeDialogProps = {
    readonly actionLabel: string; // «Изменить» / «Удалить»
    readonly onChoose: (scope: EditScope) => void;
    readonly onClose: () => void;
};

export function ScopeDialog({ actionLabel, onChoose, onClose }: ScopeDialogProps): JSX.Element {
    return (
        <div className={styles.overlay} onClick={onClose} role="presentation">
            <div
                className={styles.dialog}
                role="dialog"
                aria-modal="true"
                aria-label={`${actionLabel}: рамки изменения`}
                onClick={(e) => e.stopPropagation()}
            >
                <h3 className={styles.title}>{actionLabel}</h3>
                <p className={styles.subtitle}>Это повторяющаяся задача. Что применить?</p>
                <div className={styles.options}>
                    {OPTIONS.map((option) => (
                        <button
                            key={option.scope}
                            type="button"
                            className={styles.option}
                            onClick={() => onChoose(option.scope)}
                        >
                            <span className={styles.optionTitle}>{option.title}</span>
                            <span className={styles.optionHint}>{option.hint}</span>
                        </button>
                    ))}
                </div>
                <button type="button" className={styles.cancel} onClick={onClose}>
                    Отмена
                </button>
            </div>
        </div>
    );
}
