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
export { Textarea, type TextareaProps } from './text-area';
export { Checkbox, type CheckboxProps } from './checkbox';
export { RadioGroup, type RadioGroupProps, RadioGroupItem, type RadioGroupItemProps } from './radio';
export { Switch, type SwitchProps } from './switch';
export { SearchField, type SearchFieldProps } from './search-field';
export { PickerField, type PickerFieldProps, type PickerOption } from './picker-field';
export { ChipButton, type ChipButtonProps } from './chip-button';
export { StepsChip, type StepsChipProps, type StepsChipSize } from './steps-chip';
export { CalendarButton, type CalendarButtonProps, type CalendarButtonState } from './calendar-button';
export { AmountField, type AmountFieldProps } from './amount-field';
export { amountKopecks, groupedAmount, sanitizeAmountInput, syncAmountInputDom } from './amount-input';
export { CalendarMonth, type CalendarMonthProps } from './calendar-month';
export { MonthDaysGrid, type MonthDaysGridProps } from './month-days-grid';
export { monthTitle, MONTH_LABELS } from './month-grid';
export { MonthYearPicker, type MonthYearPickerProps } from './month-year-picker';
export { WheelPicker, type WheelPickerProps, type WheelPickerItem } from './wheel-picker';
export {
  WheelPickerSheet,
  type WheelPickerSheetProps,
  type WheelPickerSheetAction,
} from './wheel-picker-sheet';
export {
  CalendarDatePicker,
  CalendarRangePicker,
  type CalendarDatePickerProps,
  type CalendarRangePickerProps,
} from './calendar-date-picker';
export { ListRow, type ListRowProps } from './list-row';
export { RoundActionButton, type RoundActionButtonProps } from './round-action-button';
export { StickyBottomBar, type StickyBottomBarProps } from './sticky-bottom-bar';
export { StatusIcon, type StatusIconProps, type StatusIconStatus } from './status-icon';
export { PageContent, type PageContentProps } from './page-content';
export { TopNav, type TopNavProps, TopNavTitle, type TopNavTitleProps } from './top-nav';
export {
  TopNavUserContext,
  TopNavUserContextProvider,
  useTopNavUser,
  type TopNavUser,
} from './top-nav-user-context';
export {
  TabBar,
  TabBarVisibilityProvider,
  useTabBarSuppression,
  useTabBarSuppressionState,
} from './tab-bar';
export { MoreSheet, type MoreSheetProps } from './more-sheet';
export { DesktopMenuButton, type DesktopMenuButtonProps } from './desktop-menu-button';
export { DesktopSidebar } from './desktop-sidebar';
export { DesktopNavPills } from './desktop-nav-pills';
export { HeaderLogo, type HeaderLogoProps } from './header-logo';
export {
  Modal,
  ModalClose,
  ModalContent,
  ModalTrigger,
  useIsDesktop,
  type ModalContentProps,
} from './modal';
export { RoundCheckbox, type RoundCheckboxProps } from './round-checkbox';
export { Menu, MenuTrigger, MenuContent, MenuItem, type MenuContentProps, type MenuItemProps } from './menu';
export {
  PickerMenu,
  type PickerMenuProps,
  type PickerMenuGroup,
  type PickerMenuOption,
} from './picker-menu';
export {
  CollapsibleSection,
  type CollapsibleSectionProps,
} from './collapsible-section';
export { ConfirmDialog, type ConfirmDialogProps } from './confirm-dialog';
export { EmptyState, type EmptyStateProps } from './empty-state';
export { Skeleton, type SkeletonProps } from './skeleton';
