"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

// Reveal при скролле: opacity+translateY, один раз, Apple-подобная кривая;
// prefers-reduced-motion — контент показан сразу без анимации. До гидратации
// элемент прозрачен — для no-JS в root layout есть noscript-переопределение.
export function Reveal({
  children,
  delay = 0,
  className,
}: {
  children: ReactNode;
  delay?: number;
  className?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [shown, setShown] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) {
      return;
    }
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      // Без анимации: правим DOM напрямую (внешняя система) — без setState.
      // translate-y-6 в Tailwind v4 — отдельное свойство `translate`, его
      // надо гасить отдельно, transform: none его не снимает.
      el.style.opacity = "1";
      el.style.transform = "none";
      el.style.translate = "none";
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          setShown(true);
          observer.disconnect();
        }
      },
      { rootMargin: "0px 0px -10% 0px", threshold: 0.1 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={ref}
      data-reveal=""
      style={{ transitionDelay: `${delay}ms` }}
      className={`transition-[opacity,translate] duration-700 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none ${
        shown ? "translate-y-0 opacity-100" : "translate-y-6 opacity-0"
      } ${className ?? ""}`}
    >
      {children}
    </div>
  );
}
