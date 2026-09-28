import type { ComponentProps, JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';
import { CircleIcon } from './circle-icon';
import { Skeleton } from './skeleton';

/** Кнопка профиля дизайн-слоя (Figma 699:8867, «User Button»): имя
 * (M/500 14/16, #171A1C → hover #9FA8AC → active #6F787C) и справа
 * аватар-плейсхолдер — канонный круглый слот CircleIcon variant="white":
 * круг 44×44 на #F3F4F6 с белым гало 2.5px и глифом Bold/User 24×24
 * (заливка #D3D7D9 запечена в SVG — это тон плейсхолдера, а не контекстный
 * цвет). Focus-visible — обводка 2px #2B7FFF, только с
 * клавиатуры. Полный «Category Icon» с бейджами (Check/Danger/точка) —
 * отдельный компонент набора 651:5925, сюда не входит.
 *
 * pending — useMe ещё в полёте (§7): вместо имени — скелетон-полоска
 * детерминированной ширины, ширина кнопки стабильна. Текстовый
 * плейсхолдер («Пользователь») раздувал крыло бара и наезжал на заголовок
 * хаба на мобайле (аудит #876); без pending имя опционально —
 * терминальные состояния (ошибка чтения, имя-null) рендерит вызывающий,
 * подставляя плейсхолдер. Когда имени не видно (pending или
 * hideNameBelowDesktop), кнопка получает доступное имя «Профиль: {имя}»
 * — на десктопе aria-label не ставится, скринридер читает видимое имя. */
export type UserButtonProps = ComponentProps<'button'> & {
  readonly name?: string;
  readonly pending?: boolean;
  /** Имя скрыто ниже ПК (аудит #876: крыло хаба — аватар-only на
   * мобайле/планшете, ширина константна); кнопка получает aria-label
   * «Профиль», пока имени не видно. */
  readonly hideNameBelowDesktop?: boolean;
};

export function UserButton({
  name,
  pending = false,
  hideNameBelowDesktop = false,
  className,
  ...props
}: UserButtonProps): JSX.Element {
  const nameVisible = !pending && !hideNameBelowDesktop;
  return (
    <button
      aria-label={nameVisible ? undefined : name ? `Профиль: ${name}` : 'Профиль'}
      className={cn(
        'inline-flex cursor-pointer items-center gap-3 rounded-pill px-[14px] font-sans text-sm font-medium leading-4 text-content outline-none transition-colors',
        'hover:text-content-tertiary active:text-content-secondary',
        'focus-visible:ring-2 focus-visible:ring-primary',
        'disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      {...props}
    >
      {pending ? (
        <Skeleton className="h-4 w-10" />
      ) : (
        name !== undefined && (
          <span className={hideNameBelowDesktop ? 'hidden desktop:inline' : undefined}>{name}</span>
        )
      )}
      <CircleIcon variant="white" aria-hidden>
        <BoldUser className="h-6 w-6" />
      </CircleIcon>
    </button>
  );
}
