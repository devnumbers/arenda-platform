'use client';

import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { EmptyState } from '@/shared/ui/empty-state';

export function PropertySuspendedScreen(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/empty-logo.png"
      imageAlt="Логотип"
      title="Превышен лимит объектов"
      subtitle="Доступ к объекту приостановлен из-за лимита вашего тарифа. Он вернётся автоматически, когда освободится слот — например, после перехода на старший тариф или архивации другого объекта"
      actionHref={ROUTES.profileTariffChange}
      actionText="Выбрать тариф"
    />
  );
}
