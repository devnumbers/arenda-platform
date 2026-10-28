import type { JSX, ReactNode } from 'react';

/**
 * Строка секции «Управление» на экранах аренды — канон строк «Управления»
 * объекта (PropertyManageSection, Figma 1554:98469; решение владельца
 * 28.10 «точно такие же стили»): строка-кнопка 52 со скруглением, иконка в
 * слоте 44, подпись 16/medium; отклик на наведение/нажатие — фоном
 * (surface-muted-hover/active). Подпись — ReactNode: деструктивные строки
 * несут красный текст (канон #535).
 */
export function ManageRow({
  icon,
  label,
  onClick,
}: {
  readonly icon: ReactNode;
  readonly label: ReactNode;
  readonly onClick: () => void;
}): JSX.Element {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex h-[52px] w-full cursor-pointer items-center rounded-button px-1 text-left outline-none transition-colors focus-visible:ring-2 focus-visible:ring-primary hover:bg-surface-muted-hover active:bg-surface-muted-active"
    >
      <span className="flex h-11 w-11 shrink-0 items-center justify-center" aria-hidden>
        {icon}
      </span>
      <span className="pl-1 text-base font-medium">{label}</span>
    </button>
  );
}
