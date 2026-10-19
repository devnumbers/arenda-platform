import type { ReactNode } from 'react';
import type { JSX } from 'react';
import { cn } from '@/shared/lib/cn';
import { useKeyboardActivation } from '@/shared/lib/hooks/useKeyboardActivation';
import { formatMoneyKopecks } from '@/shared/lib/format-money';
import { LiveValue } from '@/shared/ui/live-value';
import { SNAP_TILE_CLASSES } from '@/shared/ui/snap-rail';

/**
 * Карточка платежа (компонент Figma «Payment Card Button», 705:10625 —
 * резолюция #449): серая плитка-карточка с иконкой категории (слот
 * `leading`, в Figma — кант по серой поверхности), суммой и датой сверху,
 * названием платежа и объектом снизу. Ширина fluid — плитка растягивается
 * по ленте: 2 карточки на мобайл-ярусе `(100%−8)/2`, 3 на планшете/ПК
 * `(100%−16)/3` (решение владельца 18.10 — «растягивалась и
 * подстраивалась»; макеты 1041:51110/1043:52256 держат ту же сетку 24/8,
 * fluid убирает обрезки краевых карточек фикс-плитки 168.5). Высота —
 * натуральная по контенту, в ленте карточки вытягиваются до самой высокой
 * соседки (justify-between держит низ у дна): 122–137 у платёжных
 * карточек, 103–118 у объектных — макет 1036:39585 рисует объектные 118,
 * жёсткий h-137 оставлял им пустоту в середине. Сумма — пропорциональными
 * цифрами (`normal-nums`: канон tabular-nums LiveValue разреживал суммы
 * против макета, решение владельца 18.10 — «суммы выглядят странно»).
 * Стили просрочки (`danger`) — срок красным, сумма остаётся чёрной
 * (макеты 1041:51110/1043:52256; решение владельца 18.10 — перекрывает
 * «сумма и срок красным» #452 для карточек хаба); danger-бейдж на иконке
 * категорий остаётся её пропом. Без `amountKopecks` верх остаётся
 * только иконкой, а `accentTitle` красит название синим (варианты «Blue
 * Title» 879:17555/879:17565 — замыкающие карточки секций главного экрана
 * «Платежи», #578). `accentDescription` красит срок синим — карточки
 * секции «На оплату»: метка «Сегодня/Завтра/Через N дней» под суммой
 * (1036:37525, #1347).
 */
export type PaymentCardButtonProps = {
  /** Заголовок карточки, например название платежа. */
  readonly title: ReactNode;
  /** Подпись под заголовком, например название объекта. */
  readonly subtitle?: ReactNode;
  /** Сумма; не задана — строки суммы нет (карточки «Все …»/«Показать все»). */
  readonly amountKopecks?: number;
  /** Срок под суммой: дата или «N дней» просрочки. */
  readonly description?: ReactNode;
  readonly leading?: ReactNode;
  /** Просроченная операция: срок красным, сумма чёрная (макеты
   * 1041:51110/1043:52256, решение владельца 18.10). */
  readonly danger?: boolean;
  /** Название синим (879:17555/879:17565) — замыкающая карточка секции. */
  readonly accentTitle?: boolean;
  /** Срок под суммой синим (1036:37525) — карточка секции «На оплату». */
  readonly accentDescription?: boolean;
  readonly onSelect?: () => void;
  readonly disabled?: boolean;
  readonly className?: string;
};

export function PaymentCardButton({
  title,
  subtitle,
  amountKopecks,
  description,
  leading,
  danger = false,
  accentTitle = false,
  accentDescription = false,
  onSelect,
  disabled = false,
  className,
}: PaymentCardButtonProps): JSX.Element {
  const activatorProps = useKeyboardActivation({ onSelect, disabled });

  return (
    <div
      {...activatorProps}
      className={cn(
        /* Ширина fluid (решение владельца 18.10): проценты резолвятся по
         * контент-боксу ленты — с bleed ≤560 это вьюпорт минус 2×24, выше —
         * колонка минус 2×24. Высота натуральная: в ленте карточки
         * вытягиваются по самой высокой соседке (mt-auto прижимает низ) —
         * 122–137 у платёжных, 103–118 у объектных (макет 1036:39585). */
        'pressable flex shrink-0 cursor-pointer flex-col rounded-card bg-surface-muted p-4 outline-none',
        /* Ширина fluid — единый канон лент (SNAP_TILE_CLASSES, 18.10). */
        SNAP_TILE_CLASSES,
        'focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white',
        'hover:opacity-80',
        onSelect === undefined && 'cursor-default',
        disabled && 'pointer-events-none opacity-50',
        className,
      )}
    >
      <span className="flex w-full pb-3">
        <span className="flex min-w-0 flex-1 items-center gap-2">
          {leading !== undefined && <span className="shrink-0">{leading}</span>}
          {amountKopecks !== undefined && (
            <span className="flex min-w-0 flex-1 flex-col justify-center gap-1 text-xs font-medium">
              {/* Канон B (flash): плотная плитка — смена суммы без
               * кроссфейда, только вспышка; valueKey — копейки. normal-nums
               * гасит канонный tabular-nums корня LiveValue: сумма плитки —
               * пропорциональными цифрами макета (18.10). */}
              <LiveValue valueKey={amountKopecks} mode="flash" className="min-w-0 max-w-full">
                <span className="truncate text-content normal-nums">
                  {formatMoneyKopecks(amountKopecks)}
                </span>
              </LiveValue>
              {description !== undefined && (
                <span
                  className={cn(
                    'truncate',
                    danger
                      ? 'text-danger'
                      : accentDescription
                        ? 'text-primary'
                        : 'text-content-tertiary',
                  )}
                >
                  {description}
                </span>
              )}
            </span>
          )}
        </span>
      </span>
      <span className="mt-auto flex w-full flex-col gap-1 text-xs font-medium">
        <span className={cn('line-clamp-2', accentTitle ? 'text-primary' : 'text-content')}>
          {title}
        </span>
        {subtitle !== undefined && (
          <span className="truncate text-content-tertiary">{subtitle}</span>
        )}
      </span>
    </div>
  );
}
