'use client';

import { useState, type JSX, type ReactElement } from 'react';
import { cn } from '@/shared/lib/cn';
import { isPhotoThumbLoaded, markPhotoThumbLoaded, photoThumbKey } from './photo-session-cache';

/** Флаги жизни фото в круге: битое (onError), готовое (onLoad) и номер
 * ретрай-попытки (0/undefined — её не было). */
export type CirclePhotoFlags = {
  readonly photoBroken: boolean;
  readonly photoReady: boolean;
  readonly retryNonce?: number;
};

/** Что показывает круг фото: чистое решение — единственное место гарда
 * пустого пути (WebKit: <img src=""> не стреляет onError — канон
 * UserAvatar), правила «глиф под фото» (глиф-подложка живёт в ожидании
 * и при ошибке, тикет #1423) и хвоста ретрай-нонса (первая ошибка
 * перезапрашивает байты с &r=N поверх версионированного URL — транзиентный
 * сбой сети/429 не прячет живое фото навсегда; второй подряд error —
 * честный фолбэк). Вынесено из компонента, чтобы покрываться
 * юнит-тестами — тестовое окружение фронта исполняет только чистую
 * логику (node, без DOM; прецедент circle-icon-variants). */
export function circlePhotoView(
  photoUrl: string | null | undefined,
  flags: CirclePhotoFlags,
): { readonly imgSrc: string | null; readonly imgHidden: boolean } {
  if (photoUrl == null || photoUrl === '' || flags.photoBroken) {
    return { imgSrc: null, imgHidden: false };
  }
  // Круг — всегда мал (≤96 CSS px): запрашивается thumbnail-вариант выдачи
  // (?size=thumb, ADR 0065 тикет миниатюр), полный объект живёт за тем же
  // стримом без параметра — крупный просмотр, если появится, не проходит
  // через этот примитив.
  const thumbSrc = photoThumbKey(photoUrl);
  const retryNonce = flags.retryNonce ?? 0;
  const imgSrc = retryNonce > 0 ? `${thumbSrc}&r=${retryNonce}` : thumbSrc;
  return { imgSrc, imgHidden: !flags.photoReady };
}

export type CirclePhotoProps = {
  /** Версионированный стриминговый путь выдачи (ADR 0065);
   * null/undefined/'' — только глиф. */
  readonly photoUrl?: string | null;
  /** Глиф-подложка: тип объекта / BoldUser — выбирает вызывающий; живёт
   * и в ожидании байтов, и при битом фото (фолбэк #1275). */
  readonly glyph: ReactElement;
  /** Ленивая подача (#1423/035): ряды списков и лент — 'lazy' (аватары за
   * экраном не соревнуются с данными); hero-круги и фото-слоты — eager
   * (дефолт, атрибут не пишется). lazy обязателен для кругов, чьи близнецы
   * бывают скрыты (display:none img с lazy не запрашивается — крыло шапки
   * под PersistentNavWings, приёмка 22.10.2026). Проводка атрибута —
   * проводка рендера, проверяется экранной приёмкой и e2e (канон
   * node-тестов). */
  readonly loading?: 'lazy';
  /** Классы обёртки-круга (размер поверх родителя). */
  readonly className?: string;
};

/**
 * Содержимое клипающего круга пользовательского фото — примитив канона
 * «глиф под фото + проявление по готовности» (ADR 0065, тикет #1423): пока
 * байты летят (холодный вход, мобильная сеть), круг показывает
 * глиф-подложку вместо пустого фона; готовое фото проявляется на дефолтных
 * transition-токенах дома (--dl-duration-fast/--dl-ease, globals.css @theme).
 * Родитель обязан клипать (overflow-hidden rounded-full) — как канон
 * CircleIcon с фото. Флаги живут до размонтирования (списки кейуются
 * по id — перемонтирование); photoReady не сбрасывается при смене src:
 * свап новой версии (?v=/&r=) на живом элементе держит старый кадр до
 * декода нового — инвариант анти-мигания. Готовность определяется не
 * только onLoad: картинка из браузерного кэша (выдача immutable, аменд
 * 2026-10-20) может завершиться до навески слушателя — ref-колбэк при
 * монтировании честно проверяет complete/naturalWidth, иначе круг навсегда
 * держит глиф при загруженных байтах (живая приёмка 20.10: шапка с фото,
 * hero с глифом). Первая ошибка загрузки даёт один ретрай с нонсом
 * (&r=N): транзиентный сбой (обрыв сети, 429) не прячет живое фото
 * навсегда; ошибка после ретрая — честный глиф (фолбэк #1275).
 * loading-атрибут — за вызывающим (ленивая подача рядов — план 035).
 *
 * Сессионный реестр загруженных фото (photo-session-cache, решение
 * владельца 22.10.2026): перемонтирование круга при навигации (шапка
 * рендерится каждым экраном) больше не проигрывает цикл «глиф → fade-in»
 * для уже показанного фото — свежий экземпляр стартует с photoReady=true
 * по реестру, img виден с первого рендера. Персистентное крыло ПК
 * (PersistentNavWings) снимает сам перемонтаж шапки на десктопе; реестр
 * закрывает мобайл, планшет и все прочие круги.
 */
export function CirclePhoto({ photoUrl, glyph, loading, className }: CirclePhotoProps): JSX.Element {
  const [photoBroken, setPhotoBroken] = useState(false);
  // Старт по реестру: фото уже показывалось в этой жизни страницы — img
  // виден с первого рендера, без кадра с глифом и transition-opacity.
  const [photoReady, setPhotoReady] = useState(
    () => photoUrl != null && photoUrl !== '' && isPhotoThumbLoaded(photoUrl),
  );
  const [retryNonce, setRetryNonce] = useState(0);
  // Ретрай и битость принадлежат одному URL (версионированному): новый
  // photoUrl — новая жизнь загрузки (ошибка старого фото не прячет
  // заменённое до перемонтирования). photoReady при смене src не сбрасывается —
  // инвариант анти-мигания (живой <img> держит старый кадр до декода нового).
  const [prevPhotoUrl, setPrevPhotoUrl] = useState(photoUrl);
  if (prevPhotoUrl !== photoUrl) {
    setPrevPhotoUrl(photoUrl);
    setRetryNonce(0);
    setPhotoBroken(false);
  }
  const view = circlePhotoView(photoUrl, { photoBroken, photoReady, retryNonce });

  // Успех загрузки — и в стейт, и в сессионный реестр: следующий смонтированный
  // круг с этим URL (после навигации) стартует сразу с видимым фото.
  const markLoaded = (): void => {
    if (photoUrl != null && photoUrl !== '') {
      markPhotoThumbLoaded(photoUrl);
    }
    setPhotoReady(true);
  };

  const failOrRetry = (): void => {
    if (retryNonce === 0) {
      setRetryNonce(1);
      return;
    }
    setPhotoBroken(true);
  };

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
          loading={loading}
          ref={(node) => {
            // Кэш-гонка: байты могут приехать до навески onLoad — готовность
            // и провал определяются фактом complete, а не только событием.
            if (node?.complete) {
              if (node.naturalWidth > 0) {
                markLoaded();
              } else {
                failOrRetry();
              }
            }
          }}
          className={cn(
            'absolute inset-0 h-full w-full object-cover transition-opacity',
            view.imgHidden ? 'opacity-0' : 'opacity-100',
          )}
          onLoad={markLoaded}
          onError={failOrRetry}
        />
      )}
    </span>
  );
}
