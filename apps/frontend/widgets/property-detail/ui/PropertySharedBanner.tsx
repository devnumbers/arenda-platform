'use client';

import type {JSX} from 'react';
import {AccessRoleBadge} from '@/entities/access/ui/AccessRoleBadge';
import type {PropertyAccess} from '@/entities/property/model/types';
import styles from './PropertySharedBanner.module.css';

export type PropertySharedBannerProps = {
    readonly access: PropertyAccess;
};

export function PropertySharedBanner({access}: PropertySharedBannerProps): JSX.Element | null {
    if (access.role === 'owner') {
        return null;
    }

    return (
        <div className={styles.root} role="alert">
            <p className={styles.text}>
                {access.ownerName
                    ? `С вами делится ${access.ownerName}`
                    : 'С вами поделились этим объектом'}
            </p>
            <AccessRoleBadge role={access.role}/>
        </div>
    );
}
