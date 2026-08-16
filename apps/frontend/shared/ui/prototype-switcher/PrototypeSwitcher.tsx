'use client';

// ПРОТОТИП (throwaway): плавающий переключатель вариантов для UI-прототипов.
// Скрыт в production-сборке, чтобы случайный мёрж не утащил панель к пользователям.

import { useCallback, useEffect, type JSX } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import styles from './PrototypeSwitcher.module.css';

export type PrototypeVariant = {
    readonly key: string;
    readonly label: string;
};

export type PrototypeSwitcherProps = {
    readonly variants: readonly PrototypeVariant[];
    readonly current: string;
    readonly paramName?: string;
};

function isEditableTarget(target: EventTarget | null): boolean {
    if (!(target instanceof HTMLElement)) return false;
    return (
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable
    );
}

export function PrototypeSwitcher({
    variants,
    current,
    paramName = 'variant',
}: PrototypeSwitcherProps): JSX.Element | null {
    const router = useRouter();
    const pathname = usePathname();
    const searchParams = useSearchParams();

    const selectVariant = useCallback(
        (key: string): void => {
            const params = new URLSearchParams(searchParams.toString());
            params.set(paramName, key);
            router.replace(`${pathname}?${params.toString()}`, { scroll: false });
        },
        [paramName, pathname, router, searchParams],
    );

    const cycle = useCallback(
        (delta: number): void => {
            const index = variants.findIndex((v) => v.key === current);
            const next = variants[(index + delta + variants.length) % variants.length];
            if (next) selectVariant(next.key);
        },
        [current, selectVariant, variants],
    );

    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent): void => {
            if (isEditableTarget(event.target)) return;
            if (event.key === 'ArrowLeft') cycle(-1);
            if (event.key === 'ArrowRight') cycle(1);
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [cycle]);

    if (process.env.NODE_ENV === 'production') return null;

    const currentVariant = variants.find((v) => v.key === current);

    return (
        <div className={styles.bar} role="group" aria-label="Переключатель вариантов прототипа">
            <button
                type="button"
                className={styles.arrow}
                aria-label="Предыдущий вариант"
                onClick={() => cycle(-1)}
            >
                ←
            </button>
            <span className={styles.label}>
                {currentVariant ? `${currentVariant.key} — ${currentVariant.label}` : current}
            </span>
            <button
                type="button"
                className={styles.arrow}
                aria-label="Следующий вариант"
                onClick={() => cycle(1)}
            >
                →
            </button>
        </div>
    );
}
