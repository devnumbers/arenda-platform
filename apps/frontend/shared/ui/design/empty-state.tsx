'use client';

import type { JSX, ReactNode } from 'react';
import Image from 'next/image';
import { cn } from '@/shared/lib/cn';
import { onIllustrationLoad, rememberIllustrationRef } from '@/shared/ui/design/illustration-decode-registry';

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
 * Обязательное поле одно — imageSrc: бывают пустоты без заголовка.
 * `titleClassName` — перекрытие класса заголовка через cn/tailwind-merge
 * (прецедент `descriptionClassName`): service-титул 16/18 regular у пустых
 * состояний «Уведомлений» (макеты 3178-86086/3187-88176, решение владельца
 * 27.10.2026) — канон 24/28 остаётся дефолтом до макетов на экран. */
export type EmptyStateProps = {
  readonly imageSrc: string;
  readonly imageAlt?: string;
  /** Круглая иллюстрация: скругление + object-cover (макет задач). */
  readonly imageRounded?: boolean;
  readonly title?: string;
  /** Перекрытие класса заголовка (tailwind-merge, победа последнего). */
  readonly titleClassName?: string;
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
  titleClassName,
  description,
  descriptionClassName,
  action,
  className,
}: EmptyStateProps): JSX.Element {
  return (
    <div className={cn('flex flex-col items-center px-8 pt-14 text-center', className)}>
      {/* sizes = 2x слота — суперсэмплинг (решение владельца 22.10.2026
       * «всегда в хорошем качестве»): точный слот давал вариант 1:1 —
       * детальный 3D-арт в мелком растре мыл, чёткость появлялась только
       * после клиентской навигации (вариант на ступень больше), перезагрузка
       * возвращала мыло. priority на всех пустых состояниях (решение
       * владельца 22.10.2026): EmptyState монтируется уже после данных
       * (скелетон до него), и вторая ленивость (дефолт loading="lazy" ждёт
       * пересечения вьюпорта) откладывала запрос иллюстрации ещё на кадр —
       * на пустом экране она всегда above the fold, lazy там не нужен ни
       * полноэкранному, ни секционному слоту. */}
      <Image
        src={imageSrc}
        alt={imageAlt}
        width={128}
        height={128}
        sizes="256px"
        quality={90}
        priority
        ref={rememberIllustrationRef}
        onLoad={onIllustrationLoad}
        className={cn('h-32 w-32', imageRounded && 'rounded-pill object-cover')}
      />
      {(title !== undefined || description !== undefined) && (
        <div className="mt-6 flex flex-col items-center gap-2">
          {title !== undefined && (
            <h2 className={cn('text-[24px] font-semibold leading-7 text-content', titleClassName)}>
              {title}
            </h2>
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
