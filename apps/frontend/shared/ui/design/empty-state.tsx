'use client';

import type { JSX, ReactNode } from 'react';
import Image from 'next/image';
import { cn } from '@/shared/lib/cn';

/** Пустое состояние дизайн-слоя — канон полноэкранных и внутристраничных
 * «пусто» (унификация 2026-09-04; до этого — локальные копии в задачах,
 * контактах, платежах и легаси EmptyState): WebP-иллюстрация 128 по центру,
 * заголовок H2 20/24 и серое описание 16/18 (максимум 360), опциональное
 * действие под описанием. Вертикальный отступ верха — pt-16, переопределяется
 * className (например py-16 у пустого периода операций). `imageRounded` —
 * круглая иллюстрация (rounded-pill + object-cover), когда исходник круга.
 * Обязательное поле одно — imageSrc: бывают пустоты без заголовка. */
export type EmptyStateProps = {
  readonly imageSrc: string;
  readonly imageAlt?: string;
  /** Круглая иллюстрация: скругление + object-cover (макет задач). */
  readonly imageRounded?: boolean;
  readonly title?: string;
  readonly description?: ReactNode;
  readonly descriptionClassName?: string;
  /** Действие под описанием — готовый элемент (кнопка/ссылка). */
  readonly action?: ReactNode;
  readonly className?: string;
};

export function EmptyState({
  imageSrc,
  imageAlt = '',
  imageRounded = false,
  title,
  description,
  descriptionClassName,
  action,
  className,
}: EmptyStateProps): JSX.Element {
  return (
    <div className={cn('flex flex-col items-center gap-4 px-6 pt-16 text-center', className)}>
      {/* sizes: слот 128 CSS — на 3x-экранах srcset отдаёт 384px-вариант
       * вместо апскейла 256 (#1002); потолок задаёт источник: меньший файл
       * Next не растягивает, 384-варианта у него нет. */}
      <Image
        src={imageSrc}
        alt={imageAlt}
        width={128}
        height={128}
        sizes="128px"
        quality={90}
        className={cn('h-32 w-32', imageRounded && 'rounded-pill object-cover')}
      />
      <div className="flex flex-col items-center gap-3">
        {title !== undefined && (
          <h2 className="text-xl font-semibold leading-6 text-content">{title}</h2>
        )}
        {description !== undefined && (
          <p
            className={cn(
              'max-w-[360px] text-base leading-[18px] text-content-secondary',
              descriptionClassName,
            )}
          >
            {description}
          </p>
        )}
      </div>
      {/* Гард ловит и false от частого «cond && <Button/>»: иначе пустой
       * div ловит gap-4 корня — фантомные 16px, когда действия нет. */}
      {action !== undefined && action !== false && <div>{action}</div>}
    </div>
  );
}
