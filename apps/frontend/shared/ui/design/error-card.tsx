'use client';

import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { Button } from './button';

export type ErrorCardProps = {
  /** Заголовок ошибки — потребитель называет, что не загрузилось
   * («Не удалось загрузить контакты», «…раздел», «…участников»). */
  readonly title: string;
  readonly onRetry: () => void;
  readonly className?: string;
};

/** Карточка ошибки загрузки с повтором — канон error-состояния списков
 * (DESIGN.md §7): серая карточка, заголовок 20/24, пояснение 14/16,
 * вторичная кнопка «Повторить». Экстракция #697: одинаковые карточки
 * контактов (#508) и хаба участников (#696) жили поэкранно. */
export function ErrorCard({ title, onRetry, className }: ErrorCardProps): JSX.Element {
  return (
    <section className={cn('mx-6 rounded-card bg-surface-muted px-6 py-6', className)}>
      <h2 className="text-xl font-semibold leading-6 text-content">{title}</h2>
      <p className="mt-2 text-sm leading-4 text-content-secondary">
        Проверьте подключение и попробуйте еще раз
      </p>
      <div className="mt-4">
        <Button size="small" variant="secondary" onClick={onRetry}>
          Повторить
        </Button>
      </div>
    </section>
  );
}
