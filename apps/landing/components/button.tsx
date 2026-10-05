import type { ButtonHTMLAttributes, AnchorHTMLAttributes, ReactNode } from "react";

// Система кнопок лендинга — сверено с доской состояний 2864-5099:
// текст Onest 400 (большие 18/22, хедерные 14/18); primary #2b7fff
// (hover #2175f5, active #176beb), gray/white — Default/Hover/Active
// #f3f4f6/#e9eaec/#dfe0e2 (белая на цветной подложке гаснет той же
// лесенкой); фокус — инсет-кольцо 2px (у синей кнопки #171a1c, у серой
// и белой #2b7fff); радиусы: 64px-кнопки 16, хедерные 44px-чипы 12.
// cursor-pointer задан явно: Tailwind v4 не даёт <button> pointer сам.

function cx(...classes: Array<string | false | undefined>) {
  return classes.filter(Boolean).join(" ");
}

const BASE =
  "inline-flex cursor-pointer items-center justify-center rounded-2xl transition-colors duration-200 outline-none select-none focus-visible:ring-2 focus-visible:ring-inset";

const VARIANTS = {
  primary:
    "bg-primary text-white hover:bg-primary-hover active:bg-primary-active focus-visible:ring-ink",
  gray: "bg-surface text-ink hover:bg-surface-hover active:bg-surface-active focus-visible:ring-primary",
  white:
    "bg-white text-ink hover:bg-surface-hover active:bg-surface-active focus-visible:ring-primary",
} as const;

const SIZES = {
  // Мобильный макет — кнопки 56px/16px (Small), десктоп — 64px/18px (Default).
  lg: "h-14 px-8 text-s desk:h-16 desk:text-r",
  // Small без десктопного до-роста: инстанс 177×56 (16/20) на всех ярусах —
  // так во всех трёх новых кадрах FAQ (2967-76035 / 3005-78296 / 3009-79890),
  // прежний макет CTA 2814-1131 рисовал Default 191×64.
  sm: "h-14 px-8 text-s",
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
