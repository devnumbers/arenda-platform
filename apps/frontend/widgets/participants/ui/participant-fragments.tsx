'use client';

import { useState, type JSX } from 'react';
import { BoldHome, BoldObjects, CheckBoxFalse, CheckBoxTrue, EditSmall, EyeSmall, FullAccessSmall, Minus } from '@/shared/assets/icons';
import NextLink from 'next/link';
import { ROUTES } from '@/shared/config/routes';
import { type PropertyType } from '@/entities/property';
import { propertyTypeIcons } from '@/entities/property';
import { cn } from '@/shared/lib/cn';
import { buttonVariants, CircleIcon, EmptyState } from '@/shared/ui/design';

/** Иконка роли у почты в рядах и карточках участников (макет 1980-107096,
 * решение владельца 13.10.2026 #1389): владелец — открытый замок,
 * «Управление» — карандаш, «Просмотр» — глаз; ключ —
 * PropertyParticipantEmailIcon (глиф выбирает emailRoleIcon, здесь только
 * имя → компонент). Одна карта на три поверхности (ряды «Участников
 * объекта», D7-карточка владельческого экрана, member-режим прав #1362). */
export const EMAIL_ROLE_ICONS = {
  full: FullAccessSmall,
  edit: EditSmall,
  eye: EyeSmall,
} as const;

/** Базовый класс кликабельного ряда списков участника (Row Button
 * 936:39348): общий для страницы участника, экрана прав и мультичека
 * приглашения; роль ряда (кнопка/checkbox) задаёт потребитель. */
export const PARTICIPANT_ROW_BASE_CLASS =
  'flex w-full cursor-pointer items-center gap-3 py-3 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80';

/** Row Button нижних строк макета 2177-59620: ведущий слот-круг 44 под
 * иконку 24, подпись 16/18 Medium; высота строки 52 (44 + py 4). Общая
 * для «Действий участника в объекте» и красного отзыва — иконки обеих
 * строк стоят на одной оси; member-режим прав (#1362) берёт класс для
 * своей строки действий. */
export const RIGHTS_ROW_ACTION_CLASS = cn(
  'flex w-full cursor-pointer items-center rounded-button py-1 text-left text-base font-medium outline-none',
  'transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface active:opacity-80',
);

/** Круглый аватар объекта 44px (Category Icon из Figma): фото или глиф —
 * Bold/Objects у агрегатной строки «Все объекты», глиф типа объекта
 * (Category Icon, карта #1217; без типа — Bold/Home) у конкретных
 * объектов; белое кольцо 2.5px по компоненту (2008-81468). Гард '' —
 * канон UserAvatar: легаси-бэк мог отдать пустую строку вместо null, а
 * <img src=""> не стреляет onError в WebKit — без него круг без глифа
 * (страница участника, фикс 14.10.2026). */
export function ObjectAvatarGlyph({
  photoUrl,
  type,
  isAll = false,
}: {
  readonly photoUrl: string | undefined;
  readonly type?: PropertyType;
  readonly isAll?: boolean;
}): JSX.Element {
  // Выборка из статичного реестра, не вызов: react-hooks/static-components.
  const Glyph = type !== undefined ? propertyTypeIcons[type] : BoldHome;
  // Битое фото (404 стрима) откатывается к глифу — канон #1275 (#1286).
  const [photoBroken, setPhotoBroken] = useState(false);
  const showPhoto = photoUrl !== undefined && photoUrl !== '' && !photoBroken;
  return (
    <CircleIcon variant="white" aria-hidden className="relative overflow-hidden rounded-full">
      {showPhoto ? (
        <img
          src={photoUrl}
          alt=""
          className="h-full w-full object-cover"
          onError={() => setPhotoBroken(true)}
        />
      ) : isAll ? (
        <BoldObjects className="h-6 w-6 text-content-tertiary" />
      ) : (
        <Glyph className="h-6 w-6 text-content-tertiary" />
      )}
    </CircleIcon>
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

/** 404-канон невидимого объекта (#719, копирайт PropertyNotFoundScreen):
 * свой доступ отозван/приостановлен в открытой сессии — перечитывание
 * отвечает 404, бэк скрывает нечитаемый объект как 404. Фрагмент здесь,
 * а не импорт из чужого виджета — cross-slice импорт виджетов запрещён.
 * Канонный EmptyState (легаси снесён, #901); действие — ссылка-кнопка
 * NextLink+buttonVariants (next/link не дружит с Radix Slot — прецедент
 * канонного button.tsx). */
export function PropertyAccessNotFound(): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/empty-logo.webp"
      imageAlt="Логотип"
      title="Объект не найден или у вас нет к нему доступа"
      description="Проверьте ссылку или попросите владельца выдать вам доступ к объекту"
      action={
        <NextLink
          href={ROUTES.properties}
          className={buttonVariants({ size: 'small', radius: 'm' })}
        >
          К списку объектов
        </NextLink>
      }
    />
  );
}

/** Пикторальный Selection Button (Figma 1031:21053; канон-иконки
 * CheckBoxTrue/False — решение владельца 07.09, вне канон-набора):
 * «mixed» («Все объекты» — выбрана часть, 1858:105670) — синий квадрат
 * радиусом 8 с белым Icon/R/Minus 16. Ряд несёт role="checkbox" —
 * глиф только рисует состояние (прецедент селектов задач/контактов).
 * Общий для мультичеков приглашений #698 и #699. */
export function SelectionGlyph({
  state,
}: {
  readonly state: 'on' | 'off' | 'mixed';
}): JSX.Element {
  if (state === 'mixed') {
    return (
      <span
        aria-hidden
        className="flex h-6 w-6 shrink-0 items-center justify-center rounded-lg bg-primary"
      >
        <Minus className="h-4 w-4 text-white" />
      </span>
    );
  }
  return state === 'on' ? (
    <CheckBoxTrue className="h-6 w-6 shrink-0" aria-hidden />
  ) : (
    <CheckBoxFalse className="h-6 w-6 shrink-0" aria-hidden />
  );
}
