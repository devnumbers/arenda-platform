'use client';

import clsx from 'clsx';
import type { JSX, ReactNode } from 'react';
import styles from './Icon.module.css';

export type IconSize = 'xs' | 's' | 'm' | 'l';

export type IconProps = {
  readonly size?: IconSize;
  readonly className?: string;
  readonly children: ReactNode;
};

const sizeClassMap: Record<IconSize, string> = {
  xs: styles.xs ?? '',
  s: styles.s ?? '',
  m: styles.m ?? '',
  l: styles.l ?? '',
};

export function Icon({ size = 'm', className, children }: IconProps): JSX.Element {
  const sizeClass = sizeClassMap[size];

  return <span className={clsx(styles.icon, sizeClass, className)}>{children}</span>;
}
