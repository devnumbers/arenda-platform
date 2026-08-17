'use client';

import type {JSX} from 'react';
import {ROUTES} from '@/shared/config/routes';
import {LinkButton} from '@/shared/ui/link-button';
import { DetailSection } from '@/shared/ui/detail-section';
import styles from './PropertyOperationsActions.module.css';

export type PropertyOperationsActionsProps = {
    readonly propertyId: string;
    readonly isArchived: boolean;
};

export function PropertyOperationsActions({
                                              propertyId,
                                              isArchived,
                                          }: PropertyOperationsActionsProps): JSX.Element {
    return (
        <DetailSection>
            <div className={styles.actions}>
                <LinkButton
                    href={`${ROUTES.financeCreateOperation}?propertyId=${propertyId}`}
                    variant="primary"
                    fullWidth
                    disabled={isArchived}
                    title={isArchived ? 'Объект в архиве' : undefined}
                >
                    Добавить операцию
                </LinkButton>
                <LinkButton
                    href={`${ROUTES.financeOperations}?property_id=${propertyId}`}
                    variant="secondary"
                    fullWidth
                >
                    Все операции
                </LinkButton>
            </div>
        </DetailSection>
    );
}
