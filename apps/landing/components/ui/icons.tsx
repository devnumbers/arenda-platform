// Иконки дизайн-слоя перенесённых ui-компонентов — инлайн-SVG (канон
// лендинга, без @svgr); оригиналы: apps/frontend/shared/assets/icons.
export function Cancel({ className }: { className?: string }) {
  return (
    <svg
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      className={className}
    >
      <path
        d="M18 6L6.00081 17.9992M17.9992 18L6 6.00085"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
