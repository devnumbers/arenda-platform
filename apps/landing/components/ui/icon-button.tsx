import type { ComponentProps, JSX, ReactNode } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/cn";

/**
 * Кнопка-иконка дизайн-слоя (Figma 934:19148, выверено экспортами всех
 * состояний) — перенос из apps/frontend (shared/ui/design/icon-button.tsx).
 * Круглая зона нажатия 44×44 под иконку 24×24:
 * Primary — прозрачная, иконка #171A1C, hover #F3F4F6, active #E9EAEC;
 * Secondary — прозрачная с серой иконкой #9FA8AC, hover и active —
 * фон #F3F4F6 с иконкой #6F787C; Danger — прозрачная с красной #FB2C36
 * (hover/active фон не меняют). Focus-visible — обводка 2px #2B7FFF
 * (ring-2 без смещения), только с клавиатуры.
 */
const iconButtonVariants = cva(
  "inline-flex shrink-0 cursor-pointer items-center justify-center rounded-pill font-sans outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary: "text-content hover:bg-surface-muted active:bg-surface-muted-hover",
        secondary:
          "text-content-tertiary hover:bg-surface-muted hover:text-content-secondary active:bg-surface-muted active:text-content-secondary",
        danger: "text-danger",
      },
    },
    defaultVariants: {
      variant: "primary",
    },
  },
);

export type IconButtonProps = ComponentProps<"button"> &
  VariantProps<typeof iconButtonVariants> & {
    readonly icon: ReactNode;
    /** Имя для screen reader — кнопка-иконка без текста обязана его иметь. */
    readonly label: string;
  };

export function IconButton({
  className,
  variant,
  icon,
  label,
  type = "button",
  ...props
}: IconButtonProps): JSX.Element {
  return (
    <button
      type={type}
      aria-label={label}
      className={cn(iconButtonVariants({ variant }), "h-11 w-11", className)}
      {...props}
    >
      <span className="flex h-6 w-6 items-center justify-center" aria-hidden>
        {icon}
      </span>
    </button>
  );
}
