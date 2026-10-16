'use client';

import {type JSX, useEffect, useState} from 'react';
import {type CloseButtonProps, cssTransition, ToastContainer,} from 'react-toastify/unstyled';
// Нативный CSS библиотеки: контейнер, позиции, скрытие прогресс-бара.
// unstyled сам стилей не несёт.
import 'react-toastify/dist/ReactToastify.css';
import {Cancel} from '@/shared/assets/icons';
import styles from './ToastProvider.module.css';

const MOBILE_MEDIA_QUERY = '(max-width: 767px)';

function useIsMobile(): boolean {
    const [isMobile, setIsMobile] = useState(false);

    useEffect(() => {
        const mql = window.matchMedia(MOBILE_MEDIA_QUERY);
        const update = (matches: boolean) => setIsMobile(matches);

        update(mql.matches);

        const onChange = (event: MediaQueryListEvent) => update(event.matches);
        mql.addEventListener('change', onChange);
        return () => mql.removeEventListener('change', onChange);
    }, []);

    return isMobile;
}

export function CloseToastButton({
                                     closeToast,
                                     ariaLabel,
                                 }: CloseButtonProps): JSX.Element {
    return (
        <button
            type="button"
            className={styles.closeButton}
            aria-label={ariaLabel ?? 'Закрыть'}
            onClick={(event) => {
                event.stopPropagation();
                closeToast(true);
            }}
        >
            <Cancel width={20} height={20} aria-hidden />
        </button>
    );
}

const ToastTransition = cssTransition({
    enter: styles.toastEnter ?? '',
    exit: styles.toastExit ?? '',
    collapseDuration: 250,
});

/** Канон автоухода тостов — 5с у всех (решение владельца 28.09, аудит
 * #876, как в iOS): единый источник для контейнера и всех вызывающих;
 * hover и потеря фокуса ставят таймер на паузу. */
export const TOAST_AUTO_CLOSE_MS = 5000;

export function ToastProvider(): JSX.Element {
    const isMobile = useIsMobile();

    return (
        <ToastContainer
            position={isMobile ? 'top-center' : 'top-right'}
            /* Максимум один тост на экране (решение владельца 13.10): новый
               показ вытесняет видимый (showDisplacingToast), стек и нативная
               очередь лишних не используются — без limit их просто нет.
               limit={1} вместо этого не годится: v11 держит тост в счётчике
               показанных до конца exit+collapse, и новый тост ждал бы в
               очереди ~600мс — замена шла бы «ступенькой», не кросс-свапом. */
            hideProgressBar
            closeOnClick={false}
            pauseOnHover
            pauseOnFocusLoss
            autoClose={TOAST_AUTO_CLOSE_MS}
            draggable
            draggableDirection={isMobile ? 'y' : 'x'}
            draggablePercent={40}
            closeButton={CloseToastButton}
            icon={false}
            transition={ToastTransition}
            className={styles.container}
            toastClassName={styles.toast}
        />
    );
}
