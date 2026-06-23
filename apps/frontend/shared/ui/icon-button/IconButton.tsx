'use client';

import type { ButtonHTMLAttributes, JSX, ReactNode } from 'react';
import clsx from 'clsx';
import { Loading } from '@/shared/assets/icons';
import { Icon, type IconSize } from '@/shared/ui/icon';
import type { ButtonSize, ButtonVariant } from '@/shared/ui/button/Button';
import styles from './IconButton.module.css';

export type IconButtonVariant = ButtonVariant | 'white-icon' | 'primary-icon';

export type IconButtonProps = {
  variant?: IconButtonVariant;
  size?: ButtonSize;
  rounded?: boolean;
  loading?: boolean;
  icon: ReactNode;
  'aria-label': string;
  className?: string;
} & ButtonHTMLAttributes<HTMLButtonElement>;

const iconSizeMap: Record<ButtonSize, IconSize> = {
  large: 'l',
  medium: 'm',
  small: 's',
  tiny: 'xs',
};

export function IconButton({
  variant = 'primary',
  size = 'medium',
  rounded = false,
  loading = false,
  icon,
  'aria-label': ariaLabel,
  className,
  disabled,
  type = 'button',
  ...rest
}: IconButtonProps): JSX.Element {
  const isDisabled = disabled || loading;

  return (
    <button
      type={type}
      disabled={isDisabled}
      aria-busy={loading || undefined}
      aria-label={ariaLabel}
      className={clsx(
        styles.button,
        styles[variant],
        styles[size],
        rounded && styles.rounded,
        loading && styles.loading,
        disabled && styles.disabled,
        className
      )}
      {...rest}
    >
      {loading ? (
        <Icon size={iconSizeMap[size]} className={styles.spinner}>
          <Loading />
        </Icon>
      ) : (
        <Icon size={iconSizeMap[size]}>{icon}</Icon>
      )}
    </button>
  );
}
