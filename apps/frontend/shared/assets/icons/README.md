# Иконки — канонический набор

Канон иконок фронтенда — **SVG из Figma «Рентли. Новые экраны сервиса»**,
утверждённые владельцем 07.09.2026: набор `Icon/Bold/*` + `Icon/R/*`
(96 иконок, 24×24), S-стиль `Icon/S/*` (9 иконок, 16×16, обводка 1.2),
градиентные статусы `Icon/Color/*` (CheckWhite/DangerWhite/GoodWhite —
цвета запечены по макету) и `Notification Dot`. Всё лежит в этой папке
и экспортируется через барель `index.ts`.

## Правила (обязательные)

1. **Иконки не рисуются и не генерируются сами** — ни инлайн-SVG в tsx, ни
   «похожая по памяти», ни правка геометрии руками. Нужной иконки нет в
   наборе → экспортируй ноду из Figma (как в таблице ниже) или спроси
   владельца.
2. Импорт — только из бареля `@/shared/assets/icons` (PascalCase-компоненты,
   svgr). Прямые импорты `*.svg` вне бареля запрещены.
3. Цвет — `currentColor` (иконка красится текстом). Запечённые цвета —
   только designer-baked исключения: `bold-user` (#D3D7D9, плейсхолдер),
   градиентные статусы `status-icon-check/danger/good/warning/info`,
   `notification-dot`; легаси `badge-*`, `checkbox-*`/`radio-*` (вне
   канона, см. ниже).
4. Именование файлов — kebab-case; Bold-стиль — префикс `bold-`,
   контурный (`Icon/R/*`) — без префикса (`check`, `calendar`, `phone`),
   S-стиль (`Icon/S/*`, 16×16) — суффикс `-small` (`clock-small`,
   `archive-small`).
5. Формат — 24×24 `viewBox="0 0 24 24"` (S-стиль — 16×16), как в экспорте
   Figma. Малый размер на экране задаёт потребитель (классы/w/h), отдельных
   «уменьшенных» файлов-дублей не заводить.

## Таблица канона — Bold/R (96)

| Экспорт | Файл | Figma | node | Статус 07.09.2026 |
|---|---|---|---|---|
| `BoldStar` | `bold-star.svg` | `Bold/Star` | 879:17720 | новая |
| `BoldWallet` | `bold-wallet.svg` | `Bold/Wallet` | 879:17713 | новая |
| `BoldObjects` | `bold-objects.svg` | `Bold/Objects` | 208:2994 | совпадала |
| `BoldBuild` | `bold-build.svg` | `Bold/Build` | 782:12777 | заменена |
| `BoldShield` | `bold-shield.svg` | `Bold/Shield` | 773:12015 | совпадала |
| `BoldFridge` | `bold-fridge.svg` | `Bold/Fridge` | 776:12101 | заменена |
| `BoldHammer` | `bold-hammer.svg` | `Bold/Hammer` | 781:12208 | заменена |
| `BoldKey` | `bold-key.svg` | `Bold/Key` | 189:933 | заменена |
| `BoldBill` | `bold-bill.svg` | `Bold/Bill` | 773:12023 | совпадала |
| `BoldPrinter` | `bold-printer.svg` | `Bold/Printer` | 777:12107 | совпадала |
| `BoldHandCoin` | `bold-hand-coin.svg` | `Bold/HandCoin` | 776:12040 | заменена |
| `BoldCalculator` | `bold-calculator.svg` | `Bold/Calculator` | 189:949 | совпадала |
| `BoldMoneyLock` | `bold-money-lock.svg` | `Bold/MoneyLock` | 776:12041 | совпадала |
| `BoldSecurity` | `bold-security.svg` | `Bold/Security` | 777:12116 | заменена |
| `BoldCar` | `bold-car.svg` | `Bold/Car` | 766:10601 | заменена |
| `BoldLight` | `bold-light.svg` | `Bold/Light` | 770:11982 | совпадала |
| `BoldPercent` | `bold-percent.svg` | `Bold/Percent` | 776:12066 | заменена |
| `BoldPerson` | `bold-person.svg` | `Bold/Person` | 778:12123 | совпадала |
| `BoldHeartBroken` | `bold-heart-broken.svg` | `Bold/HeartBroken` | 781:12230 | заменена |
| `BoldWater` | `bold-water.svg` | `Bold/Water` | 770:11987 | заменена |
| `BoldWrench` | `bold-wrench.svg` | `Bold/Wrench` | 770:11298 | заменена |
| `BoldMegaphone` | `bold-megaphone.svg` | `Bold/Megaphone` | 779:12139 | заменена |
| `BoldCoins` | `bold-coins.svg` | `Bold/Coins` | 781:12252 | заменена |
| `BoldPipeline` | `bold-pipeline.svg` | `Bold/Pipeline` | 771:11995 | совпадала |
| `BoldPaintRoller` | `bold-paint-roller.svg` | `Bold/PaintRoller` | 766:10500 | заменена |
| `BoldCourt` | `bold-court.svg` | `Bold/Court` | 779:12165 | заменена |
| `BoldClock` | `bold-clock.svg` | `Bold/Clock` | 781:12221 | совпадала |
| `BoldTemperature` | `bold-temperature.svg` | `Bold/Temperature` | 771:11998 | заменена |
| `BoldBroom` | `bold-broom.svg` | `Bold/Broom` | 776:12082 | заменена |
| `BoldWarning` | `bold-warning.svg` | `Bold/Warning` | 766:10625 | заменена |
| `BoldStamp` | `bold-stamp.svg` | `Bold/Stamp` | 782:13015 | заменена |
| `BoldGas` | `bold-gas.svg` | `Bold/Gas` | 772:12006 | совпадала |
| `BoldBox` | `bold-box.svg` | `Bold/Box` | 776:12092 | совпадала |
| `BoldCase` | `bold-case.svg` | `Bold/Case` | 766:10597 | новая |
| `BoldOther` | `bold-other.svg` | `Bold/Other` | 781:12291 | совпадала |
| `BoldTrash` | `bold-trash.svg` | `Bold/Trash` | 766:10610 | совпадала |
| `BoldInternet` | `bold-internet.svg` | `Bold/Internet` | 766:10510 | совпадала |
| `BoldFence` | `bold-fence.svg` | `Bold/Fence` | 782:12789 | заменена |
| `BoldHome` | `bold-home.svg` | `Bold/Home` | 189:931 | заменена |
| `BoldTv` | `bold-tv.svg` | `Bold/Tv` | 776:12075 | совпадала |
| `BoldBell` | `bold-bell.svg` | `Bold/Bell` | 779:12175 | совпадала |
| `BoldCredit` | `bold-credit.svg` | `Bold/Credit` | 773:12012 | совпадала |
| `BoldSofa` | `bold-sofa.svg` | `Bold/Sofa` | 289:3433 | совпадала |
| `BoldLeaf` | `bold-leaf.svg` | `Bold/Leaf` | 779:12184 | заменена |
| `BoldArchive` | `bold-archive.svg` | `Bold/Archive` | 1858:100932 | новая |
| `Phone` | `phone.svg` | `R/Phone` | 1804:105303 | новая |
| `Computer` | `computer.svg` | `R/Computer` | 1804:105311 | новая |
| `Archive` | `archive.svg` | `R/Archive` | 189:818 | новая |
| `Kebab` + `VerticalMenu` | `kebab.svg`, `vertical-menu.svg` | `R/MoreVertical` | 185:175 | совпадала (оба файла) |
| `Pin` | `pin.svg` | `R/Pin` | 501:8839 | новая |
| `Sync` | `sync.svg` | `R/Sync` | 1804:105034 | новая |
| `PinOff` | `pin-off.svg` | `R/PinOff` | 890:30954 | новая |
| `Play` | `play.svg` | `R/Play` | 851:15576 | новая |
| `Block` | `block.svg` | `R/Block` | 1804:108181 | новая |
| `Undo` | `undo.svg` | `R/Undo` | 1883:71902 | новая |
| `SortingSmallBig` | `sorting-small-big.svg` | `R/SortingSmallBig` | 418:4608 | новая |
| `Checkmark` | `checkmark.svg` | `R/Checkmark` | 1535:77389 | новая |
| `ChangeVertical` | `change-vertical.svg` | `R/ChangeVertical` | 858:20998 | новая |
| `SortingBigSmall` | `sorting-big-small.svg` | `R/SortingBigSmall` | 418:4607 | новая |
| `ArrowLeft` | `arrow-left.svg` | `R/ArrowLeft` | 529:3934 | заменена |
| `Exit` | `exit.svg` | `R/Exit` | 472:4952 | новая |
| `TeamAdd` | `team-add.svg` | `R/TeamAdd` | 1804:108296 | новая |
| `Cancel` | `cancel.svg` | `R/Cancel` | 556:10056 | новая |
| `MenuLines` | `menu-lines.svg` | `R/Menu` | 185:210 | совпадала |
| `HomeMain` | `home-main.svg` | `R/HomeMain` | 501:8460 | новая |
| `Setting` | `setting.svg` | `R/Setting` | 1740:100244 | новая |
| `Wallet` | `wallet.svg` | `R/Wallet` | 550:8808 | новая |
| `TrashBin` | `trash-bin.svg` | `R/TrashBin` | 189:853 | заменена (канонный глиф; легаси `trash.svg`/`trashbin.svg` удалены) |
| `TimeHistory` | `time-history.svg` | `R/TimeHistory` | 627:7557 | заменена |
| `CheckmarkCircle` | `checkmark-circle.svg` | `R/CheckmarkCircle` | 1644:94847 | новая |
| `UserCircle` | `user-circle.svg` | `R/UserCircle` | 1652:82357 | новая |
| `NotificationSettings` | `notification-settings.svg` | `R/NotificationSettings` | 472:4940 | новая |
| `Support` | `support.svg` | `R/Support` | 472:4939 | заменена |
| `AccountSetting` | `account-setting.svg` | `R/AccountSetting` | 472:4958 | новая |
| `Search` | `search.svg` | `R/Search` | 185:185 | новая |
| `StarOff` | `star-off.svg` | `R/StarOff` | 584:13000 | новая |
| `StarOutline` | `star-outline.svg` | `R/Star` | 584:12999 | совпадала |
| `Filter` | `filter.svg` | `R/Filter` | 374:1948 | заменена |
| `Calendar` | `calendar.svg` | `R/Calendar` | 616:6651 | новая |
| `Add` | `add.svg` | `R/Add` | 189:2436 | новая |
| `Edit` | `edit.svg` | `R/Edit` | 189:2390 | новая |
| `Pause` | `pause.svg` | `R/Pause` | 617:6814 | новая |
| `Move` | `move.svg` | `R/Move` | 693:5669 | новая |
| `Check` | `check.svg` | `R/Check` | 615:6414 | новая |
| `Minus` | `minus.svg` | `R/Minus` | 1858:105685 | новая |
| `SmallArrowRight` | `small-arrow-right.svg` | `R/SmallArrowRight` | 165:307 | новая |
| `SmallArrowDown` + `ArrowDown` | `small-arrow-down.svg`, `arrow-down.svg` | `R/SmallArrowDown` | 671:7320 | совпадала (оба файла) |
| `BoldUser` | `bold-user.svg` | `Bold/User` | 189:2047 | совпадала |
| `SmallArrowUp` | `small-arrow-up.svg` | `R/SmallArrowUp` | 671:7463 | новая |
| `Change` | `change.svg` | `R/Change` | 1134:49309 | новая |
| `PaintBrush` | `paint-brush.svg` | `R/PaintBrush` | 189:800 | новая |
| `Download` | `download.svg` | `R/Download` | 189:836 | новая |
| `Key` | `key.svg` | `R/Key` | 119:1104 | новая |
| `Team` | `team.svg` | `R/Team` | 472:5276 | новая |
| `Copy` | `copy.svg` | `R/Copy` | 1323:59291 | заменена |
| `Info` | `info.svg` | `R/Info` | 1296:48308 | новая |
| `CreditCard` | `credit-card.svg` | `R/CreditCard` | 1917:72300 | новая (12.09.2026, экран «Тариф» #620) |
| `Objects` | `objects.svg` | `R/Objects` | 1967:86377 | новая (17.09.2026, хаб «Совместный доступ» #696) |

## Таблица канона — S-стиль (9, 16×16 обводка 1.2)

| Экспорт | Файл | Figma | node | Статус 07.09.2026 |
|---|---|---|---|---|
| `ClockSmall` | `clock-small.svg` | `S/Clock` | 594:13394 | обновлена |
| `HomeMainSmall` | `home-main-small.svg` | `S/HomeMain` | 1726:86759 | обновлена |
| `Repeat` | `repeat.svg` | `S/Repeat` | 284:1108 | обновлена |
| `Star` | `star.svg` | `S/Star` | 890:28175 | обновлена |
| `ArchiveSmall` | `archive-small.svg` | `S/Archive` | 1603:90962 | новая |
| `CalendarSmall` | `calendar-small.svg` | `S/Calendar` | 1550:91499 | новая |
| `PaintBrushSmall` | `paint-brush-small.svg` | `S/PaintBrush` | 1603:90961 | новая |
| `KeySmall` | `key-small.svg` | `S/Key` | 1603:91160 | новая |
| `PinSmall` | `pin-small.svg` | `S/Pin` | 1603:91203 | новая |
| `InfoSmall` | `info-small.svg` | `S/Info` | 1912:71626 | новая (12.09.2026, экран «Тариф» #620) |
| `LockSmall` | `lock-small.svg` | `S/Lock` | 2036:83931 | новая (17.09.2026, «Ваши участники» #697) |
| `EditSmall` | `edit-small.svg` | `S/Edit` | 1961:52805 | новая (17.09.2026, страница участника #698) |
| `EyeSmall` | `eye-small.svg` | `S/Eye` | 1961:52816 | новая (17.09.2026, страница участника #698) |

## Таблица канона — статусы и бейджи (запечённые цвета)

| Экспорт | Файл | Figma | node | Статус 07.09.2026 |
|---|---|---|---|---|
| `StatusIconCheck` | `status-icon-check.svg` | `Color/CheckWhite` | 671:6754 | совпадала |
| `StatusIconDanger` | `status-icon-danger.svg` | `Color/DangerWhite` | 671:6751 | заменена |
| `StatusIconGood` | `status-icon-good.svg` | `Color/GoodWhite` | 671:6753 | совпадала |
| `NotificationDot` | `notification-dot.svg` | `Notification Dot` | 651:6759 | новая |
| `CheckNoneLine` | `check-none-line.svg` | `Color/CheckNoneLine` | 1386:66530 | новая 12.09.2026 (#621, успех возобновления) |

`NumbersAlerts` (1652:82624) — счётчик с числом: это текстовый компонент,
а не иконка (число живое) — делается design-компонентом по запросу, в
канон иконок не входит. `status-icon-warning`/`status-icon-info` — та же
семейство `Icon/Color/*`, ноды владельцем не переданы, файлы оставлены
как есть.

## Вне канона

Остаются (решение владельца 07.09: чекбоксы/радио и лого — «всё так и
должно быть»):

- контролы выбора (цвета запечены): `checkbox-true`, `checkbox-false`,
  `radio-true`, `radio-false`;
- легаси-тосты, не вызываются: `badge-danger`, `badge-good`, `badge-info`,
  `badge-warning`;
- статусы без нод (семейство `Icon/Color/*`, ждут экспорта):
  `status-icon-warning`, `status-icon-info`;
- лого: `rently-logo`.

**Красные иконки в коде (`text-error`) — маркеры замен от 07.09**: места,
где стояли легаси-иконки без канонного аналога, временно заняты каноном
«по смыслу»; владелец поставит правильные иконки, после чего маркеры
снимаются. Легаси-файлы этих мест удалены (стрелки месяцев, chevron-up,
home-add, loading, nav/bottom-навигация, star-colored, статус-бейджи
status-good/danger/door/warning).
