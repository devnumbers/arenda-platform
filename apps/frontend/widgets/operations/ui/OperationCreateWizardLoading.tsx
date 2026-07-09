import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import styles from './OperationCreateWizardLoading.module.css';

export function OperationCreateWizardLoading(): JSX.Element {
    return (
        <div
            className={styles.root}
            aria-busy="true"
            aria-label="Загрузка формы создания операции"
        >
            <div className={styles.fields}>
                <Skeleton className={styles.field} />
                <Skeleton className={styles.field} />
                <Skeleton className={styles.field} />
                <Skeleton className={styles.field} />
                <Skeleton className={styles.field} />
                <Skeleton className={styles.multilineField} />
            </div>
            <Skeleton className={styles.submit} />
        </div>
    );
}
