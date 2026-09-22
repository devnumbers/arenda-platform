'use client';

import { useEffect, useState, type JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { Cancel, SmallArrowDown } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { ROUTES } from '@/shared/config/routes';
import { pluralize } from '@/shared/lib/pluralize';
import {
  allInvitedProperties,
  collapsedInviteRows,
  inviteSelectionState,
  toInvitePropertyOption,
  toggleAllInvitedProperties,
  toggleInvitedProperty,
  type InvitePropertyOption,
} from '@/entities/participants';
import { useProperties } from '@/features/properties';
import {
  Button,
  EmptyState,
  ErrorCard,
  IconButton,
  PageContent,
  StickyBottomBar,
  TextField,
  TopNav,
  TopNavBackButton,
  TopNavTitle,
} from '@/shared/ui/design';
import { ObjectAvatarGlyph, PARTICIPANT_ROW_BASE_CLASS } from './participant-fragments';
import { InvitePropertiesRows } from './invite-properties-rows';
import { stageParticipantPopup } from '../lib/participant-popups';
import { useInviteForm } from '../lib/use-invite-form';
import { ParticipantRoleSegmented } from './participant-role-segmented';
import { ParticipantsInviteSkeleton } from './participants-skeletons';

/**
 * Экран «Пригласите участника» (карта #692, тикет #699; Figma 2008-46375,
 * 2010-134350): иллюстрация и заголовок, почта приглашаемого (плавающая
 * подпись «Электронная почта», очистка), сегмент роли «Просмотр |
 * Редактирование» (копирайт ролей — решение чарта карты #692, домен не
 * меняется) и свёрнутый выбор объектов.
 *
 * Выбор объектов: по умолчанию «Все N объектов / Поделиться всеми
 * объектами» — снапшот всех текущих активных объектов читающего (решение
 * чарта: будущие автоматически не шарятся; пустое состояние в макетах не
 * нарисовано — старт «все» выбран приёмкой). Строка открывает полноэкранный
 * пикер «Выбрать объект» (2008-46627): tri-state «Все объекты» и мультичек;
 * «Выбрать» коммитит черновик, крестик/Esc закрывают без изменений.
 *
 * «Пригласить» → POST /participants/invite (#694): зарегистрированная
 * почта получает членства сразу (per-property слоты — партия может
 * смешать active и suspended), незарегистрированная — pending-приглашения
 * и одно письмо. Любой granted (включая suspended) — попап «Участник
 * приглашен» (2010-134458) на цели goBack по канону истории; granted=0
 * или ошибка — сообщение под полем почты, экран остаётся на месте.
 */
export function ParticipantsInviteScreen(): JSX.Element {
  const router = useRouter();

  const propertiesQuery = useProperties();

  const [committedSelection, setCommittedSelection] = useState<ReadonlySet<string> | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);

  const form = useInviteForm({
    propertyIds: () => [...selection],
    grantedZeroMessage: 'Пользователь уже имеет доступ к выбранным объектам',
    onInvited: () => {
      stageParticipantPopup('invited');
      goBack(router, ROUTES.participants);
    },
  });
  const { email, setEmailValue, role, setRole, emailError, serverError, submitInvite } = form;

  // access отсутствует у собственных объектов (владелец); viewer — чужие
  // объекты, где читающий не управляет выдачей: бэк вернул бы
  // skipped_unavailable, поэтому такие строки даже не показываем.
  const properties = propertiesQuery.data;
  const options: InvitePropertyOption[] = (properties ?? [])
    .filter((property) => property.access?.role !== 'viewer')
    .map(toInvitePropertyOption);
  const optionIds = options.map((option) => option.id);

  const selection = committedSelection ?? allInvitedProperties(optionIds);
  const collapsedRows = collapsedInviteRows(options, selection);

  const trimmedEmail = email.trim();
  const canSubmit = trimmedEmail.length > 0 && selection.size > 0 && !form.isPending;

  let content: JSX.Element;
  if (propertiesQuery.isPending) {
    content = <ParticipantsInviteSkeleton />;
  } else if (propertiesQuery.isError) {
    content = (
      <ErrorCard
        title="Не удалось загрузить объекты"
        onRetry={() => void propertiesQuery.refetch()}
        className="mt-6"
      />
    );
  } else if (options.length === 0) {
    // Гард тикета: приглашать некуда — объектов читающего нет.
    content = (
      <EmptyState
        imageSrc="/images/tariff/tariff-about-sharing.png"
        title="Объектов пока нет"
        description="Чтобы пригласить участника, добавьте хотя бы один объект"
      />
    );
  } else {
    content = (
      <div className="flex flex-col pt-4">
        <Image
          src="/images/tariff/tariff-about-sharing.png"
          alt=""
          width={96}
          height={96}
          className="h-24 w-24 self-center"
        />
        <h1 className="mt-8 text-[28px] font-semibold leading-8 text-content">
          Пригласите участника
        </h1>
        <p className="mt-2 text-sm leading-4 text-content-secondary">
          Укажите электронную почту и выберите роль — просмотр или
          редактирование. Мы отправим приглашение в ваш объект
        </p>

        <div className="mt-8">
          <TextField
            variant="titleIn"
            title="Электронная почта"
            type="email"
            autoComplete="email"
            value={email}
            error={emailError ?? serverError ?? undefined}
            onClear={() => setEmailValue('')}
            onChange={(event) => setEmailValue(event.target.value)}
          />
        </div>

        <div className="mt-8">
          <ParticipantRoleSegmented value={role} disabled={form.isPending} onChange={setRole} />
        </div>

        <div className="mt-6 flex flex-col">
          {collapsedRows.map((row) => (
            <button
              key={row.kind === 'all' ? 'all' : row.option.id}
              type="button"
              onClick={() => setPickerOpen(true)}
              className={PARTICIPANT_ROW_BASE_CLASS}
            >
              {row.kind === 'all' ? (
                <>
                  <ObjectAvatarGlyph photoUrl={undefined} isAll />
                  <span className="flex min-w-0 flex-1 flex-col gap-1">
                    <span className="truncate text-base font-medium leading-[18px] text-content">
                      Все {options.length}{' '}
                      {pluralize(options.length, 'объект', 'объекта', 'объектов')}
                    </span>
                    <span className="truncate text-sm leading-4 text-content-secondary">
                      Поделиться всеми объектами
                    </span>
                  </span>
                </>
              ) : (
                <>
                  <ObjectAvatarGlyph photoUrl={row.option.photoUrl} />
                  <span className="flex min-w-0 flex-1 flex-col gap-1">
                    <span className="truncate text-base font-medium leading-[18px] text-content">
                      {row.option.name}
                    </span>
                    <span className="truncate text-sm leading-4 text-content-secondary">
                      {row.option.address}
                    </span>
                  </span>
                </>
              )}
              <SmallArrowDown className="h-6 w-6 shrink-0 text-content-tertiary" />
            </button>
          ))}
        </div>
      </div>
    );
  }

  return (
    <>
      <TopNav leading={<TopNavBackButton fallbackHref={ROUTES.participants} />} />

      {/* Боковой отступ 24px по макету (контентный фрейм x=24, ширина
       * 345 из 393) — на все состояния, включая скелетон. */}
      <PageContent className="px-6">{content}</PageContent>

      {options.length > 0 && (
        <StickyBottomBar>
          <Button
            className="w-full"
            disabled={!canSubmit}
            loading={form.isPending}
            onClick={submitInvite}
          >
            Пригласить
          </Button>
        </StickyBottomBar>
      )}

      {pickerOpen && (
        <InviteObjectsPicker
          options={options}
          initialSelection={selection}
          onClose={() => setPickerOpen(false)}
          onConfirm={(draft) => {
            setCommittedSelection(draft);
            setPickerOpen(false);
          }}
        />
      )}
    </>
  );
}

/**
 * Полноэкранный пикер «Выбрать объект» (макет 2008-46627; канвас
 * полноэкранного пикера — CalendarDatePicker #500): крестик, tri-state
 * «Все объекты / Поделиться всеми объектами», разделитель и мультичек
 * объектов. Черновик живёт, пока пикер смонтирован: «Выбрать» коммитит
 * выбор целиком, крестик или Esc закрывают без изменений. Пустой черновик
 * «Выбрать» не подтверждает: свёрнутая сводка — единственная точка входа в
 * пикер, нулевой коммит оставил бы экран без способа вернуть выбор.
 */
function InviteObjectsPicker({
  options,
  initialSelection,
  onClose,
  onConfirm,
}: {
  readonly options: ReadonlyArray<InvitePropertyOption>;
  readonly initialSelection: ReadonlySet<string>;
  readonly onClose: () => void;
  readonly onConfirm: (selection: ReadonlySet<string>) => void;
}): JSX.Element {
  const [draft, setDraft] = useState<ReadonlySet<string>>(initialSelection);

  const optionIds = options.map((option) => option.id);
  const allState = inviteSelectionState(draft, optionIds);

  // Esc закрывает пикер без коммита (канон CalendarDatePicker).
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent): void => {
      if (event.defaultPrevented || event.key !== 'Escape') {
        return;
      }
      onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [onClose]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Выбрать объект"
      className="fixed inset-0 z-50 flex flex-col bg-surface font-sans"
    >
      <TopNav
        leading={
          <IconButton icon={<Cancel />} label="Закрыть выбор объектов" onClick={onClose} />
        }
      >
        <TopNavTitle title="Выбрать объект" />
      </TopNav>

      {/* На десктопе TopNav зафиксирован над экраном (tablet:fixed) —
       * контент встаёт под ним на высоту шапки (канон CalendarDatePicker
       * #500); на мобайле шапка в потоке и отступ не нужен. */}
      <div className="min-h-0 flex-1 overflow-y-auto tablet:mt-[72px]">
        <div className="mx-auto w-full max-w-[560px] pb-[136px]">
          {/* Ряды с боковым отступом 24px по макету 2008-46627. */}
          <div className="flex flex-col px-6 pt-2">
            <InvitePropertiesRows
              options={options}
              selected={draft}
              allState={allState}
              onToggleAll={() => setDraft(toggleAllInvitedProperties(draft, optionIds))}
              onToggle={(propertyId) => setDraft(toggleInvitedProperty(draft, propertyId))}
            />
          </div>
        </div>
      </div>

      <StickyBottomBar>
        <Button
          className="w-full"
          disabled={draft.size === 0}
          onClick={() => onConfirm(draft)}
        >
          Выбрать
        </Button>
      </StickyBottomBar>
    </div>
  );
}
