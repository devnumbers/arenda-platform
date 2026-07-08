import type {JSX} from 'react';
import {Skeleton} from '@heroui/react/skeleton';
import {PropertyDetailSection} from '@/widgets/property-detail';
import styles from './LeaseDetailLoading.module.css';

export function LeaseDetailLoading(): JSX.Element {
    return (
        <div className={styles.root} aria-busy="true" aria-label="Загрузка аренды">
            <PropertyDetailSection>
                <h2 className={styles.sectionTitle}>Арендатор</h2>
                <div className={styles.card}>
                    <Skeleton className={styles.longBar}/>
                </div>
            </PropertyDetailSection>

            <PropertyDetailSection>
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
            </PropertyDetailSection>

            <PropertyDetailSection>
                <h2 className={styles.sectionTitle}>Арендная плата</h2>
                <div className={styles.card}>
                    <Skeleton className={styles.longBar}/>
                </div>
            </PropertyDetailSection>
        </div>
    );
}
