'use client';

import { type ComponentType, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import {
  Button,
  IconButton,
  PageContent,
  Skeleton,
  StatusIcon,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import { useMe } from '@/features/auth';
import type { User } from '@/entities/user';

/** Свойства потока смены: данные /me и закрытие экрана на аккаунт. */
export type MeFlowProps = {
  readonly me: User;
  readonly onClose: () => void;
};

/**
 * Обёртка экранов-потоков смены в профиле поверх /me («Изменение телефона»
 * #595, «Смена почты» #722): ошибка /me — шапка-статика и карточка
 * «Повторить»; загрузка — та же шапка и скелетон формы шага (§7 DESIGN.md);
 * с данными — сам поток. Закрытие — goBack на аккаунт (истории может не
 * быть — фолбэк). Шапка собирается на TopNav: у потоков состав слотов
 * меняется по шагам, в состояниях обёртки — только «Назад».
 */
export function MeFlowScreen({
  title,
  Flow,
}: {
  readonly title: string;
  readonly Flow: ComponentType<MeFlowProps>;
}): JSX.Element {
  const router = useRouter();
  const { data: me, isPending, isError, refetch } = useMe();

  const close = (): void => goBack(router, ROUTES.profileAccount);

  return (
    <>
      {isError && (
        <>
          <StaticHeader title={title} onClose={close} />
          <PageContent className="px-6">
            <div className="flex flex-col items-center gap-4 rounded-3xl bg-surface-muted px-6 py-8">
              <p className="m-0 text-sm text-content-secondary">Не удалось загрузить данные</p>
              <Button variant="white" onClick={() => void refetch()}>
                Повторить
              </Button>
            </div>
          </PageContent>
        </>
      )}
      {!isError && isPending && (
        <>
          <StaticHeader title={title} onClose={close} />
          <PageContent className="px-6">
            {/* Скелетон — форма шага (§7 DESIGN.md): заголовок + бокс поля,
             * внизу — шит с «кнопкой». */}
            <div role="status" aria-label="Загрузка" className="flex flex-col gap-4">
              <Skeleton className="h-6 w-3/4" />
              <Skeleton className="h-14 w-full rounded-button" />
            </div>
          </PageContent>
          <StickyBottomBar>
            <Skeleton className="h-14 w-full rounded-button" />
          </StickyBottomBar>
        </>
      )}
      {!isError && !isPending && <Flow me={me} onClose={close} />}
    </>
  );
}

/** Шапка состояний загрузки/ошибки: «Назад» на аккаунт, заголовок — как у
 * шагов потока. */
function StaticHeader({
  title,
  onClose,
}: {
  readonly title: string;
  readonly onClose: () => void;
}): JSX.Element {
  return (
    <TopNav
      leading={<IconButton icon={<ArrowLeft />} label="Назад" onClick={onClose} />}
    >
      <TopNavTitle title={title} />
    </TopNav>
  );
}

/**
 * Шаг «успех» потоков смены (телефон 1869-68303, почта 105223): галка good,
 * сообщение с новым значением, шит «Хорошо» возвращает на аккаунт; в шапке
 * только крест. Экран сам подтверждение — тост успеха не дублируется.
 */
export function MeFlowSuccessScreen({
  message,
  onClose,
}: {
  readonly message: string;
  readonly onClose: () => void;
}): JSX.Element {
  return (
    <>
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />}
      />
      <PageContent>
        <div className="flex flex-col items-center px-6 pt-16">
          <StatusIcon status="good" className="h-24 w-24" />
          <h1 className="m-0 mt-10 text-center text-xl font-semibold leading-6 text-content">
            {message}
          </h1>
        </div>
      </PageContent>
      <StickyBottomBar>
        <Button className="w-full" onClick={onClose}>
          Хорошо
        </Button>
      </StickyBottomBar>
    </>
  );
}
