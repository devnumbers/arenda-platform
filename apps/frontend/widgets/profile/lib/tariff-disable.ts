import type { TariffName } from '@/entities/user';
import { getTariffLabel } from '@/entities/user';
import { pluralize } from '@/shared/lib/pluralize';

/** Карточка «У вас N объектов» экрана «Отключение тарифа» (#622, макет
 * 1929-76786). Радиосписок и предложение «Выберите объект…» — только когда
 * активных объектов больше одного (0/1 — сохраняемый выбирает бэк). При
 * одном объекте архивная оговорка макета опускается: архивировать нечего,
 * остаются всегда верные факты про данные и участников. */
export type TariffDisableObjectsCard = {
  readonly title: string;
  readonly description: string;
  readonly showPicker: boolean;
};

const DESCRIPTION_WITH_CHOICE =
  'После отключения все объекты, кроме одного, перейдут в архив — данные сохранятся, но пользоваться ими будет нельзя. Приглашенные участники потеряют доступ к вашим объектам. Выберите объект, который останется активным';
const DESCRIPTION_WITHOUT_CHOICE =
  'После отключения данные сохранятся. Приглашенные участники потеряют доступ к вашим объектам.';

export function tariffDisableObjectsCard(
  activeCount: number,
): TariffDisableObjectsCard {
  return {
    title: `У вас ${activeCount} ${pluralize(activeCount, 'объект', 'объекта', 'объектов')}`,
    description: activeCount > 1 ? DESCRIPTION_WITH_CHOICE : DESCRIPTION_WITHOUT_CHOICE,
    showPicker: activeCount > 1,
  };
}

/** Вопрос подтверждения (#622, макет 1933-78123). */
export function disableConfirmTitle(tariffName: TariffName): string {
  return `Уверены, что хотите отключить тариф ${getTariffLabel(tariffName)}?`;
}

/** Заголовок экрана успеха (#622, макет 1933-78038). */
export function disableSuccessTitle(tariffName: TariffName): string {
  return `Тариф ${getTariffLabel(tariffName)} отключен`;
}
