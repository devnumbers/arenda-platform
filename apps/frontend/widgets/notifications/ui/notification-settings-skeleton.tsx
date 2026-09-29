import type { JSX } from 'react';
import { Skeleton } from '@/shared/ui/design';

/** Контентный скелет экрана «Настроить уведомления» — без каркаса саб-экрана:
 * шелл рендерит страница (page.tsx) или route-loading, экрану и лоадингу
 * нужна только содержимая часть. Геометрия повторяет финальный лейаут
 * (паритет #604, §7): мастер-строка py-3, группы mt-6 с заголовком 32,
 * двухстрочным описанием 18/18 и двумя рядами тумблеров py-3; ширины
 * детерминированы. Живёт в двух фазах: route-loading (loading.tsx
 * сегмента) и pending пробы браузера в самом экране — пуш-состав
 * («Разрешите пуши», тумблеры) известен только после гидратации, поэтому
 * экран держит скелет до оседания пробы и меняет его одним свапом
 * (аудит #877 — без гейта карточка вставала после монтирования и
 * сдвигала секции, CLS 0.15). */
export function NotificationSettingsContentSkeleton(): JSX.Element {
  return (
    <div className="flex flex-col pb-6 pt-1" aria-label="Загрузка настроек">
      <div className="flex items-center justify-between py-3">
        <Skeleton className="h-5 w-56" />
        <Skeleton className="h-7 w-16 rounded-pill" />
      </div>
      {[0, 1, 2, 3].map((group) => (
        <section key={group} className="mt-6">
          <Skeleton className="h-8 w-52" />
          <div className="mb-3 mt-2 flex flex-col">
            <Skeleton className="h-[18px] w-full" />
            <Skeleton className="h-[18px] w-[233px]" />
          </div>
          <div className="flex items-center justify-between py-3">
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-7 w-16 rounded-pill" />
          </div>
          <div className="flex items-center justify-between py-3">
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-7 w-16 rounded-pill" />
          </div>
        </section>
      ))}
    </div>
  );
}
