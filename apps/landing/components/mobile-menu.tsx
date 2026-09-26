"use client";

import { useState, type ReactNode } from "react";
import Link from "next/link";
import { BurgerIcon, CloseIcon } from "./icons";
import { Logo } from "./logo";
import type { NavItem } from "@/lib/nav";

// Мобильный/планшетный хедер (<1200px) — макет 2859-4421: плавающая белая
// плашка radius-24 с тенью 0 8 12 rgba(0,0,0,.12); при открытии меню строка
// хедера получает border-b #ebebeb, под ней панель пунктов 52px (16/20,
// px-24, radius-12, pressed → surface). Раскрытие — плавное (grid-rows).
export function MobileMenu({
  nav,
  action,
}: {
  nav: NavItem[];
  action: ReactNode;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="rounded-3xl bg-white shadow-[0_4px_20px_rgba(0,0,0,0)] transition-shadow duration-300 group-data-scrolled:shadow-[0_4px_20px_rgba(0,0,0,0.08)]">
      <div
        className={`flex h-16 items-center pl-4 pr-2.5 transition-colors duration-300 ${
          open ? "border-b border-line" : ""
        }`}
      >
        <Link
          href="/"
          aria-label="Рентли — на главную"
          className="flex min-w-0 flex-1 items-center"
        >
          <Logo className="h-8 w-auto" />
        </Link>
        <div className="flex items-center gap-1 py-2.5 pl-2.5">
          {action}
          <button
            type="button"
            onClick={() => setOpen((v) => !v)}
            aria-expanded={open}
            aria-controls="mobile-menu"
            aria-label={open ? "Закрыть меню" : "Открыть меню"}
            className="flex h-11 w-11 items-center justify-center rounded-xl bg-surface transition-colors duration-200 outline-none hover:bg-surface-hover active:bg-surface-active focus-visible:ring-2 focus-visible:ring-primary/40"
          >
            {open ? <CloseIcon /> : <BurgerIcon />}
          </button>
        </div>
      </div>
      <div
        id="mobile-menu"
        className={`grid transition-[grid-template-rows,opacity] duration-300 ease-out ${
          open ? "opacity-100 [grid-template-rows:1fr]" : "opacity-0 [grid-template-rows:0fr]"
        }`}
      >
        <nav className="overflow-hidden" aria-label="Разделы">
          <ul className="p-3">
            {nav.map((item) => (
              <li key={item.href}>
                <a
                  href={item.href}
                  onClick={() => setOpen(false)}
                  className="flex min-h-[52px] items-center rounded-xl px-6 text-s text-ink transition-colors duration-200 hover:bg-surface active:bg-surface-active"
                >
                  {item.label}
                </a>
              </li>
            ))}
          </ul>
        </nav>
      </div>
    </div>
  );
}
