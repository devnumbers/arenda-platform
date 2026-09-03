import type { JSX, ReactNode } from 'react';

/** Секция-карточка списка задач (#499, Figma 1531:12784): серая карточка
 * radius 24 с заголовком H3 (дата группы, «Просроченные», «Без даты») и
 * строками задач. Сворачиваемый вариант «Выполненных» — CollapsibleSection
 * дизайн-слоя. */
export function TaskSectionCard({
  title,
  testId,
  children,
}: {
  readonly title: string;
  readonly testId: string;
  readonly children: ReactNode;
}): JSX.Element {
  return (
    <section data-testid={testId} className="mx-6 rounded-card bg-surface-muted pb-2">
      <h2 className="px-6 pt-6 pb-2 text-xl font-semibold leading-6 text-content">{title}</h2>
      <div className="flex flex-col">{children}</div>
    </section>
  );
}
