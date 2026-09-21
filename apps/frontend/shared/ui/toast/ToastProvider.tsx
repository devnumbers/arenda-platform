'use client';

import {type JSX, useEffect, useState} from 'react';
import {type CloseButtonProps, cssTransition, ToastContainer,} from 'react-toastify/unstyled';
// Нативный CSS библиотеки: контейнер, позиции, стек (peek-ступени,
// data-collapsed, transition transform 0.3s). unstyled сам стилей не несёт.
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

export function ToastProvider(): JSX.Element {
    const isMobile = useIsMobile();

    return (
        <ToastContainer
            position={isMobile ? 'top-center' : 'top-right'}
            stacked
            /* Больше трёх карточек в стеке не показываются: лишние стоят в
               нативной очереди и выходят по мере закрытия (решение владельца
               21.09). */
            limit={3}
            hideProgressBar
            closeOnClick={false}
            pauseOnHover
            pauseOnFocusLoss
            autoClose={false}
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
