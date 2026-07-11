import type {JSX, ReactNode} from 'react';
import clsx from 'clsx';
import {BackButton} from '@/shared/ui/back-button';
import styles from './PageHeader.module.css';

export interface PageHeaderProps {
    backHref?: string;
    title: ReactNode;
    actions?: ReactNode;
    className?: string;
}

export function PageHeader({backHref, title, actions, className}: PageHeaderProps): JSX.Element {
    return (
        <header className={clsx(styles.header, className)}>
            <div className={styles.left}>
                {backHref && (
                    <BackButton fallbackHref={backHref}/>
                )}
                <h1 className={styles.title}>{title}</h1>
            </div>
            {actions && <div className={styles.actions}>{actions}</div>}
        </header>
    );
}
