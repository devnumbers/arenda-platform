/**
 * Шрифты офлайн-страницы (макет «Рентли» 40000051:81613, решение владельца
 * 27.10.2026: офлайн-экран рисуется фирменным Onest, а не системным стеком).
 *
 * Страница отдаётся из кэша SW при выключенной сети и не может тянуть ни
 * CSS-бандл, ни next/font, поэтому @font-face продублирован в
 * public/offline.html вручную, а сами файлы прекэшит public/sw.js
 * (OFFLINE_FONT_PATHS, inline-копия этого списка): /fonts отдаётся с
 * max-age=0/must-revalidate, офлайн перевалидация умирает — без прекэша
 * страница рисовалась бы системным фолбэком.
 *
 * Два сабсета — те же, что прелоадит app/layout.tsx ('onest-latin',
 * 'onest-cyrillic'): кириллица покрывает весь русский текст страницы,
 * латиница — «VPN»; ext-сабсеты ей не нужны.
 */
export const OFFLINE_FONT_PATHS: ReadonlyArray<string> = [
  '/fonts/onest-cyrillic.woff2',
  '/fonts/onest-latin.woff2',
];
