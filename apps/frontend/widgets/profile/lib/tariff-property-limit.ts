/**
 * Вид-модель интерстишала «Превышен лимит объектов» (карта #1459, тикет
 * #1469, макет Figma 3516-79702): адаптивный по тарифу текст описания —
 * компонент только рисует. Источник тарифа экрана — useSubscription;
 * подписки нет — читается как basic (канон #768).
 */

/** Заголовок экрана (H2) — один для всех тарифов (макет 3516-79702). */
export const PROPERTY_LIMIT_TITLE = 'Превышен лимит объектов';

const UPGRADE_TEXT_BASIC =
  'Повысьте тариф до «Про» или «Бизнес», чтобы добавить больше объектов';
const UPGRADE_TEXT_PRO =
  'Повысьте тариф до «Бизнес», чтобы добавить больше объектов';
const UPGRADE_TEXT_FALLBACK = 'Повысьте тариф, чтобы добавить больше объектов';

/** Описание под заголовком по тарифу читателя (решение владельца 15.10,
 * тикет #1469): basic (и отсутствие подписки — тоже basic) называет оба
 * платных тарифа, pro — только следующий; любой неожиданный тариф
 * (business с безлимитом на этот экран не попадает, будущие имена) —
 * defensive-фолбэк без имён. Вход — строка сознательно: сгенерированный
 * TariffName уже сегодня шире трёх именованных кейсов. */
export function propertyLimitUpgradeText(
  tariffName: string | null | undefined,
): string {
  if (tariffName === 'pro') {
    return UPGRADE_TEXT_PRO;
  }
  if (tariffName === 'basic' || tariffName === null || tariffName === undefined) {
    return UPGRADE_TEXT_BASIC;
  }
  return UPGRADE_TEXT_FALLBACK;
}
