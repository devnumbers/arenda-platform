import type { JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { buttonVariants, EmptyState } from '@/shared/ui/design';

/** 404-канон объекта: ссылки нет или доступ не выдан. Канонный EmptyState (легаси снесён, #901): действие — ссылка-
 * кнопка NextLink+buttonVariants (next/link не дружит с Radix Slot —
 * прецедент канонного button.tsx). */
export function PropertyNotFoundScreen(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/empty-logo.png"
      imageAlt="Логотип"
      title="Объект не найден или у вас нет к нему доступа"
      description="Проверьте ссылку или попросите владельца выдать вам доступ к объекту"
      action={(
        <NextLink href={ROUTES.properties} className={buttonVariants({ size: 'small' })}>
          К списку объектов
        </NextLink>
      )}
    />
  );
}
