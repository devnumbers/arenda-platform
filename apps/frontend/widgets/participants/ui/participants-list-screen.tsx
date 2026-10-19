'use client';

import { useState, type JSX } from 'react';
import { useRouter } from 'next/navigation';
import { ArrowDown, Block, Kebab, TeamAdd, TimeHistory } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { useUrlParams } from '@/shared/lib/hooks/use-url-params';
import { isPaidTariff } from '@/entities/user';
import {
  ParticipantRowButton,
  sortParticipantsByName,
  type ParticipantSortOrder,
  sortOrderPickerGroups,
} from '@/entities/participants';
import {
  useParticipantsList,
  useRevokeAllParticipants,
} from '@/features/participants';
import { useSubscription } from '@/features/subscription';
import {
  Button,
  ChipButton,
  ConfirmDialog,
  EmptyState,
  ErrorCard,
  HubCollapseAnchor,
  HubTitle,
  IconButton,
  Menu,
  MenuContent,
  MenuItem,
  MenuTrigger,
  PageContent,
  PickerMenu,
  SearchPill,
  TopNav,
} from '@/shared/ui/design';
import { notify } from '@/shared/lib/notifications';
import {
  DEFAULT_PARTICIPANTS_LIST_ORDER,
  PARTICIPANTS_LIST_ORDER_PARAMS,
  serializeParticipantsListOrderToParams,
} from '../lib/participants-list-model';
import { ParticipantsListSkeleton } from './participants-list-skeletons';

/**
 * Раздел «Участники» — единая страница списка (D1–D4 #1313; Figma
 * 3313-77128): хаб-шапка нового хрома (#565, крылья + компакт-бар), ряд
 * заголовка с кебабом, пилюля поиска (канон SearchPill — тап открывает
 * отдельную поисковую страницу /participants/search, как у книги
 * контактов), чип-сортировка «Имя» (сервер приходит name ASC —
 * направление из меню применяется клиентски; выбор живёт в адресе
 * ?order=, дефолт «А→Я» не пишется — конвенция состояния в адресе,
 * #785) и ряды участников. Хаб-карточки и «Объекты пользователей»
 * снесены тем же тикетом: раздел — только свои приглашённые.
 *
 * Строка — имя + почта и ровно два статусных бейджа (D3): «Аккаунт не
 * создан» у pending и «Превышен лимит объектов» у suspended; счётчики
 * объектов и статусы «частично/все объекты» ушли с экрана.
 *
 * Кебаб раздела (макеты 3314-79166/3313-78243, D4): «Пригласить
 * участника» (`TeamAdd`), «История действий» (`TimeHistory` — вход в
 * общую ленту журнала) и красное «Отозвать доступ всем» (`Block`) —
 * только при непустом списке (на пустоте отзывать нечего, макет
 * 3314-79248 держит два пункта). На basic кебаб скрыт целиком (D2).
 *
 * «Отозвать доступ всем» — ConfirmDialog канона #629: крупный заголовок,
 * кнопки столбиком, «Отозвать и удалить» (danger). Один вызов глобального
 * DELETE /participants (#1310): все членства и pending-приглашения по
 * всем объектам, идемпотентно. Успех — тост «Все участники удалены»
 * (макет 2008-84101; тост вместо успех-попапа, решение владельца
 * 19.10.2026).
 *
 * Поиск — клиентская фильтрация на поисковой странице (объём мал —
 * тарифные слоты, список приходит целиком без пагинации; канон
 * серверного поиска #601 — про большие ленты).
 *
 * Тариф (D2): на basic раздел открыт пустым состоянием-апселлом
 * «Участников нет / Пригласите пользователей в свои объекты и управляйте
 * вместе» (макет 3315-79334) — список скрыт до оплаты и на пустой
 * истории подписки (downgrade с живыми участниками — то же состояние,
 * данные не пропадают), CTA «Пригласить» ведёт на интерстишал шаринга
 * #1470 (решение владельца 15.10: вход в тарифный гейт — интерстишал
 * «Совместный доступ недоступен», выбор тарифа — следующей кнопкой);
 * пилюля, чип и кебаб скрыты. Чтение списка на basic не вызывается —
 * эндпоинт за тарифным гейтом (ADR 0064). На платном тарифе та же
 * пустота (макет 3313-78626) — с CTA в форму приглашения.
 *
 * Тап по ряду — страница участника (#698).
 */
export function ParticipantsListScreen({
  initialOrder,
}: {
  readonly initialOrder?: ParticipantSortOrder;
}): JSX.Element {
  const router = useRouter();
  const { write } = useUrlParams();

  const [sortOrder, setSortOrder] = useState<ParticipantSortOrder>(
    initialOrder ?? DEFAULT_PARTICIPANTS_LIST_ORDER,
  );
  const [confirmOpen, setConfirmOpen] = useState(false);

  // Тариф решает, список или апселл (D2): null-подписка — базовый тариф
  // (канон paid-sections), сбой чтения подписки — платный фолбэк канона
  // детали объекта (деградация видима на списке, не молчит). Чтение
  // списка стартует только после разрешения подписки — обречённого 402
  // на basic холодного входа нет (ревизия ADR 0064 #1313).
  const subscriptionQuery = useSubscription();
  const isPaid = subscriptionQuery.data
    ? isPaidTariff(subscriptionQuery.data.tariff.name)
    : true;
  const subscriptionResolved =
    subscriptionQuery.isSuccess || subscriptionQuery.isError;

  const participantsQuery = useParticipantsList({
    enabled: subscriptionResolved && isPaid,
  });
  const revokeAll = useRevokeAllParticipants();

  const participants = participantsQuery.data ?? [];
  const listPending = isPaid && subscriptionResolved && participantsQuery.isPending;
  // Гард «данных нет» (§7, политика ошибки перечитывания #1422): ключ
  // /participants константный — провал перечитывания при удержанных
  // данных контент не подменяет, ErrorCard только первичному провалу.
  const listError = isPaid && participantsQuery.isError && participantsQuery.data === undefined;
  const hasParticipants = !listPending && !listError && participants.length > 0;
  // Пилюля поиска: с данными и в фазах загрузки (гейт подписки, сам
  // список) — контент встаёт на её место без сдвига (паритет чипа
  // сортировки, §7); на подтверждённой пустоте (#1004), апселле basic и
  // ошибке скрыта.
  const showSearch = hasParticipants || listPending || !subscriptionResolved;

  // Направление сортировки живёт в адресе (?order=, дефолт «А→Я» не
  // пишется — конвенция состояния в адресе, #785), переживает перезагрузку.
  const changeOrder = (order: ParticipantSortOrder): void => {
    setSortOrder(order);
    write(serializeParticipantsListOrderToParams(order), {
      own: PARTICIPANTS_LIST_ORDER_PARAMS,
    });
  };

  const confirmRevokeAll = (): void => {
    revokeAll.mutate(undefined, {
      onSuccess: () => {
        setConfirmOpen(false);
        notify.scenarios.access.allRevoked();
      },
    });
  };

  // Кебаб раздела — один узел в трёх слотах (ряд заголовка и хвост
  // компакт-бара; testid у обёрток — слоты рендерят узел дважды, общий
  // testid давал бы строгую неоднозначность в e2e). На basic и на ошибке
  // не рисуется вовсе (D2, §7).
  const kebab = (
    <Menu>
      <MenuTrigger asChild>
        <IconButton icon={<Kebab />} label="Еще — действия со списком" />
      </MenuTrigger>
      <MenuContent>
        <MenuItem
          icon={<TeamAdd className="h-6 w-6" />}
          onSelect={() => router.push(ROUTES.participantsInvite)}
        >
          Пригласить участника
        </MenuItem>
        <MenuItem
          icon={<TimeHistory className="h-6 w-6" />}
          onSelect={() => router.push(ROUTES.history)}
        >
          История действий
        </MenuItem>
        {/* «Отозвать доступ всем» — только там, где есть что отзывать
         * (макет 3314-79248: на пустоте меню без красного пункта). */}
        {hasParticipants && (
          <MenuItem
            icon={<Block className="h-6 w-6 text-error" />}
            className="text-error"
            onSelect={() => setConfirmOpen(true)}
          >
            Отозвать доступ всем
          </MenuItem>
        )}
      </MenuContent>
    </Menu>
  );

  const sortChip = (
    <PickerMenu title="Сортировать" groups={sortOrderPickerGroups('Имя', sortOrder, changeOrder)}>
      <ChipButton trailingIcon={<ArrowDown className={sortOrder === 'desc' ? 'rotate-180' : undefined} />}>
        Имя
      </ChipButton>
    </PickerMenu>
  );

  return (
    <>
      <TopNav
        mobileWings
        collapse={{
          title: 'Участники',
          search: showSearch
            ? { href: ROUTES.participantsSearch, label: 'Найти участника' }
            : undefined,
          trailing: isPaid && !listError ? (
            <span data-testid="participants-section-kebab-compact">{kebab}</span>
          ) : undefined,
        }}
      />

      <PageContent>
        <HubCollapseAnchor>
          {/* Строка заголовка h-8 (паттерн хабов «Контактов»/«Объектов»):
           * кебаб справа, на basic и на ошибке скрыт (§7, D2). */}
          <div className="flex h-8 items-center justify-between pr-3.5">
            <HubTitle>Участники</HubTitle>
            {isPaid && !listError && (
              <span data-testid="participants-section-kebab-row">{kebab}</span>
            )}
          </div>

          {/* Пилюля поиска — канон SearchPill; видима с данными и в фазе
           * загрузки, на подтверждённой пустоте и апселле прячется
           * (#1004, §7: искать нечего). */}
          {showSearch && (
            <div className="mt-4 px-6">
              <SearchPill
                testId="participants-search-pill"
                label="Найти участника"
                onOpenSearch={() => router.push(ROUTES.participantsSearch)}
              />
            </div>
          )}
        </HubCollapseAnchor>

        {!subscriptionResolved ? (
          /* Подписка решает, список или апселл (D2) — до её разрешения
           * нейтральный скелетон, без флэша пустоты (гейт чтения списка —
           * ADR 0064 ревизия #1313). */
          <>
            <div className="mb-6 mt-6 px-6">{sortChip}</div>
            <ParticipantsListSkeleton />
          </>
        ) : listPending ? (
          <>
            {/* Чип сортировки реальный — вне фазы загрузки (§7, прецедент
             * книги контактов): контент встаёт на его место без сдвига. */}
            <div className="mb-6 mt-6 px-6">{sortChip}</div>
            <ParticipantsListSkeleton />
          </>
        ) : !isPaid ? (
          /* Апселл базового тарифа (D2, макет 3315-79334): список скрыт до
           * оплаты — и на пустом шаринге, и после downgrade с живыми
           * участниками; кебаба и пилюли нет. CTA — на интерстишал
           * шаринга #1470 (решение владельца 15.10), не сразу на выбор
           * тарифа. */
          <SectionEmptyState
            ctaTestId="participants-upsell-cta"
            onCta={() => router.push(ROUTES.profileTariffSharing)}
          />
        ) : listError ? (
          <ErrorCard
            title="Не удалось загрузить участников"
            onRetry={() => void participantsQuery.refetch()}
            loading={participantsQuery.isFetching}
            className="mt-6"
          />
        ) : participants.length === 0 ? (
          /* Пустой список платного тарифа (макет 3313-78626): та же пустота,
           * что у апселла, но приглашение ведёт в форму, а кебаб раздела
           * остаётся (без красного пункта). */
          <SectionEmptyState
            ctaTestId="participants-empty-cta"
            onCta={() => router.push(ROUTES.participantsInvite)}
          />
        ) : (
          <>
            {/* Ритм первого экрана хаба (макет 3225-77283, решение
             * владельца 14.10.2026): поиск → чипы 24, чипы → контент 24. */}
            <div className="mb-6 mt-6 px-6">{sortChip}</div>
            <div className="flex flex-col px-6" data-testid="participants-list">
              {sortParticipantsByName(participants, sortOrder).map((participant) => (
                <ParticipantRowButton
                  key={participant.id}
                  participant={participant}
                  onSelect={() => router.push(ROUTES.participant(participant.id))}
                />
              ))}
            </div>
          </>
        )}
      </PageContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Отозвать доступ всем пользователям к вашим объектам?"
        titleClassName="text-[28px] leading-8"
        description="Пользователи потеряют доступ ко всем вашим объектам и будут удалены из списка участников. Пригласить их можно будет снова"
        confirmLabel="Отозвать и удалить"
        cancelLabel="Отмена"
        confirmVariant="danger"
        stacked
        pending={revokeAll.isPending}
        onConfirm={confirmRevokeAll}
      />
    </>
  );
}

/** Пустота раздела (D2 #1313, макеты 3313-78626/3315-79334): иллюстрация,
 * «Участников нет / Пригласите пользователей в свои объекты и управляйте
 * вместе» и CTA «Пригласить» — Button size="small" radius="m", канон CTA
 * пустых состояний (анатомия 3235-74057). Одна анатомия у обоих состояний —
 * апселл basic и пустой список платного тарифа различаются только адресатом
 * CTA (оформление тарифа или форма приглашения). */
function SectionEmptyState({
  ctaTestId,
  onCta,
}: {
  readonly ctaTestId: string;
  readonly onCta: () => void;
}): JSX.Element {
  return (
    <EmptyState
      imageSrc="/images/tariff/tariff-about-sharing.png"
      title="Участников нет"
      description="Пригласите пользователей в свои объекты и управляйте вместе"
      action={
        <Button size="small" radius="m" data-testid={ctaTestId} onClick={onCta}>
          Пригласить
        </Button>
      }
    />
  );
}
