/**
 * Тексты экрана успеха визарда создания объекта (Figma 1425-55788, #483):
 * заголовок «Объект «{название}» создан» (20/24 SemiBold) и подзаголовок
 * макета. Подзаголовок и пара кнопок («Добавить аренду» + «Открыть
 * объект») возвращены владельцем 2026-09-02 — override дефолта #483.
 * Кнопка «Добавить аренду» — настоящий вход в визард создания аренды
 * (карта #984): заглушка ADR 0046 («домена аренд нет») снесена.
 */

export type PropertyCreateSuccessCopy = {
  readonly heading: string;
  readonly description: string;
};

export function propertyCreateSuccessCopy(name: string): PropertyCreateSuccessCopy {
  return {
    heading: `Объект «${name}» создан`,
    description: 'Вы создали объект, теперь можете добавить аренду',
  };
}
