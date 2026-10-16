import type { JSX, ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

export type HubTitleProps = {
  readonly children: ReactNode;
  readonly className?: string;
};

/** Заголовок раздела на хаб-экране (Mobile/Heading/H1/600 — 28/32):
 * единая анатомия хаб-шапки #556 — под TopNav с крыльями, внутри
 * PageContent. Горизонтальный паддинг 24 несёт сам заголовок (PageContent
 * горизонталей не вкладывает); m-0 гасит легаси-маргин h1 из @layer base
 * (0.67em — лишние ~19px до и после, в макете зазор до хедера ровно 24
 * от pt-6 PageContent); className переопределяется через tailwind-merge —
 * хабы со своей строкой-обёрткой снимают pl-6. */
export function HubTitle({ children, className }: HubTitleProps): JSX.Element {
  return (
    <h1 className={cn('m-0 pl-6 text-[28px] font-semibold leading-8 text-content', className)}>
      {children}
    </h1>
  );
}
