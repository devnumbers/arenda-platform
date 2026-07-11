'use client';

import type {JSX} from 'react';
import {useRouter} from 'next/navigation';
import {ArrowLeft} from '@/shared/assets/icons';
import {goBack} from '@/shared/lib/navigation';
import {IconButton} from '@/shared/ui/icon-button';

export type BackButtonProps = {
    fallbackHref: string;
    ariaLabel?: string;
};

export function BackButton({fallbackHref, ariaLabel = 'Назад'}: BackButtonProps): JSX.Element {
    const router = useRouter();

    return (
        <IconButton
            size={'large'}
            variant="secondary"
            aria-label={ariaLabel}
            icon={<ArrowLeft/>}
            onClick={() => goBack(router, fallbackHref)}
        />
    );
}
