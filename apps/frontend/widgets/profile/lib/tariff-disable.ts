import type { Property } from '@/entities/property';
import { filterOwnedProperties } from '@/entities/property';
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

/** Выбор сохраняемого объекта на экране «Отключение тарифа» — единый
 * источник истины для радиосписка, предвыборки и payload запроса (#1518):
 * визуал и тело POST /subscription/cancel считаются из одного места,
 * разойтись им негде. */
export type KeepPropertySelection = {
  /** Кандидаты выбора — только СВОИ неархивные объекты
   * (filterOwnedProperties): бэк валидирует keepPropertyId по владению
   * (409 invalid_keep_property на чужой), а приглашённый объект и не
   * сохраняемый — при истечении архиваются только свои, чужие ноги гасит
   * recipient-slots без выбора (биллинг-глоссарий, «Выбор сохраняемого»). */
  readonly choices: readonly Property[];
  /** Отмеченный ряд: выбор пользователя, иначе фолбэк на первый свой;
   * чужой/устаревший id до payload не доходит — тихо замещается первым
   * своим, поэтому выбранный на экране и отправленный всегда совпадают. */
  readonly selectedId: string | undefined;
  /** keepPropertyId тела запроса: явно только когда своих больше одного
   * (0/1 — сохраняемый выбирает бэк), и всегда один из choices. */
  readonly payloadKeepPropertyId: string | undefined;
};

export function keepPropertySelection(
  chosenId: string | undefined,
  properties: ReadonlyArray<Property>,
): KeepPropertySelection {
  const choices = filterOwnedProperties(properties);
  const selectedId = choices.some((property) => property.id === chosenId)
    ? chosenId
    : choices[0]?.id;
  return {
    choices,
    selectedId,
    payloadKeepPropertyId: choices.length > 1 ? selectedId : undefined,
  };
}
