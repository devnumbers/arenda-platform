import type { JSX, ReactNode } from 'react';
import Link from 'next/link';
import Image from 'next/image';
import { SmallArrowRight } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/design';
import type { PropertySectionEmptyCopy } from '../lib/property-sections';

export type PropertySectionCardProps = {
  readonly title: string;
  /** Ссылка шапки секции (шеврон у правого края). Без href — как у
   * «Управления» и «Данных» — заголовок статичен. */
  readonly href?: string;
  readonly children: ReactNode;
  /** Отступ секции сверху: первая под медиа-блоком — mt-20 (Figma 98469:
   * 80px от адреса), остальные — mt-4. */
  readonly className?: string;
};

/** Секция-карточка детали объекта (Figma 1554:98481 / 1554:99561, решение
 * владельца 11.09): серая карточка (bg-surface-muted, radius 24) со
 * строкой заголовка — название 20/24 SemiBold (Mobile/Heading/H3/600) и
 * шеврон 24 у правого края (gap 12), строка pt-24 px-24 — и содержимым
 * ниже. Нижний паддинг несёт содержимое (у пустого — py-24, у
 * «Управления» — 12, у карточек «Об объекте» — 24/32 по макету). */
export function PropertySectionCard({
  title,
  href,
  children,
  className = 'mt-4',
}: PropertySectionCardProps): JSX.Element {
  const header = (
    <div className="flex items-center gap-3 px-6 pt-6">
      <span className="min-w-0 flex-1 text-xl font-semibold leading-6 text-content">
        {title}
      </span>
      {href !== undefined && (
        <SmallArrowRight className="h-6 w-6 shrink-0 text-content-tertiary" aria-hidden />
      )}
    </div>
  );
  return (
    <section className={`rounded-card bg-surface-muted ${className}`}>
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

/** Пустое состояние внутри секции (Figma 1554:98481 / 1554:99561,
 * решение владельца 11.09): контент px-48 py-24 — иллюстрация 64, через
 * 16 текст, через 24 кнопка (radius m, 44px). Две анатомии: с описанием
 * (первый объект и статусные состояния аренды) — тёмный тайтл 20/24
 * SemiBold + серое описание 14/16; короткая (второй и далее) — одна
 * серая строка 14/16 с текстом тайтла. Анатомия буквальна по макетам;
 * канонный полноэкранный EmptyState (§7) сюда не подходит по размеру. */
export function PropertySectionEmpty({
  imageSrc,
  copy,
  onCta,
  ctaDisabled = false,
}: PropertySectionEmptyProps): JSX.Element {
  return (
    <div className="flex flex-col items-center px-12 py-6 text-center">
      <Image src={imageSrc} alt="" width={64} height={64} className="h-16 w-16" />
      {copy.description !== null ? (
        <div className="mt-4 flex flex-col items-center gap-2">
          <h3 className="text-xl font-semibold leading-6 text-content">{copy.title}</h3>
          <p className="text-sm leading-4 text-content-secondary">{copy.description}</p>
        </div>
      ) : (
        <p className="mt-4 text-sm leading-4 text-content-secondary">{copy.title}</p>
      )}
      {copy.ctaLabel !== null && onCta !== undefined && (
        <Button
          variant="primary"
          size="small"
          radius="m"
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
