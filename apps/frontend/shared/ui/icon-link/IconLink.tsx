'use client';

import type {AnchorHTMLAttributes, ButtonHTMLAttributes, JSX} from 'react';
import NextLink from 'next/link';
import clsx from 'clsx';
import {Sync} from '@/shared/assets/icons';
import {Icon, type IconSize} from '@/shared/ui/icon';
import type {ButtonSize} from '@/shared/ui/button/Button';
import type {IconButtonProps} from '@/shared/ui/icon-button/IconButton';
import styles from './IconLink.module.css';

export type IconLinkProps = {
    href: string;
} & Omit<IconButtonProps, keyof ButtonHTMLAttributes<HTMLButtonElement>> & AnchorHTMLAttributes<HTMLAnchorElement>;

const iconSizeMap: Record<ButtonSize, IconSize> = {
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
