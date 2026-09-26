import type { ButtonHTMLAttributes, AnchorHTMLAttributes, ReactNode } from "react";

// Система кнопок лендинга — стили из Figma REST (компоненты 2864-5099,
// 2865-5138/5246, 2865-5362): текст кнопок Onest 400 (большие 18/22,
// хедерные 14/18); primary #2b7fff (hover #2175f5, active #176beb),
// gray #f3f4f6 (hover #e9eaec, active #dfe0e2); радиусы: 64px-кнопки 16,
// хедерные 44px-чипы 12.

function cx(...classes: Array<string | false | undefined>) {
  return classes.filter(Boolean).join(" ");
}

const BASE =
  "inline-flex items-center justify-center rounded-2xl transition-colors duration-200 outline-none focus-visible:ring-2 focus-visible:ring-primary/40 select-none";

const VARIANTS = {
  primary:
    "bg-primary text-white hover:bg-primary-hover active:bg-primary-active",
  gray: "bg-surface text-ink hover:bg-surface-hover active:bg-surface-active",
  // Белая кнопка на цветной подложке (хиро): hover/active — приглушение в серый.
  white: "bg-white text-ink hover:bg-surface active:bg-surface-hover",
} as const;

const SIZES = {
  // Мобильный макет — кнопки 56px (Small), десктоп — 64px (Default).
  lg: "h-14 px-9 text-r desk:h-16",
  md: "h-11 rounded-xl px-5 text-xs",
} as const;

type Variant = keyof typeof VARIANTS;
type Size = keyof typeof SIZES;

export function LandingButton({
  variant = "primary",
  size = "lg",
  className,
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: Size;
  children: ReactNode;
}) {
  return (
    <button
      className={cx(BASE, VARIANTS[variant], SIZES[size], className)}
      {...props}
    >
      {children}
    </button>
  );
}

export function LandingLink({
  variant = "primary",
  size = "lg",
  className,
  children,
  ...props
}: AnchorHTMLAttributes<HTMLAnchorElement> & {
  variant?: Variant;
  size?: Size;
  children: ReactNode;
}) {
  return (
    <a
      className={cx(BASE, VARIANTS[variant], SIZES[size], className)}
      {...props}
    >
      {children}
    </a>
  );
}
