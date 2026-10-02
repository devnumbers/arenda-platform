import type { ComponentProps, JSX } from 'react';
import Link from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { BoldUser } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { CircleIcon } from './circle-icon';
import { Skeleton } from './skeleton';

/** Ссылка профиля дизайн-слоя (Figma 699:8867, «User Button»; решение
 * владельца 02.10.2026, макет 2329-148674 — отмена аватар-only аудита
 * #876): настоящий <Link> на /profile — переход работает на всех ярусах,
 * средний клик и «в новой вкладке» ведут туда же. Имя (M/500 14/16,
 * #171A1C → hover #9FA8AC → active #6F787C) видно на всех ярусах; справа
 * аватар-плейсхолдер — канонный круглый слот CircleIcon variant="white":
 * круг 44×44 на #F3F4F6 с белым гало 2.5px и глифом Bold/User 24×24
 * (заливка #D3D7D9 запечена в SVG — это тон плейсхолдера, а не контекстный
 * цвет). Focus-visible — обводка 2px #2B7FFF, только с клавиатуры. Полный
 * «Category Icon» с бейджами (Check/Danger/точка) — отдельный компонент
 * набора 651:5925, сюда не входит.
 *
 * pending — useMe ещё в полёте (§7): вместо имени — скелетон-полоска
 * детерминированной ширины, ширина ссылки стабильна. Текстовый
 * плейсхолдер («Пользователь») раздувал крыло бара и наезжал на заголовок
 * хаба на мобайле (аудит #876); без pending имя опционально —
 * терминальные состояния (ошибка чтения, имя-null) рендерит вызывающий,
 * подставляя плейсхолдер. Пока имени не видно (pending или имя не
 * принесено), ссылка получает доступное имя «Профиль: {имя}»/«Профиль» —
 * при видимом имени aria-label не ставится: доступным именем служит само
 * имя. */
export type UserButtonProps = Omit<ComponentProps<'a'>, 'href'> & {
  readonly name?: string;
  readonly pending?: boolean;
};

export function UserButton({
  name,
  pending = false,
  className,
  ...props
}: UserButtonProps): JSX.Element {
  // Имя видно, только если оно есть: без имени и вне pending в дереве нет
  // никакого текста (иконка aria-hidden) — ссылке нужно доступное имя.
  const nameVisible = !pending && name !== undefined && name !== '';
  return (
    <Link
      href={ROUTES.profile}
      aria-label={nameVisible ? undefined : name ? `Профиль: ${name}` : 'Профиль'}
      className={cn(
        'inline-flex cursor-pointer items-center gap-3 rounded-pill px-[14px] font-sans text-sm font-medium leading-4 text-content outline-none transition-colors',
        'hover:text-content-tertiary active:text-content-secondary',
        'focus-visible:ring-2 focus-visible:ring-primary',
        className,
      )}
      {...props}
    >
      {pending ? (
        <Skeleton className="h-4 w-10" />
      ) : (
        name !== undefined && <span>{name}</span>
      )}
      <CircleIcon variant="white" aria-hidden>
        <BoldUser className="h-6 w-6" />
      </CircleIcon>
    </Link>
  );
}
