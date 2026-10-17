# 001 — Меню: скролл-лок без перелэйаута документа

- **Status**: DONE (замер CPU×4: холодное открытие 424.9→83.3мс худший кадр,
  тёплое 74.4→32.4, закрытие 50.1→12.6 и 0 провалов; лок фон не скроллит
  колесом и тачем, Esc закрывает, якорь из меню садится в scroll-mt)
- **Commit**: 613729c87
- **Severity**: MEDIUM
- **Category**: Performance (живая проба CPU×4: открытие меню — худший кадр 424.9мс, 8 кадров >32мс; тёплое открытие — longtask 64мс)
- **Estimated scope**: 2 файла (components/mobile-menu.tsx, app/globals.css)

## Problem

Открытие мобильного меню роняет кадры на слабых устройствах. Лок скролла пишется инлайном на корневой скроллер — смена `overflow` на `html` инвалидирует лэйаут всего документа (страница ~13 000px, ~1700 нод):

```tsx
// components/mobile-menu.tsx:78-89 — current
useEffect(() => {
  if (!open && !closing) {
    return;
  }
  const html = document.documentElement;
  html.style.overflow = "hidden";
  html.style.scrollbarGutter = "stable";
  return () => {
    html.style.overflow = "";
    html.style.scrollbarGutter = "";
  };
}, [open, closing]);
```

Инлайновый `overflow="hidden"` заодно сбрасывает базовый `overflow-x: clip` (globals.css:244-247) до `hidden` — мягкое противоречие канону #709. Замер (проба improve-animations 17.10, 375×812, CPU×4): холодное открытие — кадр 424.9мс и два по 117мс; тёплое — longtask 64мс.

## Target

Лок без смены `overflow` корневого скроллера — блокировка жестов свойствами hit-test/композитинга, которые не инвалидируют лэйаут:

```css
/* app/globals.css — target (unlayered, рядом с html body[data-scroll-locked]) */
html.menu-scroll-locked {
  touch-action: none;
  overscroll-behavior: contain;
}
```

```tsx
// components/mobile-menu.tsx — target: эффект лока
useEffect(() => {
  if (!open && !closing) {
    return;
  }
  const html = document.documentElement;
  html.classList.add("menu-scroll-locked");
  // Колесо/трекпад (планшет <1200) — не тач: гасим слушателем.
  const onWheel = (e: WheelEvent) => e.preventDefault();
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      startClose();
    }
  };
  document.addEventListener("wheel", onWheel, { passive: false });
  document.addEventListener("keydown", onKey);
  return () => {
    html.classList.remove("menu-scroll-locked");
    document.removeEventListener("wheel", onWheel);
    document.removeEventListener("keydown", onKey);
  };
}, [open, closing]);
```

Инлайновые `overflow`/`scrollbarGutter` удаляются полностью (скроллбар больше не исчезает — и сдвига на классических скроллбарах нет в принципе).

## Repo conventions to follow

- Точечные unlayered-правила поверх Tailwind-слоёв — `globals.css:296-300` (`html body[data-scroll-locked]`).
- Esc-закрытие и координация с smooth-scroll: `components/smooth-scroll.tsx:49-59` ждёт снятия `html.style.overflow` — НАД слизывать: после этого плана smooth-scroll должен ждать снятия класса `menu-scroll-locked` (см. Steps 3).

## Steps

1. globals.css: добавить unlayered-правило `html.menu-scroll-locked { touch-action: none; overscroll-behavior: contain; }` рядом с блоком `data-scroll-locked`.
2. mobile-menu.tsx: заменить тело эффекта лока (78-89) на класс + wheel/keydown-гварды (Target выше). `startClose` объявлен ниже по файлу — вынести обработчик Esc в ref или объявить `startClose` до эффекта (сейчас он на 91-96: перенести выше эффекта).
3. smooth-scroll.tsx:51-52: условие ожидания `html.style.overflow === "hidden"` заменить на `html.classList.contains("menu-scroll-locked")`.
4. Проверить teardown на desk-переходе (mobile-menu.tsx:63-73): сброс `open/closing` снимает класс через cleanup эффекта — дописать нечего.

## Boundaries

- Do NOT трогать glass/backdrop-filter где бы то ни было.
- Do NOT трогать хореографию шторки (grid-rows 400ms, каскад 45мс, срез 440мс) — только лок.
- Do NOT добавлять зависимости (никакого body-scroll-lock/react-remove-scroll руками).
- Клавиатурный скролл фона (space/стрелки) при открытом меню остаётся возможным — осознанный компромисс, задокументировать в диффе.

## Verification

- **Mechanical**: `npm run lint && npm run typecheck && npm run build` — зелёные.
- **Проба (та же методика, что 17.10)**: 375×812, CPU×4, CDP rAF-дельты: холодное открытие — худший кадр <100мс, тёплое — 0 кадров >32мс; закрытие — без деградации к текущему (худший ~50мс → цель <32).
- **Функционально**: при открытом меню (375) тач-протяжка фона не скроллит страницу; (768) трекпад-скролл не скроллит; пункт меню → шторка закрылась → страница докатилась к якорю (smooth-scroll); Esc закрывает.
- **Feel check**: открытие на телефоне — шторка едет без ступеньки на старте.
- **Done when**: гейт пробы выполнен и функциональные проверки зелёные; иначе — откат Steps 1-3 к текущему инлайновому локу (поведение без регресса) и фиксация отказа в plans/README.md.
