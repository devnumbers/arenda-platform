'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { BoldHome } from '@/shared/assets/icons';
import { PickerField, type PickerOption } from '@/shared/ui/design';
import styles from '../../page.module.css';

/** Витринные объекты пикера (#505): с домашней иконкой в сером круге,
 * как строка «Объект» на карточке контакта. */
function ObjectAvatar(): JSX.Element {
    return (
        <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted text-content">
            <BoldHome className="h-6 w-6" />
        </span>
    );
}

const showcaseObjects: ReadonlyArray<PickerOption> = [
    { value: 'kv-1', label: 'Моя квартира', hint: 'Новаторов, 8', icon: <ObjectAvatar /> },
    { value: 'kv-2', label: 'Квартира на набережной', hint: 'Набережная, 15', icon: <ObjectAvatar /> },
    { value: 'kv-3', label: 'Дача', hint: 'Приозёрная, 2', icon: <ObjectAvatar /> },
];

export function PickerFieldSection(): JSX.Element {
    const [pickerValue, setPickerValue] = useState<string | null>('kv-1');
    const [pickerEmpty, setPickerEmpty] = useState<string | null>(null);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>PickerField · пикер с поиском (#505)</h3>
            <p className={styles.groupTitle}>
                Триггер — бокс поля с шевроном; список — в адаптивном Modal (шит на
                мобильном, карточка на десктопе), поиск по названию и подсказке,
                строка «Без объекта» очищает значение. Первый потребитель —
                «Привязанный объект» формы контакта.
            </p>
            <div className={styles.textFields}>
                <PickerField
                    title="Привязанный объект"
                    placeholder="Выберите объект"
                    options={showcaseObjects}
                    clearable
                    value={pickerValue}
                    onValueChange={setPickerValue}
                />
                <PickerField
                    title="Привязанный объект"
                    placeholder="Выберите объект"
                    options={showcaseObjects}
                    clearable
                    value={pickerEmpty}
                    onValueChange={setPickerEmpty}
                />
                <PickerField
                    title="Привязанный объект"
                    placeholder="Выберите объект"
                    options={showcaseObjects}
                    disabled
                />
            </div>
        </div>
    );
}
