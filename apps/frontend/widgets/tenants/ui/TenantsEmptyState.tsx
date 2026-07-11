'use client';

import type {JSX} from 'react';
import {EmptyState} from '@/shared/ui/empty-state';
import {Arendators} from '@/shared/assets/icons';
import {ROUTES} from '@/shared/config/routes';

export function TenantsEmptyState(): JSX.Element {
    return (
        <EmptyState
            icon={
                <Arendators/>
            }
            entities="арендаторов"
            subtitle="Добавьте первого арендатора"
            actionHref={ROUTES.tenantNew}
            actionText="Добавить арендатора"
        />
    );
}
