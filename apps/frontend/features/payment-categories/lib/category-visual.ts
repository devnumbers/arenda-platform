import type { FC, SVGProps } from 'react';
import { BoldOther } from '@/shared/assets/icons';
import { categoryIconComponents } from '@/features/payment-categories/lib/icon-registry';
import { categoryStyle } from '@/features/payment-categories/lib/category-style';

/**
 * Кругляш категории платежа для поверхностей вне карточек платежей
 * (#1348 — иконки категорий в уведомлениях): resolved-компонент bold-иконки
 * и цвет подложки каталога. Потребитель (NotificationCategoryIcon) рисует
 * компонент белым на подложке по своим вариантам размера.
 */
export type PaymentCategoryVisual = Readonly<{
  component: FC<SVGProps<SVGSVGElement>>;
  color: string;
}>;

/** Резолвит справочник категории в кругляш: дефолтная категория — иконка и
 * цвет каталога по слагу; `custom` и неизвестный слаг — дефолт
 * пользовательских (тот же договор, что у categoryStyle). */
export function paymentCategoryVisual(
  source: 'default' | 'custom',
  slug?: string,
): PaymentCategoryVisual {
  const { icon, color } = categoryStyle(source, slug);
  return { component: categoryIconComponents[icon] ?? BoldOther, color };
}

/** Снимок категории платежа из payload/кадра (#1348): id пользовательской
 * категории говорит «custom» (приоритет внутри снимка — над слагом), слаг —
 * «default» с фолбэком каталога. Нет снимка — нет кругляша: потребитель
 * рисует фото объекта/стандартную иконку. */
export function paymentCategoryVisualFromSnapshot(
  snapshot: { slug?: string; userCategoryId?: string } | undefined,
): PaymentCategoryVisual | undefined {
  if (snapshot === undefined) return undefined;
  if (snapshot.userCategoryId !== undefined) return paymentCategoryVisual('custom');
  return paymentCategoryVisual('default', snapshot.slug);
}
