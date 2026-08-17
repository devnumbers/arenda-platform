import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import { DetailSection } from '@/shared/ui/detail-section';
import styles from './LeaseDetailLoading.module.css';

export function LeaseDetailLoading(): JSX.Element {
    return (
        <div className={styles.root} aria-busy="true" aria-label="Загрузка аренды">
            <DetailSection>
                <h2 className={styles.sectionTitle}>Арендатор</h2>
                <div className={styles.card}>
                    <Skeleton className={styles.longBar}/>
                </div>
            </DetailSection>

            <DetailSection>
                <h2 className={styles.sectionTitle}>Условия аренды</h2>
                <div className={styles.card}>
                    <div className={styles.row}>
                        <Skeleton className={styles.shortBar}/>
                        <Skeleton className={styles.shortBar}/>
                    </div>
                    <div className={styles.row}>
                        <Skeleton className={styles.shortBar}/>
                        <Skeleton className={styles.shortBar}/>
                    </div>
                    <div className={styles.row}>
                        <Skeleton className={styles.shortBar}/>
                        <Skeleton className={styles.shortBar}/>
                    </div>
                    <div className={styles.row}>
                        <Skeleton className={styles.shortBar}/>
                        <Skeleton className={styles.shortBar}/>
                    </div>
                </div>
            </DetailSection>

            <DetailSection>
                <h2 className={styles.sectionTitle}>Арендная плата</h2>
                <div className={styles.card}>
                    <Skeleton className={styles.longBar}/>
                </div>
            </DetailSection>
        </div>
    );
}
