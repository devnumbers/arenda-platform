import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import {PropertyDetailSection} from '@/widgets/property-detail';
import styles from './LeaseEditFormLoading.module.css';

export function LeaseEditFormLoading(): JSX.Element {
    return (
        <div
            className={styles.root}
            aria-busy="true"
            aria-label="Загрузка формы редактирования аренды"
        >
            <PropertyDetailSection>
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
            </PropertyDetailSection>

            <div className={styles.actions}>
                <Skeleton className={styles.submit}/>
                <Skeleton className={styles.submit}/>
            </div>
        </div>
    );
}
