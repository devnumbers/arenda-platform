'use client';

import type { AnchorHTMLAttributes, ButtonHTMLAttributes, JSX } from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import { Loading } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import type { ButtonProps, ButtonSize } from '@/shared/ui/button/Button';
import styles from './LinkButton.module.css';

export type LinkButtonProps = {
  href: string;
  disabled?: boolean;
} & Omit<ButtonProps, keyof ButtonHTMLAttributes<HTMLButtonElement>> & AnchorHTMLAttributes<HTMLAnchorElement>;

const spinnerSizeMap: Record<ButtonSize, 'l' | 'm' | 's' | 'xs'> = {
  large: 'l',
  medium: 'm',
  small: 's',
  tiny: 'xs',
};

export function LinkButton({
  href,
  variant = 'primary',
  size = 'medium',
  rounded = false,
  fullWidth = false,
  loading = false,
  disabled = false,
  leftIcon,
  rightIcon,
  subtitle,
  className,
  children,
  onClick,
  ...rest
}: LinkButtonProps): JSX.Element {
  return (
    <NextLink
      href={href}
      aria-disabled={disabled || undefined}
      className={clsx(
        styles.button,
        styles[variant],
        styles[size],
        rounded && styles.rounded,
        fullWidth && styles.fullWidth,
        loading && styles.loading,
        disabled && styles.disabled,
        className
      )}
      {...rest}
      onClick={(event) => {
        if (disabled) {
          event.preventDefault();
          return;
        }
        onClick?.(event);
      }}
    >
      {loading ? (
        <Icon size={spinnerSizeMap[size]} className={styles.spinner}>
          <Loading />
        </Icon>
      ) : (
        <>
          {leftIcon && <span className={styles.leftIcon}>{leftIcon}</span>}
          <span className={styles.content}>
            <span className={styles.label}>{children}</span>
            {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
          </span>
          {rightIcon && <span className={styles.rightIcon}>{rightIcon}</span>}
        </>
      )}
    </NextLink>
  );
}
