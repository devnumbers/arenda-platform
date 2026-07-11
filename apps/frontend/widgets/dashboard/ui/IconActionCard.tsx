'use client';

import type {JSX, ReactNode} from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import styles from './IconActionCard.module.css';

export type IconActionCardProps = {
    readonly icon: ReactNode;
    readonly label: string;
    readonly href?: string;
    readonly variant?: 'filled' | 'outlined';
    readonly size?: 'small' | 'medium';
    readonly className?: string;
};

export function IconActionCard({
                                   icon,
                                   label,
                                   href,
                                   variant = 'filled',
                                   size = 'medium',
                                   className,
                               }: IconActionCardProps): JSX.Element {
    const content = (
        <>
      <span
          className={clsx(
              styles.iconBox,
              variant === 'filled' && styles.filled,
              variant === 'outlined' && styles.outlined,
              size === 'small' && styles.small,
              size === 'medium' && styles.medium,
          )}
      >
        {icon}
      </span>
            <span className={styles.label}>{label}</span>
        </>
    );

    const cardClassName = clsx(styles.root, className);

    if (href) {
        return (
            <NextLink href={href} className={cardClassName}>
                {content}
            </NextLink>
        );
    }

    return <span className={cardClassName}>{content}</span>;
}
