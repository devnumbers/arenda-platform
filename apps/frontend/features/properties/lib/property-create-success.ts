/**
 * Тексты экрана успеха визарда создания объекта (Figma 1425-55788, #483):
 * заголовок «Объект «{название}» создан» (20/24 SemiBold) и подзаголовок
 * макета. Подзаголовок и пара кнопок («Добавить аренду» + «Открыть
 * объект») возвращены владельцем 2026-09-02 — override дефолта #483
 * («одна кнопка, без аренды», домен аренд удалён ADR 0046): аренды в
 * продукте пока нет, кнопка «Добавить аренду» — заглушка с тостом.
 */

export type PropertyCreateSuccessCopy = {
  readonly heading: string;
  readonly description: string;
};

export const PROPERTY_CREATE_RENTAL_STUB_TOAST = 'Раздел «Аренда» скоро появится';

export function propertyCreateSuccessCopy(name: string): PropertyCreateSuccessCopy {
  return {
    heading: `Объект «${name}» создан`,
    description: 'Вы создали объект, теперь можете добавить аренду',
  };
}
