'use client';

import type { JSX } from 'react';
import { Button, EmptyState } from '@/shared/ui/design';

/** Совсем пустая лента «Задач нет» — канон EmptyState без переопределений
 * (DESIGN.md §7, анатомия 3235-74057): обёртка своего верхнего отступа не
 * несёт — pt-14 состояния отсчитывается от строки заголовка хаба, как на
 * «Платежах» (EmptyState сразу после хаб-шапки) и на bareEmpty «Объектов»
 * (решение владельца 14.10.2026); бывший mt-6 общей обёртки ленты съезжал
 * иллюстрацию на 24 вниз относительно других хабов. «+» шапки на пустоте
 * спрятан вместе с чипами (анатомия 3235-74057), создание — CTA состояния
 * (onCreate); нижний ритм страницы сохранён (pb-6, как у остальных веток
 * ленты). Вынесен из TasksFeedScreen чистым компонентом — шов канон-теста
 * (живой экран с хуками функцией не вызывается). */
export function TasksFeedEmpty({
  onCreate,
}: {
  readonly onCreate: () => void;
}): JSX.Element {
  return (
    <div className="flex flex-col pb-6">
      <EmptyState
        imageSrc="/images/tasks/empty-tasks.webp"
        imageRounded
        title="Задач нет"
        description="Добавьте задачу — например, позвонить арендатору, вызвать мастера или проверить состояние объекта"
        action={
          // CTA пустого состояния — канон (3235-74057): «+» шапки
          // на пустоте спрятан, создание остаётся здесь.
          <Button size="small" radius="m" onClick={onCreate}>
            Добавить задачу
          </Button>
        }
      />
    </div>
  );
}
