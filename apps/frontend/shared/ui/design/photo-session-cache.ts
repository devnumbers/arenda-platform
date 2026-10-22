/** Сессионный реестр загруженных фото (решение владельца 22.10.2026 —
 * анти-мигание аватара): thumb-ключи фото, уже показанные в этой жизни
 * страницы (до первой полной перезагрузки). Модульное состояние живёт
 * вместе с бандлом: SPA-навигации его не трогают, полная перезагрузка
 * начинает с нуля — и это честно: байты при этом приходят из HTTP-кеша
 * (immutable, ADR 0065), но перемонтированный компонент о них не знает.
 *
 * Зачем: TopNav (и круг фото в крыле) рендерится каждым экраном — при
 * каждой навигации CirclePhoto монтируется заново и без реестра
 * проигрывает цикл «глиф → прозрачный img → fade-in», даже когда байты
 * уже в браузерном кеше. Реестр даёт свежесмонтированному кругу старт
 * photoReady=true: кадр с глифом и transition-opacity не возникают,
 * img показывается с первого рендера. Персистентное крыло ПК
 * (PersistentNavWings) снимает сам перемонтаж на десктопе; реестр
 * закрывает мобайл/планшет и все прочие CirclePhoto (hero, строки).
 *
 * Ключ — версионированный URL с size=thumb, как в circlePhotoView:
 * смена фото = новый ключ (?v= — хеш ключа хранилища), реестр не держит
 * устаревший кадр. Битые фото не помечаются — ретрай и фолбэк живут в
 * CirclePhoto. Чистые функции — канон node-тестов (прецедент
 * circle-photo.test.ts); проводка в компонент — экранная приёмка. */

/** thumb-ключ фото: версионированный URL + size=thumb — ровно то, что
 * CirclePhoto кладёт в <img src> до ретрай-нонса. Единственный источник
 * правила и для circlePhotoView, и для реестра. */
export function photoThumbKey(photoUrl: string): string {
  return `${photoUrl}${photoUrl.includes('?') ? '&' : '?'}size=thumb`;
}

const loadedThumbs = new Set<string>();

/** Фото уже показывалось в этой жизни страницы — круг может стартовать
 * с видимым img (без фазы глифа и fade-in). */
export function isPhotoThumbLoaded(photoUrl: string): boolean {
  return loadedThumbs.has(photoThumbKey(photoUrl));
}

/** Отметить фото загруженным (onLoad / ref-проверка complete в CirclePhoto). */
export function markPhotoThumbLoaded(photoUrl: string): void {
  loadedThumbs.add(photoThumbKey(photoUrl));
}
