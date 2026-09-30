import type { JSX, ReactNode } from 'react';
import Link from 'next/link';
import Image from 'next/image';
import { SmallArrowRight } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/design';
import type { PropertySectionEmptyCopy } from '../lib/property-sections';
import styles from './PropertySectionCard.module.css';

export type PropertySectionCardProps = {
  readonly title: string;
  /** Ссылка секции: вся площадь карточки кликабельна (оверлей, канон
   * PropertyCard). Без href — как у «Управления» и «Данных» — секция
   * статична. Ховер живёт только на строке заголовка (решение владельца
   * 30.09, карта #984): всему блоку — курсор, без реакции на наведение. */
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
 * «Управления» — 12, у карточек «Об объекте» — 24/32 по макету).
 *
 * Кликабельность (карта #984): у секции с href вся площадь — ссылка
 * (оверлей-канон PropertyCard, курсор без ховера на блоке), при этом
 * свои действия сохраняют заголовок (с его ховером затемнения) и
 * интерактивные контролы внутри контента (кнопки блоков, строки задач и
 * контактов, CTA пустых состояний) — CSS-спасение в модуле. Оверлей
 * скрыт от клавиатуры и скринридера (tabIndex -1 + aria-hidden):
 * ссылка секции для них одна — заголовок. */
export function PropertySectionCard({
  title,
  href,
  children,
  className = 'mt-4',
}: PropertySectionCardProps): JSX.Element {
  const header = (
    // w-full обязателен: внутри flex-ссылки (или секции) иначе div сжимается
    // до контента и шеврон прижимается к заголовку.
    <div className="flex w-full items-center gap-3 px-6 pt-6">
      <span className="min-w-0 flex-1 text-xl font-semibold leading-6 text-content">
        {title}
      </span>
      {href !== undefined && (
        <SmallArrowRight className="h-6 w-6 shrink-0 text-content-tertiary" aria-hidden />
      )}
    </div>
  );
  return (
    <section className={`relative rounded-card bg-surface-muted ${className}`}>
      {href !== undefined && (
        <Link
          href={href}
          aria-hidden
          tabIndex={-1}
          className="absolute inset-0 rounded-[inherit] outline-none"
        />
      )}
      {href !== undefined ? (
        // relative: над оверлеем (тот же слой по DOM-порядку) — иначе
        // оверлей забирает и ховер, и клик заголовка.
        <Link
          href={href}
          className="relative flex outline-none transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-primary"
        >
          {header}
        </Link>
      ) : (
        header
      )}
      {/* Passthrough контента нужен только под оверлеем: без href глушить
       * клики нечем и незачем — текст секции остаётся выделяемым. */}
      <div className={href !== undefined ? styles.content : undefined}>{children}</div>
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
 * 16 текстовый блок, внутри него 24 до кнопки (зазор держит gap
 * родителя: маргины на кнопках глушит безслойный normalize — globals
 * `button { margin: 0 }`). Две анатомии: с описанием (первый объект и
 * статусные состояния аренды) — тёмный тайтл 20/24 SemiBold + серое
 * описание 14/16 через 8; короткая (второй и далее) — одна серая строка
 * 14/16 с текстом тайтла. Кнопка — radius m, 44px. Анатомия буквальна
 * по макетам; канонный полноэкранный EmptyState (§7) сюда не подходит
 * по размеру. */
export function PropertySectionEmpty({
  imageSrc,
  copy,
  onCta,
  ctaDisabled = false,
}: PropertySectionEmptyProps): JSX.Element {
  return (
    <div className="flex flex-col items-center px-12 py-6 text-center">
      {/* sizes: слот 64 CSS — на 3x-экранах srcset отдаёт даунскейл из
       * источника вместо апскейла 128 (#1002); потолок задаёт источник. */}
      <Image
        src={imageSrc}
        alt=""
        width={64}
        height={64}
        sizes="64px"
        quality={90}
        className="h-16 w-16"
      />
      <div className="mt-4 flex w-full flex-col items-center gap-6">
        {copy.description !== null ? (
          <div className="flex flex-col items-center gap-2">
            <h3 className="text-xl font-semibold leading-6 text-content">{copy.title}</h3>
            <p className="text-sm leading-4 text-content-secondary">{copy.description}</p>
          </div>
        ) : (
          <p className="text-sm leading-4 text-content-secondary">{copy.title}</p>
        )}
        {copy.ctaLabel !== null && onCta !== undefined && (
          <Button
            variant="primary"
            size="small"
            radius="m"
            disabled={ctaDisabled}
            onClick={onCta}
          >
            {copy.ctaLabel}
          </Button>
        )}
      </div>
    </div>
  );
}
