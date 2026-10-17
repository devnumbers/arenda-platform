"use client";

import { useEffect } from "react";

// Плавный докат к секциям по якорным ссылкам (#1520, решение владельца
// 17.10). CSS scroll-behavior: smooth на html не годится: он анимирует и
// программный скролл Next при смене роута — переход на terms улетал
// наверх на глазах, поэтому smooth точечный: нативный движок
// scrollIntoView без кастомной физики, полёт прерывается вводом на любом
// кадре; scroll-mt-офсеты секций scrollIntoView уважает.
//
// Делегат на document (пузырь) — один хук на все якоря (хедер, мобильное
// меню, футер). Модификаторы и средняя кнопка остаются нативными (открыть
// в новой вкладке). Мобильное меню держит html overflow:hidden всю шторку
// закрытия (440мс) — на локе скролл мёртв, поэтому старт откладывается до
// разблокировки: пункт меню закрывается, потом страница докатывается
// (последовательные движения вместо конкурирующих).
const UNLOCK_LIMIT_MS = 700;

export function SmoothScroll() {
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (
        e.defaultPrevented ||
        e.button !== 0 ||
        e.metaKey ||
        e.ctrlKey ||
        e.shiftKey ||
        e.altKey
      ) {
        return;
      }
      const link = (e.target as HTMLElement | null)?.closest('a[href^="#"]');
      if (!link) {
        return;
      }
      const href = link.getAttribute("href");
      if (!href || href === "#") {
        return;
      }
      const target = document.querySelector(href);
      if (!target) {
        return;
      }
      e.preventDefault();
      history.pushState(null, "", href);
      const html = document.documentElement;
      const t0 = performance.now();
      const scroll = () => {
        if (
          html.style.overflow === "hidden" &&
          performance.now() - t0 < UNLOCK_LIMIT_MS
        ) {
          requestAnimationFrame(scroll);
          return;
        }
        target.scrollIntoView({ behavior: "smooth", block: "start" });
      };
      scroll();
    };
    document.addEventListener("click", onClick);
    return () => document.removeEventListener("click", onClick);
  }, []);

  return null;
}
