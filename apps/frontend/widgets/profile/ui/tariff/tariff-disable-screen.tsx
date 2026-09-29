'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { BoldHome } from '@/shared/assets/icons';
import {
  Button,
  CircleIcon,
  ConfirmDialog,
  RadioGroup,
  RadioGroupItem,
  Skeleton,
  StickyBottomBar,
} from '@/shared/ui/design';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import type { ApiError } from '@/shared/api/errors';
import {
  useCancelSubscription,
  useSubscription,
} from '@/features/billing';
import { useProperties } from '@/features/properties';
import type { Property } from '@/entities/property';
import type { Subscription } from '@/entities/billing';
import {
  disableConfirmTitle,
  tariffDisableObjectsCard,
} from '@/widgets/profile/lib/tariff-disable';
import { NO_SUBSCRIPTION } from '@/widgets/profile/lib/no-subscription';
import {
  tariffAboutCard,
  tariffFeatureRows,
} from '@/widgets/profile/lib/tariff-about';
import { TariffAboutCard, TariffFeaturesCard } from './tariff-about-cards';
import { TariffDisableSuccessScreen } from './tariff-disable-success-screen';

const CONFIRM_DESCRIPTION =
  'Вы потеряете доступ к объектам и совместному доступу. Данные сохранятся и вернутся вместе с подпиской';

/** Экран «Отключение тарифа» (#622, макеты 1929-76786 выбор объекта,
 * 1933-78123 подтверждение-шит, 1933-78038 успех): карточка «У вас N
 * объектов» с радиосписком активных (только когда их больше одного —
 * иначе сохраняемый выбирает бэк), карточка тарифа с «Возможностями»,
 * футер «Оставить тариф» / «Отключить тариф». Отключение — через канон
 * ConfirmDialog (danger) → POST /subscription/cancel {keepPropertyId?}.
 * Данные — кэш GET /subscription и GET /properties (staleTime 30с),
 * скелетон повторяет геометрию (DESIGN.md §7). */
export function TariffDisableScreen(): JSX.Element {
  const subscription = useSubscription();
  const properties = useProperties();

  if (subscription.isPending || properties.isPending) {
    return <TariffDisableSkeleton />;
  }

  if (subscription.isError || properties.isError) {
    return (
      <div className="flex flex-col items-center gap-4 pt-6">
        <p className="text-center text-base leading-[18px] text-content-secondary">
          Не удалось загрузить данные тарифа
        </p>
        <Button
          variant="secondary"
          size="small"
          onClick={() => {
            void subscription.refetch();
            void properties.refetch();
          }}
        >
          Повторить
        </Button>
      </div>
    );
  }

  return (
    <TariffDisableContent
      subscription={subscription.data ?? NO_SUBSCRIPTION}
      properties={properties.data}
    />
  );
}

function TariffDisableContent({
  subscription,
  properties,
}: {
  readonly subscription: Subscription;
  readonly properties: Property[];
}): JSX.Element {
  const router = useRouter();
  const cancel = useCancelSubscription();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [keepPropertyId, setKeepPropertyId] = useState<string>();
  const [cancelled, setCancelled] = useState(false);

  const card = tariffDisableObjectsCard(properties.length);
  // По умолчанию выбран первый в списке — отправляем его явно, чтобы
  // выбор на экране и keepPropertyId в запросе не расходились.
  const selectedId = keepPropertyId ?? properties[0]?.id;

  if (cancelled) {
    return <TariffDisableSuccessScreen tariffName={subscription.tariff.name} />;
  }

  const handleConfirm = (): void => {
    cancel
      .mutateAsync(
        card.showPicker && selectedId !== undefined
          ? { keepPropertyId: selectedId }
          : undefined,
      )
      .then(() => {
        setConfirmOpen(false);
        setCancelled(true);
      })
      .catch((error: ApiError) =>
        notify.scenarios.tariff.disableError({ description: error.detail }),
      );
  };

  return (
    <>
      {/* Футер несёт две кнопки (168px) — канонного клиренса PageContent
          pb-136 не хватает, добор 48px возвращает зазор (как в grace #621). */}
      <div className="flex flex-col gap-4 pb-12">
        <div className="flex justify-center py-16">
          <h1 className="m-0 text-center text-2xl font-semibold leading-8 text-content">
            Не теряйте доступ к своим объектам
          </h1>
        </div>

        <section className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
          <div className="flex flex-col gap-2">
            <h2 className="m-0 text-2xl font-semibold leading-8 text-content">{card.title}</h2>
            <p className="m-0 text-sm leading-4 text-content-secondary">{card.description}</p>
          </div>

          {card.showPicker && (
            <RadioGroup
              aria-label="Объект, который останется активным"
              value={selectedId}
              onValueChange={setKeepPropertyId}
              className="gap-0"
            >
              {properties.map((property) => (
                <PropertyPickRow key={property.id} property={property} />
              ))}
            </RadioGroup>
          )}
        </section>

        <TariffAboutCard view={tariffAboutCard(subscription)} />

        <TariffFeaturesCard rows={tariffFeatureRows(subscription.tariff)} />
      </div>

      <StickyBottomBar>
        <div className="flex flex-col gap-2">
          <Button onClick={() => goBack(router, ROUTES.profileTariffAbout)}>
            Оставить тариф
          </Button>
          <Button variant="secondary" onClick={() => setConfirmOpen(true)}>
            Отключить тариф
          </Button>
        </div>
      </StickyBottomBar>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={disableConfirmTitle(subscription.tariff.name)}
        description={CONFIRM_DESCRIPTION}
        confirmLabel="Отключить"
        confirmVariant="danger"
        pending={cancel.isPending}
        onConfirm={handleConfirm}
      />
    </>
  );
}

/** Ряд выбора сохраняемого объекта (макет 1929-76786): фото-аватар 44
 * (без фото — белая заглушка с BoldHome #D3D7D9, как в пикере объектов),
 * название 16/18 и адрес 14/16, каноновое радио справа; отмеченный ряд
 * задаёт value группы RadioGroup. Список — ответ GET /properties целиком:
 * он несёт active+maintenance, и оба статуса бэк принимает в keepPropertyId
 * (архивные живут в отдельном /properties/archive). */
function PropertyPickRow({
  property,
}: {
  readonly property: Property;
}): JSX.Element {
  const photoUrl = property.photos?.[0]?.url;

  return (
    <label className="flex cursor-pointer items-center gap-3 py-3">
      <CircleIcon variant="muted" aria-hidden className="relative overflow-hidden rounded-full">
        {photoUrl !== undefined ? (
          <img src={photoUrl} alt="" className="h-full w-full object-cover" />
        ) : (
          <BoldHome className="h-6 w-6 text-[#D3D7D9]" />
        )}
      </CircleIcon>
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-base font-medium leading-[18px] text-content">{property.name}</span>
        <span className="truncate text-sm leading-4 text-content-secondary">
          {property.address}
        </span>
      </span>
      <RadioGroupItem value={property.id} aria-label={property.name} className="mr-2.5" />
    </label>
  );
}

/** Скелетон геометрии экрана (вариант с выбором — старший кейс). */
function TariffDisableSkeleton(): JSX.Element {
  return (
    <div
      className="flex flex-col gap-4"
      role="status"
      aria-label="Загрузка экрана отключения тарифа"
    >
      <div className="flex justify-center py-16">
        <Skeleton className="h-8 w-72 bg-surface-muted-hover" />
      </div>
      <section className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
        <Skeleton className="h-8 w-40 bg-surface-muted-hover" />
        <div className="flex flex-col gap-2">
          <Skeleton className="h-4 bg-surface-muted-hover" />
          <Skeleton className="h-4 w-4/5 bg-surface-muted-hover" />
        </div>
        <div className="flex flex-col">
          {[0, 1].map((index) => (
            <div key={index} className="flex items-center gap-3 py-3">
              <Skeleton className="h-11 w-11 rounded-full bg-surface-muted-hover" />
              <div className="flex flex-1 flex-col gap-2">
                <Skeleton className="h-[18px] w-2/5 bg-surface-muted-hover" />
                <Skeleton className="h-4 w-3/5 bg-surface-muted-hover" />
              </div>
            </div>
          ))}
        </div>
      </section>
      <section className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
        <Skeleton className="h-8 w-20 bg-surface-muted-hover" />
        {[0, 1, 2].map((index) => (
          <div key={index} className="flex flex-col gap-2">
            <Skeleton className="h-[18px] w-28 bg-surface-muted-hover" />
            <Skeleton className="h-6 bg-surface-muted-hover" style={{ width: `${72 - index * 12}%` }} />
          </div>
        ))}
      </section>
      <section className="flex flex-col gap-6 rounded-[32px] bg-surface-muted p-8">
        <Skeleton className="h-8 w-44 bg-surface-muted-hover" />
        {[0, 1].map((index) => (
          <div key={index} className="flex gap-4">
            <Skeleton className="h-12 w-12 shrink-0 bg-surface-muted-hover" />
            <div className="flex flex-col gap-2">
              <Skeleton className="h-[18px] w-40 bg-surface-muted-hover" />
              <Skeleton className="h-4 w-56 bg-surface-muted-hover" />
            </div>
          </div>
        ))}
      </section>
    </div>
  );
}
