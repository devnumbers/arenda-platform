import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { PageContent } from './page-content';
import { TopNav, TopNavTitle } from './top-nav';
import { TopNavBackButton } from './top-nav-back-button';

export type SubScreenShellProps = {
  /** Заголовок подэкрана в центре шапки (TopNavTitle, 16/18). */
  readonly title: ReactNode;
  /** Подзаголовок под заголовком (TopNavTitle subtitle, 14/16 серый) —
   * например счётчик объектов архива (#587). */
  readonly subtitle?: ReactNode;
  /** Фолбэк ведущей кнопки «Назад» при отсутствии истории — та же
   * семантика, что у TopNavBackButton (history-first goBack). */
  readonly fallbackHref: string;
  /** Действия в правом слоте шапки (кебаб-меню объекта и т.п.). */
  readonly trailing?: ReactNode;
  /** Класс контента поверх базового px-6 (tailwind-merge — можно
   * переопределить горизонтали, вертикали PageContent не вкладывает). */
  readonly contentClassName?: string;
  readonly children: ReactNode;
};

/** Каркас подэкрана (второй ярус и глубже, у хаба — mobileWings вместо
 * ведущей кнопки): TopNav с ведущим «Назад» + TopNavTitle в центре,
 * контент в PageContent с боковым паддингом 24 (прецедент дерева профиля
 * #566; «Назад» — history-first, фолбэк обязателен). Экстракция #568 —
 * однотипные шапки из ревью #566; серверная страница собирает каркас без
 * собственного 'use client'. */
export function SubScreenShell({
  title,
  subtitle,
  fallbackHref,
  trailing,
  contentClassName,
  children,
}: SubScreenShellProps): JSX.Element {
  return (
    <>
      <TopNav
        leading={<TopNavBackButton fallbackHref={fallbackHref} />}
        trailing={trailing}
      >
        <TopNavTitle title={title} subtitle={subtitle} />
      </TopNav>
      <PageContent className={cn('px-6', contentClassName)}>{children}</PageContent>
    </>
  );
}
