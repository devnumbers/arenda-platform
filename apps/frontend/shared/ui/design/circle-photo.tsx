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
  photoBlur: string | null | undefined,
  flags: CirclePhotoFlags,
): {
  readonly imgSrc: string | null;
  readonly imgHidden: boolean;
  /** Blur-фаза активна: кадр из DTO красится поверх глифа, пока thumb не
   * готов (blur-up, решение #1594). Кеш-попадание (photoReady с первого
   * рендера), битое фото и отсутствие кадра фазу не дают. */
  readonly blurActive: boolean;
} {
  if (photoUrl == null || photoUrl === '' || flags.photoBroken) {
    return { imgSrc: null, imgHidden: false, blurActive: false };
  }
  // Круг — всегда мал (≤96 CSS px): запрашивается thumbnail-вариант выдачи
  // (?size=thumb, ADR 0065 тикет миниатюр), полный объект живёт за тем же
  // стримом без параметра — крупный просмотр, если появится, не проходит
  // через этот примитив.
  const thumbSrc = photoThumbKey(photoUrl);
  const retryNonce = flags.retryNonce ?? 0;
  const imgSrc = retryNonce > 0 ? `${thumbSrc}&r=${retryNonce}` : thumbSrc;
  const imgHidden = !flags.photoReady;
  // Кадр уже в DTO (base64, #1603) — рисуется с первого кадра, без
  // ожидания; photoReady=true (кеш-попадание или готовый thumb) гасит фазу.
  const blurActive = imgHidden && photoBlur != null && photoBlur !== '';
  return { imgSrc, imgHidden, blurActive };
}

/** src blur-слоя из кадра DTO: бэк (#1603) едет голым base64 jpeg —
 * оборачивается в data-URL; готовый data-URL (на будущее) проходит как
 * есть. null/'' — кадра нет. Чистая функция — канон node-тестов. */
export function photoBlurSrc(photoBlur: string | null | undefined): string | null {
  if (photoBlur == null || photoBlur === '') {
    return null;
  }
  return photoBlur.startsWith('data:')
    ? photoBlur
    : `data:image/jpeg;base64,${photoBlur}`;
}

export type CirclePhotoProps = {
  /** Версионированный стриминговый путь выдачи (ADR 0065);
   * null/undefined/'' — только глиф. */
  readonly photoUrl?: string | null;
  /** Blur-кадр первого показа (ADR 0065 аменд, решение #1594): готовый
   * base64 16px jpeg из DTO (#1603). Рисуется поверх глифа с первого кадра,
   * пока thumb не готов; null — деградация к «глиф → fade» (старые фото,
   * webp). */
  readonly photoBlur?: string | null;
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
 *
 * Blur-up первого показа (решение владельца 25.10.2026, #1594/#1604):
 * пока thumb не готов, поверх глифа с первого кадра красится blur-кадр
 * из DTO (base64 приехал вместе с photoUrl — ожидания нет); готовый
 * thumb вытесняет его существующим fade 250ms на дефолтных токенах —
 * blur гаснет тем же opacity-only переходом (появление — без анимации).
 * Кеш-попадание по реестру (photoReady с первого рендера) фазу пропускает
 * целиком — blur не красится никогда (анти-мигание); после полной
 * перезагрузки (реестр пуст) blur честно показывается, даже когда байты
 * едут из immutable HTTP-кеша. photoBlur NULL — деградация к сегодняшнему
 * «глиф → fade». Ошибка загрузки при активной фазе — чистый фолбэк:
 * blur снимается вместе с thumb. Скелетона по-прежнему нет (канон #1423) —
 * blur контентный, не скелетон.
 */
export function CirclePhoto({
  photoUrl,
  photoBlur,
  glyph,
  loading,
  className,
}: CirclePhotoProps): JSX.Element {
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
  // Слой blur живёт с момента первого показа фазы: когда photoReady гасит
  // blurActive, слой остаётся смонтированным и уходит в opacity-0 — fade
  // «blur → thumb» успевает отыграться (мгновенный демонтаж фазу рвал бы).
  const [blurShown, setBlurShown] = useState(false);
  if (prevPhotoUrl !== photoUrl) {
    setPrevPhotoUrl(photoUrl);
    setRetryNonce(0);
    setPhotoBroken(false);
    setBlurShown(false);
  }
  const view = circlePhotoView(photoUrl, photoBlur, { photoBroken, photoReady, retryNonce });
  if (view.blurActive && !blurShown) {
    setBlurShown(true);
  }

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
      {blurShown && view.imgSrc !== null && (
        <img
          src={photoBlurSrc(photoBlur) ?? undefined}
          alt=""
          decoding="async"
          data-blur-layer
          // Кадр растянут на запас кернела CSS-размытия за край круга —
          // против бахромы по ободу клипа; прод-дефолт запаса — 0 (решение
          // владельца 27.10.2026, #1604: бахрома не замечена, растяжки нет).
          // Размер бокса явный: replaced-элемент с auto-шириной взял бы
          // интринсик 16px и лёг точкой в углу. max-w-none ОБЯЗАТЕЛЕН:
          // preflight img{max-width:100%} душил ширину до родителя — слой
          // ужимался влево, и фаза «съезжала» при появлении thumb.
          className={cn(
            'absolute inset-[calc(0px_-_var(--photo-blur-bleed,0px))] h-[calc(100%_+_2*var(--photo-blur-bleed,0px))] w-[calc(100%_+_2*var(--photo-blur-bleed,0px))] max-w-none object-cover blur-[var(--photo-blur-radius,6px)] transition-opacity',
            view.blurActive ? 'opacity-100' : 'opacity-0',
          )}
        />
      )}
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
