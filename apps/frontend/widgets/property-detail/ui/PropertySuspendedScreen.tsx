import type { JSX } from 'react';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { buttonVariants, EmptyState } from '@/shared/ui/design';

/** Лимит объектов тарифа исчерпан — доступ к объекту приостановлен.
 * Канонный EmptyState (легаси снесён, #901): действие — ссылка-кнопка
 * NextLink+buttonVariants (next/link не дружит с Radix Slot — прецедент
 * канонного button.tsx). */
export function PropertySuspendedScreen(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/empty-logo.webp"
      imageAlt="Логотип"
      title="Превышен лимит объектов"
      description="Доступ к объекту приостановлен из-за лимита вашего тарифа. Он вернётся автоматически, когда освободится слот — например, после перехода на старший тариф или архивации другого объекта"
      action={
        <NextLink href={ROUTES.profileTariffChange} className={buttonVariants({ size: 'small' })}>
          Выбрать тариф
        </NextLink>
      }
    />
  );
}
