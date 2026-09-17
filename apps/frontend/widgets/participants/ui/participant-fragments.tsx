'use client';

import type { JSX } from 'react';
import { BoldHome, BoldObjects } from '@/shared/assets/icons';

/** Базовый класс кликабельного ряда списков участника (Row Button
 * 936:39348): общий для страницы участника, экрана прав и мультичека
 * приглашения; роль ряда (кнопка/checkbox) задаёт потребитель. */
export const PARTICIPANT_ROW_BASE_CLASS =
  'flex w-full cursor-pointer items-center gap-3 py-3 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80';

/** Круглый аватар объекта 44px (Category Icon из Figma): фото или канонная
 * иконка — Bold/Objects у агрегатной строки «Все объекты», Bold/Home у
 * конкретных объектов; белое кольцо 2.5px по компоненту (2008-81468). */
export function ObjectAvatarGlyph({
  photoUrl,
  isAll = false,
}: {
  readonly photoUrl: string | undefined;
  readonly isAll?: boolean;
}): JSX.Element {
  return (
    <span
      aria-hidden
      className="relative flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-surface-muted shadow-[0_0_0_2.5px_var(--dl-surface)]"
    >
      {photoUrl !== undefined ? (
        <img src={photoUrl} alt="" className="h-full w-full object-cover" />
      ) : isAll ? (
        <BoldObjects className="h-6 w-6 text-content-tertiary" />
      ) : (
        <BoldHome className="h-6 w-6 text-content-tertiary" />
      )}
    </span>
  );
}

/** 404-политика deep-link (#693): человек вне сцопа читающего или уже
 * отозван — приватный 404, вежливый текст по центру (страница участника
 * #698, экраны прав и приглашения — тот же агрегат). */
export function ParticipantNotFound(): JSX.Element {
  return (
    <p className="pt-16 text-center text-base leading-[18px] text-content-secondary">
      Участник не найден
    </p>
  );
}
