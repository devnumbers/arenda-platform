/**
 * Дизайн-слой новых экранов (ADR 0050, тикет #455): shadcn/ui-паттерн —
 * компоненты в репо поверх Radix-примитивов, стили Tailwind-утилитами на
 * токенах shared/styles/tokens.css (проброс — @theme inline в
 * app/globals.css), шрифт Onest. Легаси-киты shared/ui/<компонент>/ на
 * CSS-модулях не переписываются — дублирование переходного периода
 * осознанно (ADR 0050).
 */
export { Button, type ButtonProps } from './button';
export { IconButton, type IconButtonProps } from './icon-button';
export { UserButton, type UserButtonProps } from './user-button';
export { TextField, type TextFieldProps, type TextFieldVariant } from './text-field';
export { Checkbox, type CheckboxProps } from './checkbox';
export { RadioGroup, type RadioGroupProps, RadioGroupItem, type RadioGroupItemProps } from './radio';
export { Switch, type SwitchProps } from './switch';
export { SearchField, type SearchFieldProps } from './search-field';
export { ChipButton, type ChipButtonProps } from './chip-button';
export { StepsChip, type StepsChipProps, type StepsChipSize } from './steps-chip';
export { CalendarButton, type CalendarButtonProps, type CalendarButtonState } from './calendar-button';
export { ListRow, type ListRowProps } from './list-row';
export { StickyBottomBar, type StickyBottomBarProps } from './sticky-bottom-bar';
export { StatusIcon, type StatusIconProps, type StatusIconStatus } from './status-icon';
export { PageContent, type PageContentProps } from './page-content';
export { TopNav, type TopNavProps, TopNavTitle, type TopNavTitleProps } from './top-nav';
export {
  Modal,
  ModalClose,
  ModalContent,
  ModalTrigger,
  type ModalContentProps,
} from './modal';
