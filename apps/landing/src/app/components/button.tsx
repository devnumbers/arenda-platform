import { ReactNode } from "react";

type Variant = "primary" | "white" | "ghost" | "light" | "light-sm";

const base =
  "content-stretch flex items-center justify-center gap-[8px] overflow-clip cursor-pointer transition-[background-color,transform,opacity] duration-150 select-none active:scale-[0.97]";

const variants: Record<Variant, string> = {
  // Blue CTA
  primary:
    "bg-[#2b7fff] text-white px-[24px] py-[16px] rounded-[16px] hover:bg-[#1f6fe8] active:bg-[#175fcc]",
  // White button on dark background
  white:
    "bg-white text-[#222] px-[24px] py-[16px] rounded-[16px] hover:bg-[#eef0f3] active:bg-[#e2e5ea]",
  // Translucent white button on image/dark background
  ghost:
    "bg-[rgba(255,255,255,0.2)] text-white px-[24px] py-[16px] rounded-[16px] hover:bg-[rgba(255,255,255,0.3)] active:bg-[rgba(255,255,255,0.15)]",
  // Light gray button
  light:
    "bg-[#f1f3f6] text-[#222] px-[24px] py-[16px] rounded-[16px] hover:bg-[#e7eaef] active:bg-[#dde1e8]",
  // Small light gray button (header "Войти")
  "light-sm":
    "bg-[#f1f3f6] text-[#222] px-[20px] py-[9px] rounded-[12px] hover:bg-[#e7eaef] active:bg-[#dde1e8]",
};

export function Button({
  variant = "primary",
  children,
  icon,
  onClick,
  className = "",
  type = "button",
  disabled,
}: {
  variant?: Variant;
  children: ReactNode;
  icon?: ReactNode;
  onClick?: () => void;
  className?: string;
  type?: "button" | "submit";
  disabled?: boolean;
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className={`${base} ${variants[variant]} ${
        disabled ? "opacity-40 pointer-events-none" : ""
      } ${className}`}
    >
      {icon}
      <span className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] text-[16px] text-center whitespace-nowrap overflow-hidden text-ellipsis">
        {children}
      </span>
    </button>
  );
}
