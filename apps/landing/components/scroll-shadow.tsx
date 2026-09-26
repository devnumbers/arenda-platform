"use client";

import { useEffect, useState, type ReactNode } from "react";

// Тень хедера по скроллу: у верхней позиции (скролла нет) тени нет — макет
// 2814-1164; при прокрутке плашка получает мягкую тень. Флаг кладётся
// атрибутом data-scrolled на обёртку — бары подхватывают его групповым
// вариантом group-data-scrolled:*, оставаясь серверными компонентами.
export function ScrollShadow({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 4);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <div className={className} data-scrolled={scrolled ? "" : undefined}>
      {children}
    </div>
  );
}
