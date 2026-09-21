'use client';

import { useEffect, useRef, useState } from 'react';

/** Подсказка «Скопировано» держится 2 с — канон копирования
 * (карточка контакта, 1285:55112). */
const COPIED_RESET_MS = 2000;

/**
 * Канон копирования «Скопировано» (карточка контакта): `copy(text)` пишет в
 * буфер и включает подсказку на 2 с; `copied` — подсказка горит. Буфер может
 * быть недоступен (небезопасный контекст, http) — подсказка показывается
 * независимо от него. Повторное копирование перезапускает таймер (он всегда
 * один), уход с экрана его гасит. Разметка подсказки — у поверхности
 * (иконка IconButton, trailingIcon кнопки, span-хинт); сведение трёх ручных
 * копий паттерна (контакты, шеринг объекта, SupportModal) — pre-merge
 * карты #761, находка арх-прохода.
 */
export function useCopiedHint(): {
    readonly copied: boolean;
    readonly copy: (text: string) => void;
    /** Погасить подсказку сразу (например, при закрытии поверхности). */
    readonly reset: () => void;
} {
    const [copied, setCopied] = useState(false);
    const copiedTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

    // Таймер подсказки гасим при размонтировании.
    useEffect(
        () => () => {
            if (copiedTimeoutRef.current !== null) {
                clearTimeout(copiedTimeoutRef.current);
            }
        },
        [],
    );

    const copy = (text: string): void => {
        // Расширение типа сохраняет runtime-проверку: DOM-тип считает
        // clipboard всегда доступным, навигатор — нет.
        const clipboard = navigator.clipboard as Clipboard | undefined;
        clipboard?.writeText(text).catch(() => {
            // подсказку «Скопировано» показываем независимо от буфера
        });
        setCopied(true);
        if (copiedTimeoutRef.current !== null) {
            clearTimeout(copiedTimeoutRef.current);
        }
        copiedTimeoutRef.current = setTimeout(() => setCopied(false), COPIED_RESET_MS);
    };

    const reset = (): void => {
        if (copiedTimeoutRef.current !== null) {
            clearTimeout(copiedTimeoutRef.current);
        }
        setCopied(false);
    };

    return { copied, copy, reset };
}
