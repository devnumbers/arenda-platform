'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Skeleton } from './skeleton';

/**
 * Скелетон блока фото/баннера (#604): плейсхолдер медиа-блока — фото объекта,
 * баннер, иллюстрационная зона. По умолчанию — во всю ширину колонки,
 * высота 160, скругление карточки; размер и форму (круглый аватар крупного
 * слота, лента) задаёт потребитель через className — скелетон обязан
 * повторять геометрию реального блока (правило паритета, §7 DESIGN.md).
 * Имена и API окончательно решаются на первом реальном экране карты #603
 * (тикет #605).
 */
export type SkeletonMediaProps = {
  readonly className?: string;
};

export function SkeletonMedia({ className }: SkeletonMediaProps): JSX.Element {
  return <Skeleton className={cn('h-40 w-full rounded-card', className)} />;
}
