import type { JSX, ReactNode } from 'react';
import clsx from 'clsx';
import { IconLink } from '@/shared/ui/icon-link';
import { ArrowLeft } from '@/shared/assets/icons';
import styles from './PageHeader.module.css';

export interface PageHeaderProps {
  backHref?: string;
  title: ReactNode;
  actions?: ReactNode;
  className?: string;
}

export function PageHeader({ backHref, title, actions, className }: PageHeaderProps): JSX.Element {
  return (
    <header className={clsx(styles.header, className)}>
      <div className={styles.left}>
        {backHref && (
          <IconLink
            href={backHref}
            aria-label="Назад"
            icon={<ArrowLeft />}
            variant="icon-black"
          />
        )}
        <h1 className={styles.title}>{title}</h1>
      </div>
      {actions && <div className={styles.actions}>{actions}</div>}
    </header>
  );
}
