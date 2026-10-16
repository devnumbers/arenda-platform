import { type Subscription } from '@/entities/billing';

/**
 * Подписка-заместитель «подписки нет» (#768): 404 GET /subscription хук
 * отдаёт null, а экраны «Тарифа» не имеют спроектированного состояния
 * отсутствия подписки. Продукт и так читает отсутствие подписки как
 * бесплатный базовый тариф — хаб объектов у такого юзера показывает
 * «Достигнут лимит объектов по тарифу — сменить тариф» (#760), поэтому
 * экраны тарифа рендерят `subscription ?? NO_SUBSCRIPTION` — базовый,
 * активный, без периодов и платных атрибутов. Состояние в продукте
 * недостижимо (регистрация создаёт basic, #245) — это сид-юзеры.
 */
export const NO_SUBSCRIPTION: Subscription = {
  id: 'no-subscription',
  status: 'active',
  tariff: {
    id: 'basic',
    name: 'basic',
    monthlyPriceKopecks: 0,
    yearlyPriceKopecks: 0,
    activePropertyLimit: 1,
  },
  autoRenewEnabled: false,
};
