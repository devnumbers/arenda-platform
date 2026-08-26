import type { ComponentProps, JSX } from 'react';
import { Switch as SwitchRoot, SwitchThumb } from '@radix-ui/react-switch';
import { cn } from '@/shared/lib/cn';

/** Переключатель дизайн-слоя (Figma 1031:21053): дорожка 64×28 radius 100,
 * бегунок-пилюля 36×24 (белый, тень 0 2px 8px rgba(0,0,0,0.16)) с ходом
 * 24px — в духе системного тумблера Apple: скольжение бегунка на кривой
 * --dl-ease (350ms, transition-transform), дорожка перекрашивается
 * цветовым пресетом. Включено — синяя дорожка (#2B7FFF → hover #2175F5 →
 * active #176BEB), выключено — серая с шагами muted-hover/active. */
export type SwitchProps = ComponentProps<typeof SwitchRoot>;

export function Switch({ className, ...props }: SwitchProps): JSX.Element {
  return (
    <SwitchRoot
      className={cn(
        'inline-flex h-7 w-16 shrink-0 cursor-pointer rounded-pill bg-surface-muted p-[2px] font-sans outline-none transition-colors',
        'hover:bg-surface-muted-hover active:bg-surface-muted-active',
        'data-[state=checked]:bg-primary data-[state=checked]:hover:bg-primary-hover data-[state=checked]:active:bg-primary-active',
        'focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
        'disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
      {...props}
    >
      <SwitchThumb className="block h-6 w-9 rounded-pill bg-white shadow-[0_2px_8px_rgba(0,0,0,0.16)] transition-transform ease-apple duration-(--dl-duration-move) data-[state=checked]:translate-x-6" />
    </SwitchRoot>
  );
}
