/** Decode-реестр статических иллюстраций (карта #1587, тикет #1601,
 * решение владельца 23.10): src → выполненный decode() для WebP-иллюстраций
 * пустых состояний. Диагноз тикета: экраны кабинета перемонтируются при
 * клиентской навигации, и иллюстрация «Объектов»/«Платежей» на возврате
 * назад повторяет цикл «пустой бокс → загрузка → decode → paint» — видимое
 * «дёрганье» иконки, даже когда байты уже в кешах (SW static-assets-v1
 * убирает сеть под вспышкой, но не саму вспышку — её decode-фазу закрывает
 * этот реестр).
 *
 * Механика: слот помечает показ по факту загрузки реального <img> —
 * currentSrc это точный URL, который srcset next/image уже выбрал под DPR;
 * ключ реестра совпадает с рендером по построению, предекод не может
 * разминуться с ним и создать двойной трафик мимо кешей. По пометке
 * создаётся offscreen-копия `new Image()` с тем же src и вызывается
 * decode(): живой элемент в реестре удерживает ресурс (байты в
 * memory-кеше) и его декодированный растр — повторное монтирование <img>
 * того же src рисуется из decode-кеша браузера первым кадром, без фазы
 * пустого бокса. Первый показ реестр не трогает: пометка случается уже
 * после него, поведение прежнее.
 *
 * Пометка — пара `rememberIllustrationRef` + `onIllustrationLoad`, урок
 * CirclePhoto (CODING_STANDARDS «User photos»): кеш immutable-выдачи может
 * завершить загрузку до навески слушателя, поэтому ref-проверка complete
 * ловит готовый кадр (SSR-гидратация, кеш-хит до mount), а onLoad —
 * загрузившийся после. Без пары cold-entry-показ не запоминался бы, и
 * возврат назад мигал бы, как до тикета.
 *
 * Сессия — модульное состояние живёт вместе с бандлом, как у
 * photo-session-cache: SPA-навигации его не сбрасывают, полная
 * перезагрузка начинает с нуля — и это честно: байты при этом приходят из
 * HTTP-кеша (immutable, год) и SW static-assets-v1. Провал decode (битый
 * или оборванный ресурс) отметку снимает — следующий показ попробует
 * снова. Объём ограничен каноном: иллюстрации пустых состояний
 * (~десяток WebP 64–256px, §7) и unoptimized-иконки категорий
 * уведомлений — memory-цена копеечная. Решения 22.10.2026 (priority на
 * всех пустых, суперсэмплинг sizes=2x) не затрагиваются. */

import type { SyntheticEvent } from 'react';

const decodedIllustrations = new Map<string, HTMLImageElement>();

/** Offscreen-копия показа: пока жива в реестре — браузер держит
 * загруженный ресурс и его декодированный растр. */
function rememberDecodedCopy(src: string): void {
  if (src === '' || decodedIllustrations.has(src)) return;
  const element = new Image();
  element.src = src;
  decodedIllustrations.set(src, element);
  // Битый/оборванный ресурс — отметку снимаем: навсегда битая иллюстрация
  // не должна навсегда числиться декодированной, следующий показ создаст
  // копию заново. Сравнение по элементу — за время полёта decode запись
  // могла смениться.
  element.decode().catch(() => {
    if (decodedIllustrations.get(src) === element) {
      decodedIllustrations.delete(src);
    }
  });
}

/** Ref-слота иллюстрации: готовый на монтировании кадр (SSR-гидратация,
 * кеш-хит раньше слушателя) помечается сразу — onLoad для него не придёт. */
export function rememberIllustrationRef(element: HTMLImageElement | null): void {
  if (element === null || !element.complete) return;
  rememberDecodedCopy(element.currentSrc);
}

/** onLoad-слота иллюстрации: помечает показ по currentSrc — точному URL,
 * выбранному srcset'ом под DPR. Пустой currentSrc (маловероятный у
 * загруженного img) молча игнорируется. */
export function onIllustrationLoad(event: SyntheticEvent<HTMLImageElement>): void {
  rememberDecodedCopy(event.currentTarget.currentSrc);
}
