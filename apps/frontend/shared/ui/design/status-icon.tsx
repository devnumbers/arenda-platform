import type { FC, JSX, SVGProps } from 'react';
import {
  StatusIconCheck,
  StatusIconDanger,
  StatusIconGood,
  StatusIconInfo,
  StatusIconWarning,
} from '@/shared/assets/icons';
import { cn } from '@/shared/lib/cn';

/** Статусный бейдж дизайн-слоя (Figma 671:6751–6755): пять градаций —
 * градиентная плашка 24×24 с белым кантом и белым глифом. Цвета запечены
 * в SVG-экспортах Figma, размер управляется обёрткой. */
export type StatusIconStatus = 'danger' | 'warning' | 'good' | 'check' | 'info';

type IconComponent = FC<SVGProps<SVGSVGElement>>;

const statusIconMap = {
  danger: StatusIconDanger,
  warning: StatusIconWarning,
  good: StatusIconGood,
  check: StatusIconCheck,
  info: StatusIconInfo,
} as const satisfies Record<StatusIconStatus, IconComponent>;

export type StatusIconProps = {
  readonly status: StatusIconStatus;
  readonly className?: string;
};

export function StatusIcon({ status, className }: StatusIconProps): JSX.Element {
  const Icon = statusIconMap[status];
  // Кант по умолчанию белый (text-surface): в SVG он stroke="currentColor",
  // иначе цвет канта зависел бы от унаследованного текста потребителя.
  return <Icon className={cn('text-surface h-6 w-6 shrink-0', className)} aria-hidden />;
}
