'use client';

import type { ComponentProps, JSX } from 'react';
import { cn } from '@/shared/lib/cn';

/** Скелетон дизайн-слоя — канон блоков загрузки (унификация 2026-09-04;
 * до этого — inline animate-pulse в каждом виджете). Базовый вид: пульс на
 * `bg-surface-muted`, размер и форму задаёт потребитель через className
 * (строка: `h-11`, заголовок: `h-6 w-40`, круглый аватар: `h-12 w-12
 * rounded-full`). Внутри серой карточки (фон `bg-surface-muted`) блок
 * приглушается до `bg-surface-muted-hover`, чтобы оставался видимым.
 * aria-hidden не нужен: сам элемент декоративен, экран обязан давать
 * текстовый статус загрузки рядом (role="status" и т.п.). */
export type SkeletonProps = ComponentProps<'div'>;

export function Skeleton({ className, ...props }: SkeletonProps): JSX.Element {
  return <div aria-hidden className={cn('animate-pulse rounded-pill bg-surface-muted', className)} {...props} />;
}
