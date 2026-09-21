# Иконки PWA: процесс обновления (RealFaviconGenerator)

Генераторов в репо больше нет — набор иконок собирается внешним сервисом
[RealFaviconGenerator](https://realfavicongenerator.net) (решение владельца
21.09.2026, тикет #781). Владелец прогоняет мастер через сайт и приносит
результат, агент раскладывает файлы по репо.

## Мастера

| Файл | Что это |
|---|---|
| `source-icon.svg` | скруглённый мастер из Figma («Рентли. Новые экраны сервиса», узел 2358:139864) — дом #2B7FFF, дверной путь white, фон #BCDCFF с прозрачными углами |
| `source-square.svg` | тот же арт полным квадратом (Figma 2353:52637) — **его загружают в RFG** (источник таб-иконок, apple и maskable) |
| `source-icon-fitted.png` | референс вписанного арта: дом вписан в 74% холста и отцентрован (решение владельца от 21.09: macOS 26 маскирует иконки Dock в системный сквиркл — полный арт режется); в RFG не загружается |
| `source-monochrome.svg` | силуэт дома для Android 13+ themed-иконок (механизм monochrome, RFG его не делает) |

## Как обновить набор

1. Загрузить на realfavicongenerator.net **исходный квадратный мастер**
   (`source-square.svg` или его PNG-экспорт) — не вписанный файл: тогда
   таб-фавикон остаётся полным артом, а вписывание дома делается
   платформенными настройками на шаге 2.
2. Настройки платформ:
   - **iOS / apple-touch-icon**: background color `#BCDCFF`, отступы такие,
     чтобы дом в превью был вписан (≤ ~74% ширины, ориентир —
     `source-icon-fitted.png`); итог непрозрачный полный квадрат
     (iOS красит альфу в чёрный);
   - **Android Chrome**: включить 192 и 512; **maskable** — background color
     `#BCDCFF` (лоунчер сам кладёт маску);
   - **Desktop**: по умолчанию (ICO + PNG);
   - **Windows tiles и Safari pinned tab (mask-icon): выключить** — мёртвые
     механизмы, research `docs/research/pwa-icons-canon.md` их запрещает;
   - их `site.webmanifest` и HTML-сниппет **не брать** — манифест у нас свой
     (`app/manifest.ts`), линковку делает Next.js file conventions.
3. Скачать зип и передать агенту.

## Куда что кладёт агент (маппинг, Next.js-набор RFG от 21.09)

| Файл из RFG | Куда в репо |
|---|---|
| `favicon.ico` (16/32/48) | `app/favicon.ico` |
| `apple-icon.png` (180, непрозрачный) | `app/apple-icon.png` |
| `icon1.png` (96, прозрачный) | `app/icon.png` |
| `web-app-manifest-512x512.png` | `public/icons/icon-maskable-512.png` |
| `web-app-manifest-192x192.png` | некуда: в манифесте нет 192-maskable записи — не брать |
| `icon0.svg` (SVG-favicon), `manifest.json` («MyWebSite»), прочее | **не брать** — SVG-favicon запрещён research #780, манифест свой (`app/manifest.ts`) |

Нюанс: RFG в Next.js-наборе отдаёт `web-app-manifest-*` как **непрозрачные
квадраты (purpose maskable)** — они идут только в maskable. Purpose `any`
(`public/icons/icon-192.png`, `icon-512.png` — прозрачные, дом вписан 74%)
RFG не делает: эти два файла в репо постоянные, пока RFG не начнёт отдавать
прозрачный вариант.

`public/icons/icon-monochrome-512.png` RFG не делает — файл в репо, обновляется
только вместе с `source-monochrome.svg`.

## Проверка иконки в Dock (macOS)

Какая иконка куда попадает: Dock-приложение, ставимое Chrome, берёт
манифестные `any` 192/512 (наши прозрачные вписанные); «Добавить в Dock»
в Safari — `app/apple-icon.png`; таб браузера — `favicon.ico`/`app/icon.png`;
Android-лончер — `icon-maskable-512.png`.

Иконка Dock кэшируется macOS (LaunchServices/Dock): установка поверх старой
может показывать прежнюю иконку даже после переустановки. Грабля 21.09
(выглядело как регресс набора «иконка снова обрезается»): в Dock оставалась
старая дофиттинговая иконка из кэша. Перед проверкой новой иконки: закрыть
браузер, снести ВСЕ установленные копии PWA (в т.ч. прежние версии в
`~/Applications/Chrome Apps`), поставить заново — только тогда Dock прочтёт
свежий icns.
