import type { ComponentProps, JSX } from 'react';
import { Switch as SwitchRoot, SwitchThumb } from '@radix-ui/react-switch';
import { cn } from '@/shared/lib/cn';

/** Переключатель дизайн-слоя (Figma 1031:21053): дорожка 40×28 radius 100
 * с белым бегунком 36×24 (тень 0 2px 8px rgba(0,0,0,0.16)); включённое
 * состояние — синяя дорожка, нажатие — тёмно-синий #176BEB. */
export type SwitchProps = ComponentProps<typeof SwitchRoot>;

export function Switch({ className, ...props }: SwitchProps): JSX.Element {
  return (
    <SwitchRoot
      className={cn(
        'inline-flex h-7 w-10 shrink-0 rounded-pill bg-surface-muted p-[2px] font-sans outline-none transition-colors',
        'hover:bg-surface-muted-hover active:bg-surface-muted-active',
        'data-[state=checked]:bg-primary data-[state=checked]:hover:bg-primary-hover data-[state=checked]:active:bg-primary-active',
        'focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
        'disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <SwitchThumb className="block h-6 w-9 rounded-pill bg-white shadow-[0_2px_8px_rgba(0,0,0,0.16)]" />
    </SwitchRoot>
  );
}
