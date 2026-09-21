# Иконки PWA: процесс обновления (RealFaviconGenerator)

Генераторов в репо больше нет — набор иконок собирается внешним сервисом
[RealFaviconGenerator](https://realfavicongenerator.net) (решение владельца
21.09.2026, тикет #781). Владелец прогоняет мастер через сайт и приносит
результат, агент раскладывает файлы по репо.

## Мастера

| Файл | Что это |
|---|---|
| `source-icon.svg` | скруглённый мастер из Figma («Рентли. Новые экраны сервиса», узел 2358:139864) — дом #2B7FFF, дверной путь white, фон #BCDCFF с прозрачными углами |
| `source-square.svg` | тот же арт полным квадратом (Figma 2353:52637) — источник maskable/apple |
| `source-icon-fitted.png` | **мастер для загрузки в RFG**: из `source-icon.svg`, дом вписан в 74% холста и отцентрован (решение владельца от 21.09: macOS 26 маскирует иконки Dock в системный сквиркл — полный арт режется) |
| `source-monochrome.svg` | силуэт дома для Android 13+ themed-иконок (механизм monochrome, RFG его не делает) |

## Как обновить набор

1. Загрузить `source-icon-fitted.png` на realfavicongenerator.net.
2. Настройки платформ:
   - **iOS / apple-touch-icon**: background color `#BCDCFF`, margin 0 —
     получается непрозрачный полный квадрат (iOS красит альфу в чёрный);
   - **Android Chrome**: включить 192 и 512; **maskable** — background color
     `#BCDCFF` (лоунчер сам кладёт маску);
   - **Desktop**: по умолчанию (ICO + PNG);
   - **Windows tiles и Safari pinned tab (mask-icon): выключить** — мёртвые
     механизмы, research `docs/research/pwa-icons-canon.md` их запрещает;
   - их `site.webmanifest` и HTML-сниппет **не брать** — манифест у нас свой
     (`app/manifest.ts`), линковку делает Next.js file conventions.
3. Скачать зип и передать агенту.

## Куда что кладёт агент (маппинг)

| Файл из RFG | Куда в репо |
|---|---|
| `android-chrome-192x192.png` | `public/icons/icon-192.png` |
| `android-chrome-512x512.png` | `public/icons/icon-512.png` и `app/icon.png` |
| `android-chrome-*-maskable*.png` (512) | `public/icons/icon-maskable-512.png` |
| `apple-touch-icon.png` | `app/apple-icon.png` |
| `favicon.ico` | `app/favicon.ico` |
| mstile-*, browserconfig.xml, safari-pinned-tab.svg, site.webmanifest | **не брать** |

`public/icons/icon-monochrome-512.png` RFG не делает — файл в репо, обновляется
только вместе с `source-monochrome.svg`.
