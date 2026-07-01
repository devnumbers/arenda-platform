'use client';

import type {JSX, ReactNode} from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import styles from './EntityCard.module.css';

export type EntityCardProps = {
    readonly imageUrl?: string | null;
    readonly placeholderIcon?: ReactNode;
    readonly title: string;
    readonly subtitle?: string;
    readonly href?: string;
    readonly size?: 'small' | 'medium';
    readonly className?: string;
};

export function EntityCard({
  imageUrl,
  placeholderIcon,
  title,
  subtitle,
  href,
  size = 'medium',
  className,
}: EntityCardProps): JSX.Element {
  const content = (
    <>
      <div
        className={clsx(
          styles.image,
          size === 'medium' && styles.medium,
          size === 'small' && styles.small,
          !imageUrl && styles.placeholder,
        )}
        style={imageUrl ? { backgroundImage: `url(${imageUrl})` } : undefined}
      >
        {!imageUrl && placeholderIcon}
      </div>
      <div className={styles.text}>
        <span className={styles.title}>{title}</span>
        {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
      </div>
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

    return <div className={cardClassName}>{content}</div>;
}
