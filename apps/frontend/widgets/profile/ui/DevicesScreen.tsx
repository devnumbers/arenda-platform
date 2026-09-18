'use client';

import { useState, type ComponentType, type JSX } from 'react';
import {
  Computer,
  Exit,
  Phone,
  SmallArrowRight,
} from '@/shared/assets/icons';
import {
  Button,
  ConfirmDialog,
  ListRow,
  Modal,
  ModalContent,
  Skeleton,
} from '@/shared/ui/design';
import {
  useLogoutOtherSessions,
  useRevokeSession,
  useSessions,
} from '@/features/profile';
import type { SessionDevice, SessionDeviceType } from '@/entities/session';
import { cn } from '@/shared/lib/cn';
import { notify } from '@/shared/lib/notifications';
import {
  deviceIconName,
  sessionSubtitle,
  sessionTitle,
  splitSessions,
} from '../lib/devices';

/** Иконка типа устройства — два глифа макета (канон §10: файл один,
 * размер задаёт потребитель). */
const deviceIcons: Record<'phone' | 'computer', ComponentType<{ className?: string }>> = {
  phone: Phone,
  computer: Computer,
};

function DeviceGlyph({
  deviceType,
  className,
}: {
  readonly deviceType: SessionDeviceType;
  readonly className?: string;
}): JSX.Element {
  const Icon = deviceIcons[deviceIconName(deviceType)];
  return <Icon className={className} />;
}

/** Серая строка устройства (мок 1804-105061: карточка #F3F4F6 radius 24,
 * паддинг 16/12, слот иконки 44, заголовок 16/18 + подзаголовок 14/16,
 * шеврон в круге 44). Тело — канон ListRow: встроенные px-6/py-2/gap-3
 * переопределяются классом карточки (tailwind-merge), title/subtitle —
 * анатомия Row Button. Подзаголовок текущей сессии синий #2B7FFF, чужой —
 * серый #6F787C мока. Строка чужой сессии открывает шит завершения;
 * текущая неинтерактивна (завершается выходом на хабе, #728). */
function DeviceRow({
  session,
  onSelect,
}: {
  readonly session: SessionDevice;
  readonly onSelect?: () => void;
}): JSX.Element {
  const subtitle = sessionSubtitle(session, new Date());

  return (
    <ListRow
      className="gap-2 rounded-3xl bg-surface-muted px-3 py-4"
      leading={
        <span aria-hidden className="flex h-11 w-11 items-center justify-center text-content">
          <DeviceGlyph deviceType={session.deviceType} className="h-6 w-6" />
        </span>
      }
      title={sessionTitle(session)}
      subtitle={subtitle.text}
      subtitleClassName={subtitle.online ? 'text-primary' : 'text-content-secondary'}
      trailing={
        onSelect !== undefined ? (
          <span
            aria-hidden
            className="flex h-11 w-11 shrink-0 items-center justify-center text-content-tertiary"
          >
            <SmallArrowRight className="h-6 w-6" />
          </span>
        ) : undefined
      }
      onSelect={onSelect}
    />
  );
}

/** Заголовок секции — канон Heading макета (H3 20/24 SemiBold, паддинг
 * 24 горизонталей даёт SubScreenShell). */
function SectionHeading({ children }: { readonly children: string }): JSX.Element {
  return <h2 className="m-0 text-xl font-semibold leading-6 text-content">{children}</h2>;
}

/** Скелетон строки — форма карточки DeviceRow (паритет #604): круг-слот
 * иконки, заголовок, подзаголовок и шеврон, как у чужой строки; шеврона
 * нет в варианте текущей сессии (она неинтерактивна) — блок гасится
 * пропом. */
function DeviceRowSkeleton({ chevron = true }: { readonly chevron?: boolean }): JSX.Element {
  return (
    <div className="flex items-center gap-2 rounded-3xl bg-surface-muted px-3 py-4">
      <Skeleton className="h-11 w-11 shrink-0 bg-surface-muted-hover" />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <Skeleton className="h-5 w-36 bg-surface-muted-hover" />
        <Skeleton className="h-4 w-48 bg-surface-muted-hover" />
      </div>
      {chevron && <Skeleton className="h-6 w-6 shrink-0 bg-surface-muted-hover" />}
    </div>
  );
}

function DevicesScreenSkeleton(): JSX.Element {
  return (
    <div role="status" aria-label="Загрузка устройств" className="flex flex-col">
      <Skeleton className="mb-4 h-6 w-40" />
      {/* Паритет (#604): текущая строка без шеврона, красное действие и
       * секция «Активные сессии» — как при наличии чужих сессий. */}
      <DeviceRowSkeleton chevron={false} />
      <div className="mt-2 flex h-6 items-center px-[22px]">
        <Skeleton className="h-6 w-6 bg-surface-muted-hover" />
        <Skeleton className="ml-[18px] h-5 w-56 bg-surface-muted-hover" />
      </div>
      <Skeleton className="mb-4 mt-6 h-6 w-40" />
      <div className="flex flex-col gap-2">
        <DeviceRowSkeleton />
        <DeviceRowSkeleton />
        <DeviceRowSkeleton />
      </div>
    </div>
  );
}

/** Экран «Устройства» (#730, карта #724; моки 1804-105061, 1903-39135,
 * 1903-38679): секция «Это устройство» (текущая сессия, синий «В сети» •
 * город), красное действие «Завершить все другие сессии» (Icon/R/Exit) и
 * «Активные сессии» — чужие сессии по свежей активности. Шит «Завершить
 * сессии» — канон ConfirmDialog по моку 1903-39135 (описание 16/18 R/400 —
 * descriptionClassName="text-base", прецедент #627); шит одной сессии —
 * мок 1903-38679: иконка устройства 48, заголовок H3 по центру, детали
 * 16/18 серым, «Отменить» + danger-«Завершить» — сборка канона Modal
 * (sr-only title для a11y-имени). Тексты говорят доменным каноном
 * «сессия» (identity/CONTEXT.md), не «сеанс» макета. Если чужих сессий
 * нет — действие и секция скрыты (действия нет — не рисуется). */
export function DevicesScreen(): JSX.Element {
  const { data: sessions, isError, isLoading, refetch } = useSessions();
  const revoke = useRevokeSession();
  const logoutOthers = useLogoutOtherSessions();
  const [othersSheetOpen, setOthersSheetOpen] = useState(false);
  const [target, setTarget] = useState<SessionDevice | null>(null);

  const confirmLogoutOthers = (): void => {
    logoutOthers.mutate(undefined, {
      onSuccess: () => setOthersSheetOpen(false),
      onError: (error) => notify.scenarios.profile.logoutOthersError(error),
    });
  };

  const confirmRevoke = (): void => {
    if (target === null) {
      return;
    }
    revoke.mutate(target.id, {
      onSuccess: () => setTarget(null),
      onError: (error) => notify.scenarios.profile.sessionRevokeError(error),
    });
  };

  if (isError) {
    return (
      <div className="flex flex-col items-center gap-4 rounded-3xl bg-surface-muted px-6 py-8">
        <p className="text-sm text-content-secondary">Не удалось загрузить устройства</p>
        <Button variant="white" onClick={() => void refetch()}>
          Повторить
        </Button>
      </div>
    );
  }

  if (isLoading || sessions === undefined) {
    return <DevicesScreenSkeleton />;
  }

  const { current, others } = splitSessions(sessions);

  return (
    <div className="flex flex-col">
      {current !== null && (
        <>
          <SectionHeading>Это устройство</SectionHeading>
          <div className="mt-4">
            <DeviceRow session={current} />
          </div>
        </>
      )}

      {others.length > 0 && (
        <>
          <button
            type="button"
            onClick={() => setOthersSheetOpen(true)}
            className={cn(
              'mt-2 flex w-full cursor-pointer items-center gap-[18px] px-[22px] text-left outline-none',
              'transition-opacity hover:opacity-80 active:opacity-80',
              'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
            )}
          >
            <span aria-hidden className="flex h-6 w-6 shrink-0 items-center justify-center">
              <Exit className="h-6 w-6 text-danger" />
            </span>
            <span className="text-base font-medium text-danger">
              Завершить все другие сессии
            </span>
          </button>

          <div className="mt-6">
            <SectionHeading>Активные сессии</SectionHeading>
          </div>
          <div className="mt-4 flex flex-col gap-2">
            {others.map((session) => (
              <DeviceRow
                key={session.id}
                session={session}
                onSelect={() => setTarget(session)}
              />
            ))}
          </div>
        </>
      )}

      <ConfirmDialog
        open={othersSheetOpen}
        onOpenChange={setOthersSheetOpen}
        title="Завершить сессии"
        description="Уверены, что хотите завершить все другие сессии, кроме текущей?"
        descriptionClassName="text-base leading-[18px]"
        confirmLabel="Завершить"
        cancelLabel="Отменить"
        confirmVariant="danger"
        pending={logoutOthers.isPending}
        onConfirm={confirmLogoutOthers}
      />

      <Modal
        open={target !== null}
        onOpenChange={(open) => {
          if (!open && !revoke.isPending) {
            setTarget(null);
          }
        }}
      >
        {target !== null && (
          <ModalContent title="Завершить сессию" titleSrOnly>
            <div className="flex flex-col items-center gap-8">
              <span aria-hidden className="flex h-12 w-12 items-center justify-center text-content">
                <DeviceGlyph deviceType={target.deviceType} className="h-12 w-12" />
              </span>
              <div className="flex flex-col items-center gap-2">
                <p className="m-0 text-xl font-semibold leading-6 text-content">
                  {sessionTitle(target)}
                </p>
                <p className="m-0 text-base leading-[18px] text-content-secondary">
                  {sessionSubtitle(target, new Date()).text}
                </p>
              </div>
              <div className="grid w-full grid-cols-2 gap-2">
                <Button
                  variant="secondary"
                  disabled={revoke.isPending}
                  onClick={() => setTarget(null)}
                >
                  Отменить
                </Button>
                <Button variant="danger" loading={revoke.isPending} onClick={confirmRevoke}>
                  Завершить
                </Button>
              </div>
            </div>
          </ModalContent>
        )}
      </Modal>
    </div>
  );
}
