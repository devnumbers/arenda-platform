'use client';

import type {AnchorHTMLAttributes, JSX, ReactNode} from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import {Sync} from '@/shared/assets/icons';
import {Icon, type IconSize} from '@/shared/ui/icon';
import styles from './IconLink.module.css';

/** Типы локальные (легаси button/icon-button снесены, #901): вариант и
 * размер ссылаются на классы собственного CSS-модуля, набор повторяет
 * снесённый IconButton (primary/secondary/clear/icon-black × 4 размера). */
export type IconLinkVariant = 'primary' | 'secondary' | 'clear' | 'icon-black';
export type IconLinkSize = 'large' | 'medium' | 'small' | 'tiny';

export type IconLinkProps = {
    href: string;
    variant?: IconLinkVariant;
    size?: IconLinkSize;
    rounded?: boolean;
    loading?: boolean;
    icon: ReactNode;
    'aria-label'?: string;
} & AnchorHTMLAttributes<HTMLAnchorElement>;

const iconSizeMap: Record<IconLinkSize, IconSize> = {
    large: 'l',
    medium: 'm',
    small: 's',
    tiny: 'xs',
};

export function IconLink({
                             href,
                             variant = 'primary',
                             size = 'medium',
                             rounded = false,
                             loading = false,
                             icon,
                             'aria-label': ariaLabel,
                             className,
                             ...rest
                         }: IconLinkProps): JSX.Element {
    return (
        <NextLink
            href={href}
            aria-label={ariaLabel}
            className={clsx(
                styles.button,
                styles[variant],
                styles[size],
                rounded && styles.rounded,
                loading && styles.loading,
                className
            )}
            {...rest}
        >
            {loading ? (
                <Icon size={iconSizeMap[size]} className={styles.spinner}>
                    <Sync className="text-error"/>
                </Icon>
            ) : (
                <Icon size={iconSizeMap[size]}>{icon}</Icon>
            )}
        </NextLink>
    );
}
