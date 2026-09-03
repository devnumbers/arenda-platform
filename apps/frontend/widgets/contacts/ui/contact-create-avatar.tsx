'use client';

import type { JSX } from 'react';
import { BoldUser } from '@/shared/assets/icons';
import { Button } from '@/shared/ui/design';

/**
 * Блок аватара из макета создания контакта (1281:48439): серый круг 96px с
 * «болдом» пользователя и кнопка «Добавить фото». Фото в контракте #507
 * нет (ContactCreateRequest без аватара), поэтому кнопка пока без действия —
 * открытое решение владельца на приёмке #509: либо фото пойдёт следующим
 * срезом, либо блок уйдёт из макета.
 */
export function ContactCreateAvatar(): JSX.Element {
  return (
    <div className="flex flex-col items-center gap-2">
      <div className="flex h-24 w-24 items-center justify-center rounded-full bg-surface-muted">
        <BoldUser className="h-13 w-13 text-content-tertiary" />
      </div>
      <Button variant="clear" className="text-base font-medium">
        Добавить фото
      </Button>
    </div>
  );
}
