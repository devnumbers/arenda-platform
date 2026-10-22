'use client';

import type { JSX, ReactNode } from 'react';
import Image from 'next/image';
import { cn } from '@/shared/lib/cn';

/** Пустое состояние дизайн-слоя — канон полноэкранных и внутристраничных
 * «пусто» (унификация 2026-09-04; анатомия по макету хаба «Объектов»
 * 3235-74057): WebP-иллюстрация 128 по центру, заголовок H2 24/28 и серое
 * описание 16/18 (максимум 360), опциональное действие под описанием.
 * Отступы — верх pt-14 и бока px-8, иллюстрация→текст 24 (mt-6),
 * заголовок→описание 8 (gap-2), текст→действие 32 (mt-8); CTA в слоте —
 * `Button size="small" radius="m"` (44/12), как у секционных пустых (#588).
 * Вертикальные отступы переопределяются className длинными руками
 * (например pt-16 pb-16 у пустого периода операций). `imageRounded` —
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
    <div className={cn('flex flex-col items-center px-8 pt-14 text-center', className)}>
      {/* sizes: слот 128 CSS — на 3x-экранах srcset отдаёт 384px-вариант
       * вместо апскейла 256 (#1002); потолок задаёт источник: меньший файл
       * Next не растягивает, 384-варианта у него нет. priority на всех
       * пустых состояниях (решение владельца 22.10.2026): EmptyState
       * монтируется уже после данных (скелетон до него), и вторая ленивость
       * (дефолт loading="lazy" ждёт пересечения вьюпорта) откладывала запрос
       * иллюстрации ещё на кадр — на пустом экране она всегда above the
       * fold, lazy там не нужен ни полноэкранному, ни секционному слоту. */}
      <Image
        src={imageSrc}
        alt={imageAlt}
        width={128}
        height={128}
        sizes="128px"
        quality={90}
        priority
        className={cn('h-32 w-32', imageRounded && 'rounded-pill object-cover')}
      />
      {(title !== undefined || description !== undefined) && (
        <div className="mt-6 flex flex-col items-center gap-2">
          {title !== undefined && (
            <h2 className="text-[24px] font-semibold leading-7 text-content">{title}</h2>
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
      )}
      {/* Гард ловит и false от частого «cond && <Button/>»: иначе пустой
       * div ловит mt-8 корня — фантомные 32px, когда действия нет. */}
      {action !== undefined && action !== false && <div className="mt-8">{action}</div>}
    </div>
  );
}
