'use client';

import {type JSX, useEffect, useState} from 'react';
import {type CloseButtonProps, cssTransition, ToastContainer,} from 'react-toastify/unstyled';
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
            <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none">
                <path
                    d="M15.2437 3.57611C15.5692 3.25068 16.0967 3.25067 16.4221 3.57611C16.7475 3.90155 16.7475 4.42908 16.4221 4.75449L11.1772 9.99863L16.4213 15.2428C16.7467 15.5682 16.7467 16.0957 16.4213 16.4212C16.0959 16.7466 15.5684 16.7465 15.2429 16.4212L9.99879 11.177L4.75628 16.4203C4.43087 16.7457 3.90334 16.7457 3.5779 16.4203C3.25246 16.0949 3.25246 15.5674 3.5779 15.242L8.82041 9.99863L3.57708 4.7553C3.25165 4.42987 3.25165 3.90235 3.57708 3.57692C3.90252 3.2515 4.43003 3.25149 4.75547 3.57692L9.99879 8.82025L15.2437 3.57611Z"
                    fill="currentColor"/>
            </svg>
        </button>
    );
}

const ToastTransition = cssTransition({
    enter: styles.toastEnter,
    exit: styles.toastExit,
    collapseDuration: 250,
});

export function ToastProvider(): JSX.Element {
    const isMobile = useIsMobile();

    return (
        <ToastContainer
            position={isMobile ? 'top-center' : 'top-right'}
            stacked
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
