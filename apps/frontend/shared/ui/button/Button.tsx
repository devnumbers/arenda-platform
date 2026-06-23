'use client';

import type { ButtonHTMLAttributes, JSX, ReactNode } from 'react';
import clsx from 'clsx';
import { Loading } from '@/shared/assets/icons';
import { Icon } from '@/shared/ui/icon';
import styles from './Button.module.css';

export type ButtonVariant = 'primary' | 'secondary' | 'clear' | 'icon-black';
export type ButtonSize = 'large' | 'medium' | 'small' | 'tiny';

export type ButtonProps = {
  variant?: ButtonVariant;
  size?: ButtonSize;
  rounded?: boolean;
  fullWidth?: boolean;
  loading?: boolean;
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
  subtitle?: ReactNode;
  className?: string;
  children: ReactNode;
} & ButtonHTMLAttributes<HTMLButtonElement>;

const spinnerSizeMap: Record<ButtonSize, 'l' | 'm' | 's' | 'xs'> = {
  large: 'l',
  medium: 'm',
  small: 's',
  tiny: 'xs',
};

export function Button({
  variant = 'primary',
  size = 'medium',
  rounded = false,
  fullWidth = false,
  loading = false,
  leftIcon,
  rightIcon,
  subtitle,
  className,
  children,
  disabled,
  type = 'button',
  ...rest
}: ButtonProps): JSX.Element {
  const isDisabled = disabled || loading;

  return (
    <button
      type={type}
      disabled={isDisabled}
      aria-busy={loading || undefined}
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
    </button>
  );
}
