// PROTOTYPE — throwaway, issue #106
'use client';

import { type JSX, useCallback, useEffect } from 'react';
import NextLink from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import clsx from 'clsx';
import { ArrowLeft, ArrowRight } from '@/shared/assets/icons';
import {
  parsePrototypeVariant,
  PROTOTYPE_CALENDAR_ROUTE,
  PROTOTYPE_VARIANT_NAMES,
  PROTOTYPE_VARIANT_ORDER,
  type PrototypeVariant,
} from '../model/variant';
import styles from './PrototypeCalendarScaffold.module.css';

const SURFACES: readonly { readonly href: string; readonly label: string }[] = [
  { href: PROTOTYPE_CALENDAR_ROUTE, label: 'Календарь' },
];

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  const tagName = target.tagName;
  return (
    tagName === 'INPUT' ||
    tagName === 'TEXTAREA' ||
    tagName === 'SELECT' ||
    target.isContentEditable
  );
}

export function PrototypeCalendarScaffold(): JSX.Element | null {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const router = useRouter();

  const variant: PrototypeVariant = parsePrototypeVariant(searchParams.get('variant'));

  const switchVariant = useCallback(
    (delta: number) => {
      const currentIndex = PROTOTYPE_VARIANT_ORDER.indexOf(variant);
      const nextIndex =
        (currentIndex + delta + PROTOTYPE_VARIANT_ORDER.length) % PROTOTYPE_VARIANT_ORDER.length;
      const params = new URLSearchParams(searchParams.toString());
      params.set('variant', PROTOTYPE_VARIANT_ORDER[nextIndex]);
      router.replace(`${pathname}?${params.toString()}`);
    },
    [pathname, searchParams, router, variant],
  );

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') {
        return;
      }
      if (isEditableTarget(event.target)) {
        return;
      }
      event.preventDefault();
      switchVariant(event.key === 'ArrowLeft' ? -1 : 1);
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [switchVariant]);

  if (process.env.NODE_ENV === 'production') {
    return null;
  }

  return (
    <>
      <nav className={styles.strip} aria-label="Навигация по прототипу">
        <span className={styles.stripBadge}>Прототип #106</span>
        {SURFACES.map((surface) => {
          const isCurrent = pathname.startsWith(surface.href);
          return (
            <NextLink
              key={surface.href}
              href={`${surface.href}?variant=${variant}`}
              aria-current={isCurrent ? 'page' : undefined}
              className={clsx(styles.stripLink, isCurrent && styles.stripLinkActive)}
            >
              {surface.label}
            </NextLink>
          );
        })}
      </nav>

      <div className={styles.switcher} role="group" aria-label="Переключение варианта прототипа">
        <button
          type="button"
          className={styles.switcherArrow}
          aria-label="Предыдущий вариант"
          onClick={() => switchVariant(-1)}
        >
          <ArrowLeft aria-hidden="true" />
        </button>
        <span className={styles.switcherLabel}>
          <span className={styles.switcherLetter}>Вариант {variant}</span>
          <span className={styles.switcherName}>{PROTOTYPE_VARIANT_NAMES[variant]}</span>
        </span>
        <button
          type="button"
          className={styles.switcherArrow}
          aria-label="Следующий вариант"
          onClick={() => switchVariant(1)}
        >
          <ArrowRight aria-hidden="true" />
        </button>
      </div>
    </>
  );
}
