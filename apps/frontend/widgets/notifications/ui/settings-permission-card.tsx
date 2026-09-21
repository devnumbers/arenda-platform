'use client';

import type { JSX } from 'react';
import Image from 'next/image';
import { Button } from '@/shared/ui/design';

/** Inline-карточка «Разрешите пуши» (макет 2329-150165): живёт под
 * мастер-тумблером, пока он включён, а разрешение браузера не выдано.
 * «Разрешить» ведёт флоу разрешения (#746) без шита — сама карточка и есть
 * приглашение к системному промпту. Колокольчик — канонный 3D-белл ленты
 * (пустые состояния #744). */
export function PushPermissionCard({
  onAllow,
}: {
  readonly onAllow: () => void;
}): JSX.Element {
  return (
    <div className="mt-3 flex items-start gap-3 rounded-3xl bg-surface-muted p-4">
      <Image
        src="/images/notifications/empty-bell.png"
        alt=""
        width={64}
        height={64}
        className="h-16 w-16 shrink-0"
        unoptimized
      />
      <div className="flex min-w-0 flex-col gap-2">
        <p className="m-0 text-base font-semibold leading-5 text-content">Разрешите пуши</p>
        <p className="m-0 text-sm leading-[18px] text-content-tertiary">
          Разрешите пуш-уведомления в браузере, чтобы получать уведомления
        </p>
        <Button variant="primary" size="small" className="self-start" onClick={onAllow}>
          Разрешить
        </Button>
      </div>
    </div>
  );
}
