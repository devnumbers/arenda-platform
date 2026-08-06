'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { EmptyState } from '@/shared/ui/empty-state';

export function PropertyNotFoundScreen(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/empty-logo.png"
      imageAlt="Логотип"
      title="Объект не найден или у вас нет к нему доступа"
      subtitle="Проверьте ссылку или попросите владельца выдать вам доступ к объекту"
      actionHref={ROUTES.properties}
      actionText="К списку объектов"
    />
  );
}
