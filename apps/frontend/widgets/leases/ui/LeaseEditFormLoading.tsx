import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import { DetailSection } from '@/shared/ui/detail-section';
import styles from './LeaseEditFormLoading.module.css';

export function LeaseEditFormLoading(): JSX.Element {
    return (
        <div
            className={styles.root}
            aria-busy="true"
            aria-label="Загрузка формы редактирования аренды"
        >
            <DetailSection>
                <Skeleton className={styles.sectionTitle}/>
                <div className={styles.fields}>
                    <Skeleton className={styles.field}/>
                    <Skeleton className={styles.field}/>
                    <div className={styles.dateRow}>
                        <Skeleton className={styles.field}/>
                        <Skeleton className={styles.field}/>
                    </div>
                    <Skeleton className={styles.field}/>
                    <Skeleton className={styles.field}/>
                    <Skeleton className={styles.multilineField}/>
                </div>
            </DetailSection>

            <div className={styles.actions}>
                <Skeleton className={styles.submit}/>
                <Skeleton className={styles.submit}/>
            </div>
        </div>
    );
}
