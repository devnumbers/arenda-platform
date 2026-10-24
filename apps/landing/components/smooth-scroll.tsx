"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

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
//
// Кросс-роут (решение владельца 24.10): с юрстраниц (/privacy, /terms)
// секции главной недоступны — прежний клик только менял хеш в URL и не
// возвращал на лендинг. Нет цели на странице → роутер ведёт на "/"+якорь
// (клиентская замена контента), затем тот же нативный докат к появившейся
// секции. «Поддержка» (#support) не затронута: футер есть на каждой
// странице (site)/layout, её якорь докатит к локальному футеру — кнопка
// «Задать вопрос» там же. Пределы ожидания в мс от клика: секцию ждём до
// SECTION_WAIT_MS (замена контента ещё идёт), разблокировку лока —
// до UNLOCK_LIMIT_MS; после предела отдаёмся случаю — лок висеть не
// должен, а пропавшая секция докатной цели и не обещала.
const UNLOCK_LIMIT_MS = 700;
const SECTION_WAIT_MS = 2000;

export function SmoothScroll() {
  const router = useRouter();
  useEffect(() => {
    let cancelled = false;
    // Докат: цель на странице и скролл разблокированы — scrollIntoView
    // нативным движком; иначе — rAF-цикл до пределов ожидания (петля
    // гасится unmount'ом — runner переживать смену роутера не обязан).
    const scrollIntoViewWhenReady = (href: string, since: number) => {
      const html = document.documentElement;
      const step = () => {
        if (cancelled) {
          return;
        }
        const elapsed = performance.now() - since;
        const target = document.querySelector(href);
        if (
          (!target && elapsed < SECTION_WAIT_MS) ||
          (target && html.classList.contains("menu-scroll-locked") && elapsed < UNLOCK_LIMIT_MS)
        ) {
          requestAnimationFrame(step);
          return;
        }
        if (!target) {
          return;
        }
        target.scrollIntoView({ behavior: "smooth", block: "start" });
      };
      step();
    };

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
      e.preventDefault();
      if (document.querySelector(href)) {
        // Секция на этой странице: как прежде — свой pushState и докат.
        history.pushState(null, "", href);
        scrollIntoViewWhenReady(href, performance.now());
        return;
      }
      // Кросс-роут: навигацию ведёт роутер ("#contact" на юрстранице —
      // к секции главной), докат запускается сразу и догоняет контент.
      // Мёртвый якорь тоже попадает сюда — на главную это софт-переход
      // на тот же URL: после SECTION_WAIT_MS петля угасает без доката.
      router.push("/" + href);
      scrollIntoViewWhenReady(href, performance.now());
    };
    document.addEventListener("click", onClick);
    return () => {
      cancelled = true;
      document.removeEventListener("click", onClick);
    };
  }, [router]);

  return null;
}
