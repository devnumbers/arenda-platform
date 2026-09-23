"use client";

import type { JSX, KeyboardEvent as ReactKeyboardEvent } from "react";
import { useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Reorder, useDragControls, useReducedMotion } from "framer-motion";
import {
  ArrowLeft,
  Cancel,
  Edit,
  Move,
  Star,
  TrashBin,
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
  StatusIcon,
  StickyBottomBar,
  TopNav,
  TopNavTitle,
} from "@/shared/ui/design";
import { cn } from "@/shared/lib/cn";
import { goBack } from "@/shared/lib/navigation";
import { notify } from "@/shared/lib/notifications";
import { useLongPress } from "@/shared/lib/hooks/useLongPress";
import {
  globalFavoritePayments,
  nearestDateLine,
} from "../lib/payments-global-model";
import {
  favoriteDragAnnouncement,
  type FavoriteDragAnnouncementKind,
  hasFavoritesEdits,
  moveFavorite,
  selectFavoriteSelection,
  selectedFavoritesTitle,
  toggleFavoriteSelection,
} from "../lib/favorites-edit-model";
import { GlobalPaymentRuleIcon, PaymentsRowsSkeleton, PaymentsStateCard } from "./payments-sections";

/**
 * Экран «Избранные платежи» (карта #573, тикет #579; вход — карточка
 * «Все избранные» главного экрана #578). Список: строки канона
 * PaymentRowButton — иконка категории с красной точкой просрочки (как в
 * стопках), название, объект со звездой Icon/S/Star перед именем
 * (954-52460); справа сумма и дата-«Ближайший» (просто дата графика).
 * Плашка просмотра и плашка правки анатомически идентичны (954-52460 =
 * 954-52219) — иначе список дёргается при входе в правку. Тап строке —
 * страница платежа; тап звезде убирает из избранного на месте — строка
 * исчезает. Иконка
 * правки (693:5903): строки правки — каноническая середина с галочкой на
 * иконке категории (889:25528) и ручка Move справа — плавное
 * перетаскивание (карта #811, тикет #813: плашка следует за пальцем/
 * курсором за ручку, лифт-эффект, соседи пружинят, автоскролл у краёв —
 * framer-motion Reorder; клавиатурный порядок — свой слой на ручке:
 * пробел берёт, стрелки перемещают, Escape отпускает). Выделение для
 * удаления (#814, решения владельца 23.09, макет 954-51409): iOS-стиль —
 * long-press плашки на таче выделяет её и включает режим выбора, дальше
 * одиночные тапы переключают; на десктопе клик = выделить (зажатие —
 * только тач-жест); при выборе заголовок → «Выбрано N», trash в навбаре —
 * на mobile так же; удаление выбранных — канон ConfirmDialog и попап
 * успеха «Платежи больше не в избранном» (StatusIcon, как #533), пустое
 * состояние, если избранного не осталось (889:28111). Старые StarOff-
 * пометки строк (#579) снесены. «Сохранить» применяет только порядок.
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
  // Черновик правки: порядок строк и выделенные на удаление (#814).
  // initial — порядок на момент входа в правку, от него считается дифф
  // «Сохранить».
  const [initialOrder, setInitialOrder] = useState<
    ReadonlyArray<GlobalPayment>
  >([]);
  const [draftOrder, setDraftOrder] = useState<ReadonlyArray<GlobalPayment>>(
    [],
  );
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(
    new Set(),
  );
  // Режим выбора на таче включается первым long-press'ом: до него тапы
  // строк ничего не делают, после — переключают выделение. На десктопе
  // клик выделяет всегда, флаг не участвует.
  const [selectionArmed, setSelectionArmed] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [successShown, setSuccessShown] = useState(false);
  const [saving, setSaving] = useState(false);

  const setFavorite = useSetPaymentFavorite();
  const saveOrder = useSaveFavoritesOrder();

  const enterEdit = (): void => {
    setInitialOrder(favorites);
    setDraftOrder(favorites);
    setSelectedIds(new Set());
    setSelectionArmed(false);
    setEditing(true);
  };

  const exitEdit = (): void => {
    setEditing(false);
    setDraftOrder([]);
    setSelectedIds(new Set());
    setSelectionArmed(false);
  };

  const toggleSelected = (id: string): void => {
    setSelectedIds((previous) => toggleFavoriteSelection(previous, id));
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

  /** «Сохранить» применяет только порядок (#814: удаление живёт на
   * trash'е навбара и не ждёт сохранения). */
  const save = async (): Promise<void> => {
    if (saving) {
      return;
    }
    setSaving(true);
    try {
      await saveOrder.mutateAsync(
        draftOrder.map((item) => item.id),
      );
      exitEdit();
    } catch (error) {
      notify.scenarios.payments.favoriteError(error);
    } finally {
      setSaving(false);
    }
  };

  /** Trash навбара: убирает выбранные из избранного и сохраняет порядок
   * оставшихся (нарушенный порядок тоже применяется — по контракту PUT
   * это полное замещение). Правка продолжается; без оставшихся — выход.
   * Попап успеха ждёт выходной анимации диалога (vaul-грабля выше). */
  const deleteSelected = async (): Promise<void> => {
    if (saving) {
      return;
    }
    setSaving(true);
    try {
      const selected = draftOrder.filter((item) => selectedIds.has(item.id));
      await Promise.all(
        selected.map((item) =>
          setFavorite.mutateAsync({
            propertyId: item.propertyId,
            paymentId: item.id,
            favorite: false,
          }),
        ),
      );
      const remaining = draftOrder.filter(
        (item) => !selectedIds.has(item.id),
      );
      await saveOrder.mutateAsync(remaining.map((item) => item.id));
      setConfirmOpen(false);
      setSelectedIds(new Set());
      setSelectionArmed(false);
      if (remaining.length === 0) {
        exitEdit();
      } else {
        // Правка продолжается: черновик и его initial — оставшиеся в
        // сохранённом порядке, «Сохранить» гаснет до следующей перестановки.
        setInitialOrder(remaining);
        setDraftOrder(remaining);
      }
      window.setTimeout(() => setSuccessShown(true), SUCCESS_POPUP_DELAY_MS);
    } catch (error) {
      notify.scenarios.payments.favoriteError(error);
    } finally {
      setSaving(false);
    }
  };

  const listReady = !pending && !feedQuery.isError;
  const selectedCount = selectedIds.size;

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
          (editing ? (
            selectedCount > 0 && (
              <IconButton
                icon={<TrashBin />}
                label="Убрать выбранные из избранного"
                data-testid="favorites-trash"
                onClick={() => setConfirmOpen(true)}
              />
            )
          ) : (
            <IconButton
              icon={<Edit />}
              label="Изменить избранные"
              data-testid="favorites-edit-button"
              onClick={enterEdit}
            />
          ))
        }
      >
        <TopNavTitle
          title={
            editing && selectedCount > 0
              ? selectedFavoritesTitle(selectedCount)
              : "Избранные платежи"
          }
        />
      </TopNav>

      <PageContent>
        <div data-testid="payments-favorites-screen" className="flex flex-col">
          {pending && (
            // Паритет §7: строки канона PaymentRowButton (иконка, название,
            // объект, сумма и дата) — контент встаёт на место без сдвига.
            <PaymentsRowsSkeleton />
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
                  // Подзаголовок по макету 954-52461: звезда Icon/S/Star
                  // 16 — первый элемент второй строки, имя объекта за ней.
                  // Кнопка идёт через subtitleIcon — тот же слот, что у
                  // строки правки ниже, иначе режимы расходятся анатомией
                  // и контент прыгает при входе в правку. Звезда здесь —
                  // тап «убрать из избранного» (#579); хит-зона 32 через
                  // before:-inset-2 — отрицательные margin на голой кнопке
                  // перебивает preflight (кнопочная грабля).
                  subtitleIcon={
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
                      className={cn(
                        "relative h-4 w-4 cursor-pointer outline-none",
                        "focus-visible:ring-2 focus-visible:ring-primary",
                        "before:absolute before:-inset-2",
                      )}
                    >
                      <Star className="h-4 w-4" aria-hidden />
                    </button>
                  }
                  subtitle={payment.propertyName}
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
              selectedIds={selectedIds}
              selectionArmed={selectionArmed}
              onToggleSelected={toggleSelected}
              onSelectOnly={(id) =>
                setSelectedIds((previous) =>
                  selectFavoriteSelection(previous, id),
                )
              }
              onArmSelection={() => setSelectionArmed(true)}
              onReorder={(next) => setDraftOrder(next)}
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
            disabled={!hasFavoritesEdits(draftOrder, initialOrder) || saving}
            loading={saving}
            onClick={() => void save()}
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
        onConfirm={() => void deleteSelected()}
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

/** Список режима правки (693:5903): строки Edit — каноническая середина
 * с галочкой на иконке категории у выделенных (#814), хвостовая ручка
 * Move. Строка в правке платеж не открывает — вся строка управляет
 * правкой. DnD (#813) — framer-motion Reorder: жест живёт только на
 * ручке (dragListener={false} + useDragControls), плашка тащится
 * трансформом за указателем 1:1, соседи пружинят layout-анимацией,
 * автоскролл у краёв встроен. Лифт — scale и тень единым motion-стейтом:
 * whileDrag (указательный жест) и animate (клавиатурный захват) дают один
 * и тот же подъём. Клавиатурный порядок (#813): ручка — фокусируемая
 * кнопка, пробел или Enter берёт/отпускает, стрелки перемещают, Escape
 * отпускает (уход фокуса тоже); анонсы — в sr-only aria-live.
 * Движение — по кривой дома (DESIGN.md §8): 350ms, reduced-motion —
 * 150ms, не отключается. */
const EDIT_ROW_LIFT_SCALE = 1.02;
/** Лифт-тень взятой плашки: та же структура строки, что у «нет тени» —
 * motion интерполирует бокс-шэдоу только между схожими значениями. */
const EDIT_ROW_SHADOW = "0px 8px 24px rgba(23,26,28,0.16)";
const EDIT_ROW_NO_SHADOW = "0px 0px 0px rgba(23,26,28,0)";
/** Лифт (scale+тень) и пружина соседей — движение/трансформы: 350ms;
 * reduced-motion — 150ms (DESIGN.md §8). */
const EDIT_ROW_LIFT_S = 0.35;
const EDIT_ROW_REDUCED_S = 0.15;
/** cubic-bezier(0.32, 0.72, 0, 1) — единая кривая дома (--dl-ease). */
const EDIT_ROW_EASE: [number, number, number, number] = [0.32, 0.72, 0, 1];

function FavoritesEditList({
  draftOrder,
  selectedIds,
  selectionArmed,
  onToggleSelected,
  onSelectOnly,
  onArmSelection,
  onReorder,
  onMove,
}: {
  readonly draftOrder: ReadonlyArray<GlobalPayment>;
  readonly selectedIds: ReadonlySet<string>;
  readonly selectionArmed: boolean;
  readonly onToggleSelected: (id: string) => void;
  readonly onSelectOnly: (id: string) => void;
  readonly onArmSelection: () => void;
  readonly onReorder: (next: ReadonlyArray<GlobalPayment>) => void;
  readonly onMove: (from: number, to: number) => void;
}): JSX.Element {
  const [grabbedId, setGrabbedId] = useState<string | null>(null);
  const [announce, setAnnounce] = useState("");
  const reducedMotion = useReducedMotion() ?? false;

  const announceDrag = (
    kind: FavoriteDragAnnouncementKind,
    payment: GlobalPayment,
    position: number,
  ): void => {
    setAnnounce(
      favoriteDragAnnouncement(
        kind,
        payment.title,
        position,
        draftOrder.length,
      ),
    );
  };

  return (
    <>
      {/* values — копия черновика: Group мутирует массив при подсчёте
          следующего порядка, ReadonlyArray отдаём как новый список. */}
      <Reorder.Group
        as="ul"
        axis="y"
        className="flex flex-col"
        values={[...draftOrder]}
        onReorder={onReorder}
      >
        {draftOrder.map((payment, index) => (
          <FavoritesEditRow
            key={payment.id}
            payment={payment}
            index={index}
            total={draftOrder.length}
            selected={selectedIds.has(payment.id)}
            selectionArmed={selectionArmed}
            grabbed={grabbedId === payment.id}
            reducedMotion={reducedMotion}
            onToggleSelected={() => onToggleSelected(payment.id)}
            onSelectOnly={() => onSelectOnly(payment.id)}
            onArmSelection={onArmSelection}
            onGrabChange={(grabbed) => {
              setGrabbedId(grabbed ? payment.id : null);
              announceDrag(grabbed ? "grab" : "release", payment, index + 1);
            }}
            onMoved={(to) => {
              onMove(index, to);
              announceDrag("move", payment, to + 1);
            }}
          />
        ))}
      </Reorder.Group>
      <p className="sr-only" aria-live="polite" data-testid="favorites-dnd-live">
        {announce}
      </p>
    </>
  );
}

/** Строка-плашка списка правки: обёртка Reorder.Item над каноном
 * PaymentRowButton. Взятой (рука или клавиатура) — лифт: scale 1.02
 * плюс тень. release-возврат — теми же таймингами. Выделение (#814):
 * вся строка — переключатель (Enter/Space тоже), long-press на таче
 * выделяет и включает режим выбора; пометка видна галочкой на иконке
 * категории (889:25528). */
function FavoritesEditRow({
  payment,
  index,
  total,
  selected,
  selectionArmed,
  grabbed,
  reducedMotion,
  onToggleSelected,
  onSelectOnly,
  onArmSelection,
  onGrabChange,
  onMoved,
}: {
  readonly payment: GlobalPayment;
  readonly index: number;
  readonly total: number;
  readonly selected: boolean;
  readonly selectionArmed: boolean;
  readonly grabbed: boolean;
  readonly reducedMotion: boolean;
  readonly onToggleSelected: () => void;
  readonly onSelectOnly: () => void;
  readonly onArmSelection: () => void;
  readonly onGrabChange: (grabbed: boolean) => void;
  readonly onMoved: (to: number) => void;
}): JSX.Element {
  const controls = useDragControls();
  // Клик-хвост жеста ручки: drag завершается поднятой над строкой
  // указателем — браузер отдаёт click контейнеру строки, и без флага он
  // мусорно переключает выделение. Флаг ставит pointerdown ручки, снимают
  // capture-фазы строки (следующий pointerdown/keydown любого жеста).
  const gripGestureRef = useRef(false);
  const {
    handlers: longPressHandlers,
    isTouchPointer,
    consumeClickAfterLongPress,
  } = useLongPress({
    // Зажатие выделяет и включает режим выбора; само никогда не
    // развыделяет — переключение остаётся за тапами.
    onLongPress: () => {
      onArmSelection();
      onSelectOnly();
    },
    // Жест зажатия не начинается на ручке dnd — там свой pointer-жест.
    skipOn: (event) =>
      event.target instanceof Element &&
      event.target.closest("[data-grip]") !== null,
  });

  const activate = (): void => {
    if (gripGestureRef.current) {
      gripGestureRef.current = false;
      return;
    }
    if (consumeClickAfterLongPress()) {
      return;
    }
    // До первого зажатия тач-тап ничего не делает (вход в выбор —
    // только зажатием); десктоп-клик и клавиатура выделяют всегда.
    if (isTouchPointer() && !selectionArmed) {
      return;
    }
    onToggleSelected();
  };

  const duration = reducedMotion ? EDIT_ROW_REDUCED_S : EDIT_ROW_LIFT_S;
  // Единый лифт для тача и клавиатуры: whileDrag и animate дают один
  // и тот же подъём — scale плюс тень.
  const lift = {
    scale: EDIT_ROW_LIFT_SCALE,
    boxShadow: EDIT_ROW_SHADOW,
  };
  const rest = {
    scale: 1,
    boxShadow: EDIT_ROW_NO_SHADOW,
  };

  const handleKeyDown = (
    event: ReactKeyboardEvent<HTMLButtonElement>,
  ): void => {
    if (event.key === " " || event.key === "Enter") {
      event.preventDefault();
      onGrabChange(!grabbed);
      return;
    }
    if (!grabbed) {
      return;
    }
    if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      event.preventDefault();
      const to = event.key === "ArrowUp" ? index - 1 : index + 1;
      if (to >= 0 && to < total) {
        onMoved(to);
      }
      return;
    }
    if (event.key === "Escape") {
      event.preventDefault();
      onGrabChange(false);
    }
  };

  return (
    <Reorder.Item
      value={payment}
      dragListener={false}
      dragControls={controls}
      whileDrag={lift}
      animate={grabbed ? lift : rest}
      transition={{
        duration,
        ease: EDIT_ROW_EASE,
      }}
      data-testid={`favorites-edit-row-${payment.id}`}
      // Плашка правки не выделяет текст и не зовёт системное меню по
      // зажатию (iOS callout, Android selection) — зажатие служит
      // выделению; на десктопе нативное контекстное меню не трогаем.
      className="select-none [-webkit-touch-callout:none]"
      onContextMenu={(event) => {
        if (isTouchPointer()) {
          event.preventDefault();
        }
      }}
      onPointerDownCapture={() => {
        gripGestureRef.current = false;
      }}
      onKeyDownCapture={() => {
        gripGestureRef.current = false;
      }}
      {...longPressHandlers}
    >
      <PaymentRowButton
        // relative — система координат absolute-ручки: ручка выведена из
        // потока в правый паддинг строки, контентная область остаётся
        // шириной с просмотр и сумма с датой не смещаются при входе в
        // правку (954-52461; в макете правки 954-52219 ручка сжимает
        // контент на ширину слота — владелец: смещения быть не должно).
        className="relative px-6"
        categoryIcon={
          <GlobalPaymentRuleIcon payment={payment} check={selected} />
        }
        title={payment.title}
        // Плашка правки = плашка просмотра (954-52219 = 954-52461):
        // та же звезда Icon/S/Star первым элементом второй строки, иначе
        // контент прыгает при входе в правку.
        subtitleIcon={<Star className="h-4 w-4" aria-hidden />}
        subtitle={payment.propertyName}
        amountKopecks={payment.amountKopecks}
        description={nearestDateLine(payment)}
        pressed={selected}
        onSelect={activate}
        trailing={
          // Ручка dnd (#813): pointer-жест стартует только здесь, из
          // клавиатуры — пробел или Enter берёт, стрелки перемещают,
          // Escape/повторный пробел отпускает; уход фокуса с взятой
          // ручки отпускает строку (плашка не зависает поднятой);
          // анонсы позиции — живым регионом списка. Жест ручки ставит
          // флаг gripGestureRef (клик-хвост drag'а строка съедает), а
          // stopPropagation страхует клавиатурные клики ручки.
          // absolute (по центру правого паддинга px-6) — вне потока:
          // обёртка trailing-слота схлопывается в нулевую ширину и не
          // сдвигает сумму с датой.
          <button
            type="button"
            aria-label={`Переместить: ${payment.title}`}
            data-testid={`favorites-grip-${payment.id}`}
            // Прод-маркер ручки: гейт «зажатие не с плашки» цепляется
            // за него, а не за тестовый атрибут.
            data-grip
            className={cn(
              "absolute right-1.5 top-1/2 -translate-y-1/2",
              "cursor-grab touch-none rounded-sm text-content-tertiary outline-none",
              "focus-visible:ring-2 focus-visible:ring-primary",
              "active:cursor-grabbing",
            )}
            onClick={(event) => event.stopPropagation()}
            onPointerDown={(event) => {
              gripGestureRef.current = true;
              controls.start(event);
            }}
            onKeyDown={handleKeyDown}
            onBlur={() => {
              if (grabbed) {
                onGrabChange(false);
              }
            }}
          >
            <Move className="h-6 w-6" aria-hidden />
          </button>
        }
      />
    </Reorder.Item>
  );
}
