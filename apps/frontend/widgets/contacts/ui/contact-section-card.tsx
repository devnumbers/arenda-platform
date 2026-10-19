import type { JSX, ReactNode } from 'react';
import Link from 'next/link';
import { SmallArrowRight } from '@/shared/assets/icons';
import styles from './contact-section-card.module.css';

export type ContactSectionCardProps = {
  readonly title: string;
  /** Адрес секции: вся площадь карточки кликабельна (оверлей-канон
   * PropertySectionCard), клавиатурная ссылка — заголовок. Без href —
   * секция статична без шеврона: «Информация» без ссылки у всех — правка
   * через кебаб шапки (решение владельца 18.10), ссылка осталась у
   * «Объекта». */
  readonly href?: string;
  readonly children: ReactNode;
  /** Отступ секции сверху: первая под шапкой ФИО — mt-8 (макет 3304:75726:
   * 32 от блока ФИО), остальные — mt-4 (канон PropertySectionCard). */
  readonly className?: string;
};

/**
 * Секционная карточка контакта (макеты 3304:75726 / 3304:76141, тикет
 * #1333): серая карточка (bg-surface-muted, radius 24) со строкой
 * заголовка — название 20/24 SemiBold (Heading-канон, H3/600) и шеврон 24
 * у правого края (gap 12, строка pt-24 px-24) — и содержимым ниже
 * (свой нижний паддинг). Анатомия и кликабельность — канон
 * PropertySectionCard (деталь объекта, карта #984): у секции с href вся
 * площадь — ссылка (оверлей под контентом), интерактивные контролы
 * контента (кнопки копирования строк) спасаются CSS-модулем; оверлей
 * скрыт от клавиатуры и скринридера — ссылка секции для них одна,
 * заголовок. Собственный экземпляр, а не импорт из property-detail:
 * sideways-импорты между виджетами запрещены (FSD, eslint boundaries);
 * второе потребление Heading-канона — кандидат на переезд в
 * shared/ui/design, вне скоупа тикета #1333.
 */
export function ContactSectionCard({
  title,
  href,
  children,
  className = 'mt-4',
}: ContactSectionCardProps): JSX.Element {
  const header = (
    // w-full обязателен: внутри flex-ссылки иначе div сжимается до
    // контента и шеврон прижимается к заголовку.
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
    <section
      className={`relative rounded-card bg-surface-muted ${className}`}
      data-testid="contact-section-card"
    >
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
       * клики нечем и незачем — текст остаётся выделяемым. */}
      <div className={href !== undefined ? styles.content : undefined}>{children}</div>
    </section>
  );
}
