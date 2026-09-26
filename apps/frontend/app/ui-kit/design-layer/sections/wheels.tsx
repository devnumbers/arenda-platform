'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Button, WheelPicker, WheelPickerSheet, paddedItems } from '@/shared/ui/design';
import styles from '../../page.module.css';

/** Колёса демо-шита WheelPickerSheet: часы 00–23 и минуты 00–59. Состояние
 * демо держит HH:MM-строки, значения совпадают с метками (паддинг —
 * контракт shared paddedItems), иначе колесо не находит выбранное и садится
 * на первый ряд (находка приёмки #810). */
const sheetHourItems = paddedItems(24);
const sheetMinuteItems = paddedItems(60);

export function WheelPickerSheetSection(): JSX.Element {
    const [wheelSheetOpen, setWheelSheetOpen] = useState(false);
    const [wheelSheetValue, setWheelSheetValue] = useState('09:00');
    const [sheetHour, setSheetHour] = useState('09');
    const [sheetMinute, setSheetMinute] = useState('00');

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>WheelPickerSheet · универсальный шит колёс</h3>
            <p className={styles.groupTitle}>
                Любое число колёс + передаваемые кнопки (Figma 1539-82659); колёса крутятся без
                конца — кольцо в обе стороны (#810), на мобиле — выезжающий шит (vaul), на
                десктопе — карточка.
            </p>
            <div className={styles.column}>
                <Button onClick={() => setWheelSheetOpen(true)}>Открыть шит колёс</Button>
                <p className="px-6 text-base text-content-secondary">
                    Выбрано: {wheelSheetValue}
                </p>
            </div>
            <WheelPickerSheet
                title="Время"
                open={wheelSheetOpen}
                onOpenChange={setWheelSheetOpen}
                actions={[
                    {
                        label: 'Отменить',
                        variant: 'secondary',
                        onSelect: () => setWheelSheetOpen(false),
                    },
                    {
                        label: 'Выбрать',
                        onSelect: () => {
                            setWheelSheetValue(`${sheetHour}:${sheetMinute}`);
                            setWheelSheetOpen(false);
                        },
                    },
                ]}
                columns={[
                    <WheelPicker
                        key="hour"
                        label="Часы"
                        strip={false}
                        loop
                        items={sheetHourItems}
                        value={sheetHour}
                        onValueChange={setSheetHour}
                    />,
                    <WheelPicker
                        key="minute"
                        label="Минуты"
                        strip={false}
                        loop
                        items={sheetMinuteItems}
                        value={sheetMinute}
                        onValueChange={setSheetMinute}
                    />,
                ]}
            />
        </div>
    );
}
