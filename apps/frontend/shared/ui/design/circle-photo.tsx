'use client';

import { useState, type JSX, type ReactElement } from 'react';
import { cn } from '@/shared/lib/cn';

/** Флаги жизни фото в круге: битое (onError) и готовое (onLoad). */
export type CirclePhotoFlags = {
  readonly photoBroken: boolean;
  readonly photoReady: boolean;
};

/** Что показывает круг фото: чистое решение — единственное место гарда
 * пустого пути (WebKit: <img src=""> не стреляет onError — канон
 * UserAvatar) и правила «глиф под фото» (глиф-подложка живёт в ожидании
 * и при ошибке, тикет #1423). Вынесено из компонента, чтобы покрываться
 * юнит-тестами — тестовое окружение фронта исполняет только чистую
 * логику (node, без DOM; прецедент circle-icon-variants). */
export function circlePhotoView(
  photoUrl: string | null | undefined,
  flags: CirclePhotoFlags,
): { readonly imgSrc: string | null; readonly imgHidden: boolean } {
  if (photoUrl == null || photoUrl === '' || flags.photoBroken) {
    return { imgSrc: null, imgHidden: false };
  }
  return { imgSrc: photoUrl, imgHidden: !flags.photoReady };
}

export type CirclePhotoProps = {
  /** Стриминговый путь выдачи (ADR 0065); null/undefined/'' — только глиф. */
  readonly photoUrl?: string | null;
  /** Глиф-подложка: тип объекта / BoldUser — выбирает вызывающий; живёт
   * и в ожидании байтов, и при битом фото (фолбэк #1275). */
  readonly glyph: ReactElement;
  /** Классы обёртки-круга (размер поверх родителя). */
  readonly className?: string;
};

/**
 * Содержимое клипающего круга пользовательского фото — примитив канона
 * «глиф под фото + проявление по onLoad» (ADR 0065, тикет #1423): пока
 * байты летят (холодный вход, мобильная сеть), круг показывает
 * глиф-подложку вместо пустого фона; готовое фото проявляется на дефолтных
 * transition-токенах дома (--dl-duration-fast/--dl-ease, globals.css @theme).
 * Родитель обязан клипать (overflow-hidden rounded-full) — как канон
 * CircleIcon с фото. Флаги живут до размонтирования (списки кейуются
 * по id — перемонтирование); photoReady не сбрасывается при смене src:
 * свап ?v=N на живом элементе держит старый кадр до декода нового —
 * инвариант анти-мигания бастера (photoDisplayUrl). loading-атрибут —
 * за вызывающим (ленивая подача рядов — план 035).
 */
export function CirclePhoto({ photoUrl, glyph, className }: CirclePhotoProps): JSX.Element {
  const [photoBroken, setPhotoBroken] = useState(false);
  const [photoReady, setPhotoReady] = useState(false);
  const view = circlePhotoView(photoUrl, { photoBroken, photoReady });

  return (
    <span
      aria-hidden
      className={cn(
        'relative flex h-full w-full items-center justify-center',
        className,
      )}
    >
      {glyph}
      {view.imgSrc !== null && (
        <img
          src={view.imgSrc}
          alt=""
          decoding="async"
          className={cn(
            'absolute inset-0 h-full w-full object-cover transition-opacity',
            view.imgHidden ? 'opacity-0' : 'opacity-100',
          )}
          onLoad={() => setPhotoReady(true)}
          onError={() => setPhotoBroken(true)}
        />
      )}
    </span>
  );
}
