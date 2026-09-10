import type { JSX, ReactNode } from 'react';
import Link from 'next/link';
import Image from 'next/image';
import { SmallArrowRight } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/design';
import type { PropertySectionEmptyCopy } from '../lib/property-sections';

export type PropertySectionCardProps = {
  readonly title: string;
  /** Ссылка шапки секции (шеврон у правого края). Без href — как у
   * «Управления» — заголовок статичен. */
  readonly href?: string;
  readonly children: ReactNode;
  /** Отступ секции сверху: первая под медиа-блоком — mt-20 (Figma 98469:
   * 80px от адреса), остальные — mt-4. */
  readonly className?: string;
};

/** Секция-карточка детали объекта (Figma 1554:98469): серая карточка
 * (bg-surface-muted, radius карточки) со строкой заголовка 48 — название
 * 16/18 SemiBold и шеврон у правого края — и содержимым ниже. */
export function PropertySectionCard({
  title,
  href,
  children,
  className = 'mt-4',
}: PropertySectionCardProps): JSX.Element {
  const header = (
    <div className="flex h-12 items-center justify-between px-6">
      <span className="text-base font-semibold text-content">{title}</span>
      {href !== undefined && (
        <SmallArrowRight className="h-6 w-6 text-content-tertiary" aria-hidden />
      )}
    </div>
  );
  return (
    <section className={`rounded-card bg-surface-muted pb-6 ${className}`}>
      {href !== undefined ? (
        <Link
          href={href}
          className="flex outline-none transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-primary"
        >
          {header}
        </Link>
      ) : (
        header
      )}
      {children}
    </section>
  );
}

export type PropertySectionEmptyProps = {
  readonly imageSrc: string;
  readonly copy: PropertySectionEmptyCopy;
  /** Действие CTA «Добавить»; без CTA не рисуется (Операции, Figma 98469). */
  readonly onCta?: () => void;
  /** CTA глохнет (архив read-only, Figma 1581:52407). */
  readonly ctaDisabled?: boolean;
};

/** Пустое состояние внутри секции: иллюстрация 64 (40 после строки
 * заголовка, как в макете), заголовок 16/24 SemiBold, серое описание
 * 14/16 (короткий набор — без него) и кнопка «Добавить» 44 (24 после
 * описания). Контент по центру карточки. Анатомия — буквально из макета
 * секций детали (1554:98469); канонный полноэкранный EmptyState (§7)
 * сюда не подходит по размеру — вопрос расширения канона вариантом
 * размера открыт до решения владельца. */
export function PropertySectionEmpty({
  imageSrc,
  copy,
  onCta,
  ctaDisabled = false,
}: PropertySectionEmptyProps): JSX.Element {
  return (
    <div className="flex flex-col items-center px-6 pt-10 text-center">
      <Image src={imageSrc} alt="" width={64} height={64} className="h-16 w-16" />
      <h3 className="mt-4 max-w-[250px] text-base font-semibold leading-6 text-content">
        {copy.title}
      </h3>
      {copy.description !== null && (
        <p className="mt-2 max-w-[250px] text-sm leading-4 text-content-secondary">
          {copy.description}
        </p>
      )}
      {copy.ctaLabel !== null && onCta !== undefined && (
        <Button
          variant="primary"
          size="small"
          className="mt-6"
          disabled={ctaDisabled}
          onClick={onCta}
        >
          {copy.ctaLabel}
        </Button>
      )}
    </div>
  );
}
