"use client";

import type { JSX, PointerEvent as ReactPointerEvent } from "react";
import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ArrowLeft,
  Cancel,
  Edit,
  Move,
  Star,
  StarOff,
} from "@/shared/assets/icons";
import { ROUTES } from "@/shared/config/routes";
import {
  useGlobalPayments,
  useSaveFavoritesOrder,
  useSetPaymentFavorite,
} from "@/features/payments";
import type { GlobalPayment } from "@/entities/payment";
import { PaymentRowButton } from "@/entities/payment";
import {
  Button,
  ConfirmDialog,
  EmptyState,
  IconButton,
  Modal,
  ModalContent,
  PageContent,
  Skeleton,
  StatusIcon,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from "@/shared/ui/design";
import { goBack } from "@/shared/lib/navigation";
import { notify } from "@/shared/lib/notifications";
import {
  globalFavoritePayments,
  nearestDateLine,
} from "../lib/payments-global-model";
import {
  hasFavoritesEdits,
  moveFavorite,
  remainingFavoriteIds,
} from "../lib/favorites-edit-model";
import { GlobalPaymentRuleIcon, PaymentsStateCard } from "./payments-sections";

/**
 * Экран «Избранные платежи» (карта #573, тикет #579; вход — карточка
 * «Все избранные» главного экрана #578). Список (693:5546): строки канона
 * PaymentRowButton — иконка категории с красной точкой просрочки (как в
 * стопках), название, объект со звездой Icon/S/Star; справа сумма и
 * дата-«Ближайший» (просто дата графика). Тап строке — страница платежа;
 * тап звезде убирает из избранного на месте — строка исчезает. Иконка
 * правки в шапке открывает режим правки (693:5903): слева у строк
 * крест-звезда — пометка на удаление (синяя галочка на иконке категории,
 * 889:25528), справа ручка Move — ручной порядок (pointer-dnd, на десктопе
 * та же мышиная перетаскация), футер-кнопка «Сохранить». Сохранение с
 * помеченными просит канон ConfirmDialog (889:25522); после удаления —
 * попап успеха «Платежи больше не в избранном» (канон попапов StatusIcon,
 * как #533) и пустое состояние, если избранного не осталось (889:28111).
 * Кнопка StarOff в шапке правки из макета не воспроизведена: её поведение
 * не определимо из макета и не описано тикетом — вопрос владельцу.
 */
/** vaul не поднимает шит, смонтированный в одном батче с закрытием
 * другого (остаётся за нижним краем) — попап успеха ждёт выходной
 * анимации подтверждения (280ms) с запасом. */
const SUCCESS_POPUP_DELAY_MS = 450;

export function PaymentFavoritesScreen(): JSX.Element {
  const router = useRouter();
  const feedQuery = useGlobalPayments();

  // Скелетон — пока данных нет вовсе; ошибка без данных — карточка повтора
  // (канон состояний, как на главном экране платежей).
  const pending = feedQuery.data === undefined && !feedQuery.isError;

  const favorites = globalFavoritePayments(feedQuery.data?.items ?? []);

  const [editing, setEditing] = useState(false);
  // Черновик правки: порядок строк и пометки на удаление. initial —
  // порядок на момент входа в правку, от него считается дифф «Сохранить».
  const [initialOrder, setInitialOrder] = useState<
    ReadonlyArray<GlobalPayment>
  >([]);
  const [draftOrder, setDraftOrder] = useState<ReadonlyArray<GlobalPayment>>(
    [],
  );
  const [removedIds, setRemovedIds] = useState<ReadonlySet<string>>(new Set());
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [successShown, setSuccessShown] = useState(false);
  const [saving, setSaving] = useState(false);

  const setFavorite = useSetPaymentFavorite();
  const saveOrder = useSaveFavoritesOrder();

  const enterEdit = (): void => {
    setInitialOrder(favorites);
    setDraftOrder(favorites);
    setRemovedIds(new Set());
    setEditing(true);
  };

  const exitEdit = (): void => {
    setEditing(false);
    setDraftOrder([]);
    setRemovedIds(new Set());
  };

  const unfavorite = (payment: GlobalPayment): void => {
    void setFavorite
      .mutateAsync({
        propertyId: payment.propertyId,
        paymentId: payment.id,
        favorite: false,
      })
      .catch((error: unknown) =>
        notify.scenarios.payments.favoriteError(error),
      );
  };

  const save = async (): Promise<void> => {
    if (saving) {
      return;
    }
    setSaving(true);
    try {
      const marked = draftOrder.filter((item) => removedIds.has(item.id));
      await Promise.all(
        marked.map((item) =>
          setFavorite.mutateAsync({
            propertyId: item.propertyId,
            paymentId: item.id,
            favorite: false,
          }),
        ),
      );
      await saveOrder.mutateAsync(remainingFavoriteIds(draftOrder, removedIds));
      setConfirmOpen(false);
      exitEdit();
      if (marked.length > 0) {
        window.setTimeout(
          () => setSuccessShown(true),
          SUCCESS_POPUP_DELAY_MS,
        );
      }
    } catch (error) {
      notify.scenarios.payments.favoriteError(error);
    } finally {
      setSaving(false);
    }
  };

  const requestSave = (): void => {
    if (removedIds.size > 0) {
      setConfirmOpen(true);
    } else {
      void save();
    }
  };

  const listReady = !pending && !feedQuery.isError;

  return (
    <>
      <TopNav
        leading={
          <IconButton
            icon={<ArrowLeft />}
            label={editing ? "Выйти из правки" : "Назад"}
            onClick={() =>
              editing ? exitEdit() : goBack(router, ROUTES.payments)
            }
          />
        }
        trailing={
          listReady &&
          favorites.length > 0 &&
          !editing && (
            <IconButton
              icon={<Edit />}
              label="Изменить избранные"
              data-testid="favorites-edit-button"
              onClick={enterEdit}
            />
          )
        }
      >
        <TopNavTitle title="Избранные платежи" />
      </TopNav>

      <PageContent>
        <div data-testid="payments-favorites-screen" className="flex flex-col">
          {pending && (
            <div className="flex flex-col" aria-hidden>
              {Array.from({ length: 5 }, (_, index) => (
                <Skeleton key={index} className="mb-1 h-[60px] w-full" />
              ))}
            </div>
          )}

          {feedQuery.isError && (
            <PaymentsStateCard
              title="Не удалось загрузить избранное"
              hint="Проверьте подключение и попробуйте еще раз"
              action={
                <Button
                  variant="secondary"
                  size="small"
                  onClick={() => void feedQuery.refetch()}
                >
                  Повторить
                </Button>
              }
            />
          )}

          {listReady && favorites.length === 0 && <FavoritesEmpty />}

          {listReady && favorites.length > 0 && !editing && (
            <>
              {favorites.map((payment) => (
                <PaymentRowButton
                  key={payment.id}
                  className="px-6"
                  categoryIcon={<GlobalPaymentRuleIcon payment={payment} />}
                  title={payment.title}
                  subtitle={payment.propertyName}
                  subtitleSuffix={
                      <button
                        type="button"
                        data-testid={`favorites-row-star-${payment.id}`}
                        aria-label="Убрать из избранного"
                        disabled={
                          setFavorite.isPending &&
                          setFavorite.variables.paymentId === payment.id
                        }
                        onClick={(event) => {
                          event.stopPropagation();
                          unfavorite(payment);
                        }}
                        className="-m-2 cursor-pointer p-2 outline-none focus-visible:ring-2 focus-visible:ring-primary"
                      >
                        <Star className="h-4 w-4" aria-hidden />
                      </button>
                    }
                    amountKopecks={payment.amountKopecks}
                    description={nearestDateLine(payment)}
                    onSelect={() =>
                      router.push(
                        ROUTES.propertyPayment(payment.propertyId, payment.id),
                      )
                    }
                  />
              ))}
            </>
          )}

          {listReady && favorites.length > 0 && editing && (
            <FavoritesEditList
              draftOrder={draftOrder}
              removedIds={removedIds}
              onToggleRemoved={(id) => {
                setRemovedIds((previous) => {
                  const next = new Set(previous);
                  if (next.has(id)) {
                    next.delete(id);
                  } else {
                    next.add(id);
                  }
                  return next;
                });
              }}
              onMove={(from, to) =>
                setDraftOrder((previous) => moveFavorite(previous, from, to))
              }
            />
          )}
        </div>
      </PageContent>

      {editing && (
        // Кабинетная зона: BottomNav не глушится подавлением канонного
        // TabBar — панель поднимается над ним (как на выборщиках операций).
        <StickyBottomBar className="max-[1199px]:bottom-[calc(5rem_+_env(safe-area-inset-bottom))]">
          <Button
            data-testid="favorites-save"
            className="w-full"
            disabled={
              !hasFavoritesEdits(draftOrder, initialOrder, removedIds) || saving
            }
            loading={saving}
            onClick={requestSave}
          >
            Сохранить
          </Button>
        </StickyBottomBar>
      )}

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Убрать выбранные платежи из избранного?"
        description="Платежи можно будет снова добавить в избранное"
        confirmLabel="Убрать"
        cancelLabel="Отменить"
        pending={saving}
        onConfirm={() => void save()}
      />

      {/* Всегда смонтирован с open-пропом: vaul не поднимает шит,
          открытый при самом монтировании (условный рендер оставляет
          карточку за нижним краем). Важно: без className-пропов с
          position — twMerge перебил бы fixed шита. */}
      <Modal
        open={successShown}
        onOpenChange={(open) => open || setSuccessShown(false)}
      >
        <ModalContent title="Платежи больше не в избранном" titleSrOnly>
          <div className="absolute right-3 top-3 hidden desktop:block">
            <IconButton
              icon={<Cancel />}
              label="Закрыть"
              onClick={() => setSuccessShown(false)}
            />
          </div>
          <div className="flex flex-col items-center gap-2">
            <StatusIcon status="good" className="h-12 w-12" />
            <p className="text-center text-base font-medium leading-[18px] text-success">
              Платежи больше не в избранном
            </p>
          </div>
        </ModalContent>
      </Modal>
    </>
  );
}


/** Пустое состояние (889:28111): иллюстрация favorites-empty.png 128 и
 * две тёмные подписи — заголовок и подсказка (оба #171A1C в макете).
 * Центрируется в доступной высоте, как глобальное пустое главного экрана. */
function FavoritesEmpty(): JSX.Element {
  return (
    <div
      data-testid="favorites-empty"
      className="flex min-h-[calc(100dvh-216px)] flex-1 items-center justify-center"
    >
      <EmptyState
        imageSrc="/images/payments/favorites-empty.png"
        title="Вы пока не добавляли платежи в избранное"
        description="Добавьте платеж в избранное и он появится здесь"
        descriptionClassName="text-content"
      />
    </div>
  );
}

/** Список режима правки (693:5903): строки Edit — ведущая крест-звезда
 * (пометка на удаление), каноническая середина строки с галочкой на
 * иконке категории, хвостовая ручка Move. Строка в правке платеж не
 * открывает — вся строка управляет правкой. DnD: pointer-события ручки,
 * цель — ряд под курсором по его середине; touch-none держит палец от
 * прокрутки страницы (на десктопе тот же pointer-драг мышью). */
function FavoritesEditList({
  draftOrder,
  removedIds,
  onToggleRemoved,
  onMove,
}: {
  readonly draftOrder: ReadonlyArray<GlobalPayment>;
  readonly removedIds: ReadonlySet<string>;
  readonly onToggleRemoved: (id: string) => void;
  readonly onMove: (from: number, to: number) => void;
}): JSX.Element {
  const listRef = useRef<HTMLDivElement>(null);
  const dragIndex = useRef<number | null>(null);

  const handleMove = (event: ReactPointerEvent<HTMLButtonElement>): void => {
    const from = dragIndex.current;
    const list = listRef.current;
    if (from === null || list === null) {
      return;
    }
    const rows = Array.from(list.children) as HTMLElement[];
    let insertion = 0;
    for (const [index, row] of rows.entries()) {
      const rect = row.getBoundingClientRect();
      if (event.clientY > rect.top + rect.height / 2) {
        insertion = index + 1;
      }
    }
    let to = insertion;
    if (to > from) {
      to -= 1;
    }
    if (to !== from) {
      onMove(from, to);
      dragIndex.current = to;
    }
  };

  return (
    <div ref={listRef} className="flex flex-col">
      {draftOrder.map((payment, index) => {
        const marked = removedIds.has(payment.id);
        return (
          <PaymentRowButton
            key={payment.id}
            categoryIcon={<GlobalPaymentRuleIcon payment={payment} check={marked} />}
            title={payment.title}
            subtitle={payment.propertyName}
            amountKopecks={payment.amountKopecks}
            description={nearestDateLine(payment)}
            leading={
              <IconButton
                icon={<StarOff />}
                variant="secondary"
                label={marked ? "Снять пометку" : "Пометить на удаление"}
                aria-pressed={marked}
                data-testid={`favorites-mark-${payment.id}`}
                onClick={() => onToggleRemoved(payment.id)}
              />
            }
            trailing={
              // Ручка dnd — только указательный ввод, из a11y-дерева
              // исключена (клавиатурный порядок тикетом не заведён).
              <button
                type="button"
                aria-hidden
                tabIndex={-1}
                className="cursor-grab touch-none outline-none active:cursor-grabbing"
                onPointerDown={(event) => {
                  event.currentTarget.setPointerCapture(event.pointerId);
                  dragIndex.current = index;
                }}
                onPointerMove={handleMove}
                onPointerUp={() => {
                  dragIndex.current = null;
                }}
                onPointerCancel={() => {
                  dragIndex.current = null;
                }}
              >
                <Move className="h-6 w-6" aria-hidden />
              </button>
            }
          />
        );
      })}
    </div>
  );
}
