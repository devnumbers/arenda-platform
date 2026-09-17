'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { Cancel, CheckBoxFalse, CheckBoxTrue, Minus } from '@/shared/assets/icons';
import { goBack } from '@/shared/lib/navigation';
import { ROUTES } from '@/shared/config/routes';
import {
  availableInviteProperties,
  type InvitePropertyOption,
  type ParticipantAccessRole,
} from '@/entities/participants';
import {
  useAddParticipantProperties,
  useParticipant,
} from '@/features/participants';
import { useProperties } from '@/features/properties';
import {
  Button,
  EmptyState,
  ErrorCard,
  IconButton,
  Modal,
  ModalContent,
  PageContent,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from '@/shared/ui/design';
import {
  ObjectAvatarGlyph,
  PARTICIPANT_ROW_BASE_CLASS,
  ParticipantNotFound,
} from './participant-fragments';
import { stageParticipantPopup } from '../lib/participant-popups';
import { ParticipantRoleSegmented } from './participant-role-segmented';
import { ParticipantInviteSkeleton } from './participants-skeletons';

/**
 * Экран «Пригласить в объект» (карта #692, тикет #698; Figma
 * 2010-131329/131721): закрытие крестиком, первая строка «Все объекты»
 * (tri-state чекбокс: выбор всех опций разом — снапшот текущих объектов,
 * решение чарта карты #692), разделитель и мультичек-ряды объектов
 * читающего. Список — manage-объекты читающего (где он владелец или
 * full_access — остальное бэк вернул бы skipped_unavailable) минус ноги,
 * уже выданные этому участнику (active/suspended/pending — повторный
 * грант даёт skipped_duplicate).
 *
 * Кнопка «Пригласить» открывает шит роли (сегмент «Просмотр |
 * Редактирование», кнопка «Пригласить» — макет 2010-131724) → POST
 * /participants/{id}/properties → назад на страницу участника с попапом
 * «Доступ выдан» (2010-131859, search-param). granted=0 при выбранных
 * объектах — инфра-аномалия параллельного отзыва: шит остаётся открыт
 * с ошибкой.
 */
export function ParticipantInviteScreen({
  participantId,
}: {
  readonly participantId: string;
}): JSX.Element {
  const router = useRouter();

  const participantQuery = useParticipant(participantId);
  const propertiesQuery = useProperties();
  const addProperties = useAddParticipantProperties(participantId);

  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [role, setRole] = useState<ParticipantAccessRole>('viewer');
  const [sheetOpen, setSheetOpen] = useState(false);
  const [grantError, setGrantError] = useState<string | null>(null);

  // access отсутствует у собственных объектов (владелец); viewer —
  // чужие объекты, где читающий не управляет выдачей: бэк вернул бы
  // skipped_unavailable, поэтому такие строки даже не показываем.
  const properties = propertiesQuery.data;
  const manageable = (properties ?? []).filter(
    (property) => property.access?.role !== 'viewer',
  );
  const participant = participantQuery.data;
  const options: InvitePropertyOption[] =
    participant === undefined
      ? []
      : availableInviteProperties(manageable, participant);

  const allChecked = options.length > 0 && selected.size === options.length;
  const mixed = selected.size > 0 && selected.size < options.length;
  const allState = mixed ? 'mixed' : allChecked ? 'on' : 'off';

  const toggleAll = (): void =>
    setSelected(allChecked ? new Set() : new Set(options.map((option) => option.id)));
  const toggleOne = (id: string): void =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });

  const close = (): void => {
    if (addProperties.isPending) {
      return;
    }
    // Отмена потока — возврат на источник по канону истории.
    goBack(router, ROUTES.participant(participantId));
  };

  const invite = (): void => {
    addProperties.mutate(
      { role, propertyIds: [...selected] },
      {
        onSuccess: (result) => {
          // Частичный исход (granted>0, skipped>0) макетом не различён:
          // попап один, повторная выдача на пропущенный объект безопасна
          // (бэк отвечает skipped_duplicate). Вопрос владельцу — в отчёте.
          if (result.granted === 0) {
            setGrantError('Не удалось выдать доступ — попробуйте еще раз');
            return;
          }
          stageParticipantPopup('granted');
          goBack(router, ROUTES.participant(participantId));
        },
        onError: () => setGrantError('Не удалось выдать доступ — попробуйте еще раз'),
      },
    );
  };

  let content: JSX.Element;
  if (participantQuery.isPending || propertiesQuery.isPending) {
    content = <ParticipantInviteSkeleton />;
  } else if (participantQuery.isError) {
    content =
      participantQuery.error.status === 404 ? (
        <ParticipantNotFound />
      ) : (
        <ErrorCard
          title="Не удалось загрузить участника"
          onRetry={() => void participantQuery.refetch()}
          className="mt-6"
        />
      );
  } else if (propertiesQuery.isError) {
    content = (
      <ErrorCard
        title="Не удалось загрузить объекты"
        onRetry={() => void propertiesQuery.refetch()}
        className="mt-6"
      />
    );
  } else if (options.length === 0) {
    content = (
      <EmptyState
        imageSrc="/images/tariff/tariff-about-sharing.png"
        title="Доступ уже открыт"
        description="У участника есть доступ ко всем объектам, которыми вы управляете"
      />
    );
  } else {
    content = (
      <div className="flex flex-col">
        <button
          type="button"
          role="checkbox"
          aria-checked={mixed ? 'mixed' : allChecked}
          onClick={toggleAll}
          className={PARTICIPANT_ROW_BASE_CLASS}
        >
          <ObjectAvatarGlyph photoUrl={undefined} isAll />
          <span className="flex min-w-0 flex-1 flex-col gap-1">
            <span className="truncate text-base font-medium leading-[18px] text-content">
              Все объекты
            </span>
            <span className="truncate text-sm leading-4 text-content-secondary">
              Поделиться всеми объектами
            </span>
          </span>
          <SelectionGlyph state={allState} />
        </button>
        <div aria-hidden className="mx-6 h-px bg-surface-muted" />
        {options.map((option) => (
          <button
            key={option.id}
            type="button"
            role="checkbox"
            aria-checked={selected.has(option.id)}
            onClick={() => toggleOne(option.id)}
            className={PARTICIPANT_ROW_BASE_CLASS}
          >
            <ObjectAvatarGlyph photoUrl={option.photoUrl} />
            <span className="flex min-w-0 flex-1 flex-col gap-1">
              <span className="truncate text-base font-medium leading-[18px] text-content">
                {option.name}
              </span>
              <span className="truncate text-sm leading-4 text-content-secondary">
                {option.address}
              </span>
            </span>
            <SelectionGlyph state={selected.has(option.id) ? 'on' : 'off'} />
          </button>
        ))}
      </div>
    );
  }

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<Cancel />}
            label="Закрыть приглашение"
            onClick={close}
          />
        }
      >
        <TopNavTitle title="Пригласить в объект" />
      </TopNav>

      {/* Ряды макета edge-to-edge живут в центральной колонке канона —
       * на мобайле это совпадает с макетом, на планшете/ПК ряды не
       * разъезжаются из-под шапки (канон PageContent 948:47567). */}
      <PageContent className="pt-4">{content}</PageContent>

      {options.length > 0 && (
        <StickyBottomBar>
          <Button
            className="w-full"
            disabled={selected.size === 0}
            onClick={() => {
              setGrantError(null);
              setSheetOpen(true);
            }}
          >
            Пригласить
          </Button>
        </StickyBottomBar>
      )}

      {/* Шит роли (макет 2010-131724): сегмент «Просмотр | Редактирование»
       * и кнопка «Пригласить». Заголовок — только для a11y. */}
      <Modal
        open={sheetOpen}
        onOpenChange={(open) => {
          if (!addProperties.isPending) {
            setSheetOpen(open);
          }
        }}
      >
        <ModalContent title="Роль доступа" titleSrOnly>
          <div className="flex flex-col gap-6">
            <ParticipantRoleSegmented
              value={role}
              disabled={addProperties.isPending}
              onChange={setRole}
            />
            {grantError !== null && (
              <p className="text-center text-sm leading-4 text-error" role="alert">
                {grantError}
              </p>
            )}
            <Button className="w-full" loading={addProperties.isPending} onClick={invite}>
              Пригласить
            </Button>
          </div>
        </ModalContent>
      </Modal>
    </>
  );
}

/** Пикторальный Selection Button (Figma 1031:21053; канон-иконки
 * CheckBoxTrue/False — решение владельца 07.09, вне канон-набора):
 * «mixed» («Все объекты» — выбрана часть, 1858:105670) — синий квадрат
 * радиусом 8 с белым Icon/R/Minus 16. Ряд несёт role="checkbox" —
 * глиф только рисует состояние (прецедент селектов задач/контактов). */
function SelectionGlyph({
  state,
}: {
  readonly state: 'on' | 'off' | 'mixed';
}): JSX.Element {
  if (state === 'mixed') {
    return (
      <span
        aria-hidden
        className="flex h-6 w-6 shrink-0 items-center justify-center rounded-lg bg-primary"
      >
        <Minus className="h-4 w-4 text-white" />
      </span>
    );
  }
  return state === 'on' ? (
    <CheckBoxTrue className="h-6 w-6 shrink-0" aria-hidden />
  ) : (
    <CheckBoxFalse className="h-6 w-6 shrink-0" aria-hidden />
  );
}
