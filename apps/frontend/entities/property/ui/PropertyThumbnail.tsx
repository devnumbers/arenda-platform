'use client';

import type {JSX} from 'react';
import clsx from 'clsx';
import {Icon} from '@/shared/ui/icon';
import {Home} from '@/shared/assets/icons';
import styles from './PropertyThumbnail.module.css';

export type PropertyThumbnailSize = 'small' | 'medium' | 'large';

export type PropertyThumbnailProps = {
    readonly size?: PropertyThumbnailSize;
    readonly className?: string;
};

export function PropertyThumbnail({size = 'small', className}: PropertyThumbnailProps): JSX.Element {
    return (
        <div className={clsx(styles.root, styles[size], className)}>
            <Icon size="l">
                <Home/>
            </Icon>
        </div>
    );
}
