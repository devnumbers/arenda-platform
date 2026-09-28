'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import Image from 'next/image';
import { useRouter } from 'next/navigation';
import { ArrowLeft, Calendar, Cancel } from '@/shared/assets/icons';
import { ROUTES } from '@/shared/config/routes';
import { goBack } from '@/shared/lib/navigation';
import { notify } from '@/shared/lib/notifications';
import type { IsoDate } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';
import {
  formatMoneyKopecks,
  kopecksToAmountInputString,
  parseRublesToKopecks,
} from '@/shared/lib/format-money';
import {
  buildRentalCompleteCommand,
  completePlannedEndDate,
  rentalCompletedTitle,
  useCompleteRental,
  useRentalSummary,
  RENTAL_COMMENT_MAX,
} from '@/features/rentals';
import type { Rental } from '@/entities/rental';
import {
  Button,
  CalendarDatePicker,
  ChipButton,
  IconButton,
  PageContent,
  StepsChip,
  StickyBottomBar,
  Textarea,
  TopNav,
  useTabBarSuppression,
} from '@/shared/ui/design';
import { MoneyField, PickerTriggerBox, WizardBottomBar } from './wizard-chrome';
import { RentalSummaryContent } from './rental-summary-content';
import { RentalSummarySkeleton } from './rental-skeletons';

/**
 * Поток мастера «Завершение аренды» (#534, клиентское состояние на одном
 * маршруте — прецедент визарда создания #530): подтверждение без номера
 * шага («Отмена»/«Продолжить»), шаг 1 — дата окончания (чипы «Сегодня» /
 * «По плану», границы [начало, сегодня] — ADR 0053 §3; пикер с allowPast —
 * дата завершения бывает задним числом, #805), шаг 2 — возврат залога (0
 * валиден — «не вернул», комментарий ≤2000), шаг 3 — итоги: на телефоне
 * (<561) сначала hero-экран с «Подвести итоги», обзор открывается по кнопке
 * (макеты 1433:60681 → 1433:61074); на планшете и десктопе (≥561) обзор
 * сразу (1433:61389/1433:61927). Финал — «Аренда объекта … завершена»
 * (1433:60975): «Хорошо» уводит из мастера, «Посмотреть аренду» — на экран
 * «Аренда» (завершённая аренда — материал «Прошлых аренд» #535).
 *
 * Сумма залога обязательна (кнопка шага скрыта до готовности — канон
 * 2026-09-05); в макете поле с плейсхолдером «Сумма» — канон арендных форм
 * (пустое поле пустое, решение владельца 2026-09-05) важнее буквы макета —
 * сверить на приёмке (#805: сверено, поле пустое).
 */

const COMPLETE_WIZARD_STEPS = 3;

type CompleteStage = 'confirm' | 'date' | 'deposit' | 'summary';

const STAGE_NUMBERS: Record<Exclude<CompleteStage, 'confirm'>, number> = {
  date: 1,
  deposit: 2,
  summary: 3,
};

export type RentalCompleteFlowProps = {
  readonly rental: Rental;
  readonly propertyName: string;
  readonly onClose: () => void;
};

export function RentalCompleteFlow({
  rental,
  propertyName,
  onClose,
}: RentalCompleteFlowProps): JSX.Element {
  const router = useRouter();
  const completeRental = useCompleteRental(rental.propertyId, rental.id);
  // Мастер — экран-поток: футер глушится на всех шагах.
  useTabBarSuppression();

  const [stage, setStage] = useState<CompleteStage>('confirm');
  const [completedDate, setCompletedDate] = useState<IsoDate | undefined>(undefined);
  const [depositRaw, setDepositRaw] = useState('');
  const [comment, setComment] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);
  // Шаг 3 на телефоне: hero → обзор; на ≥561 обзор виден сразу (CSS).
  const [summaryRevealed, setSummaryRevealed] = useState(false);
  const [completed, setCompleted] = useState<Rental | null>(null);

  const today = rental.today;
  const planEnd = completePlannedEndDate(rental);
  const depositKopecks = rental.depositKopecks;
  const depositAmount = parseRublesToKopecks(depositRaw);

  // Итоги запрашиваются только на шаге 3 — с выбранной датой (до этого
  // превью не нужно); пустой until держит запрос выключенным.
  const summaryQuery = useRentalSummary(
    rental.propertyId,
    rental.id,
    stage === 'summary' ? (completedDate ?? '') : '',
  );

  if (completed !== null) {
    return (
      <>
        {/* Шапка финала (Figma 1433:60985): крестик без заголовка и без
          «назад» — поток завершён. */}
      <TopNav
        leading={<IconButton icon={<Cancel />} label="Закрыть" onClick={finishFlow} />}
      />
        <PageContent>
          <div className="flex flex-col items-center gap-4 px-8 pt-16">
            {/* Иллюстрация финала 128 (Figma 1433:61073). */}
            <Image
              src="/images/rentals/complete-done.png"
              alt=""
              width={128}
              height={128}
              className="h-32 w-32"
              aria-hidden
            />
            <h1 className="text-center font-sans text-xl font-semibold leading-6 text-content">
              {rentalCompletedTitle(propertyName)}
            </h1>
          </div>
        </PageContent>
        <StickyBottomBar>
          <WizardBottomBar>
            <Button className="w-full" onClick={finishFlow}>
              Хорошо
            </Button>
            <Button variant="secondary" className="w-full" onClick={openRental}>
              Посмотреть аренду
            </Button>
          </WizardBottomBar>
        </StickyBottomBar>
      </>
    );
  }

  return (
    <>
      <TopNav
        leading={
          stage === 'confirm' ? (
            <IconButton icon={<Cancel />} label="Отменить завершение" onClick={onClose} />
          ) : (
            <IconButton icon={<ArrowLeft />} label="Назад" onClick={goToPreviousStage} />
          )
        }
        trailing={
          stage !== 'confirm' ? (
            <IconButton icon={<Cancel />} label="Закрыть" onClick={onClose} />
          ) : undefined
        }
      >
        {stage !== 'confirm' && (
          <StepsChip step={STAGE_NUMBERS[stage]} total={COMPLETE_WIZARD_STEPS} size="m" />
        )}
      </TopNav>

      <PageContent className="pt-0">
        {stage === 'confirm' && (
          <div className="flex flex-col items-center gap-8 px-12 pt-6">
            {/* Ключ аренды 96 — единая картинка (1232:61491). */}
            <Image
              src="/images/rentals/rental-hero.png"
              alt=""
              width={96}
              height={96}
              className="h-24 w-24"
              aria-hidden
            />
            <div className="flex flex-col items-center gap-2">
              <h1 className="text-center font-sans text-xl font-semibold leading-6 text-content">
                Завершить аренду?
              </h1>
              <p className="text-center text-base leading-[18px] text-content-secondary">
                Информацию об этой аренде можно будет посмотреть на странице прошлых аренд
              </p>
            </div>
          </div>
        )}

        {/* Панель подтверждения (Figma 1428:58303): «Отмена» и «Продолжить»
            рядом равными долями. */}
        {stage === 'confirm' && (
          <StickyBottomBar>
            <div className="flex gap-2">
              <Button variant="secondary" className="flex-1" onClick={onClose}>
                Отменить
              </Button>
              <Button className="flex-1" onClick={() => setStage('date')}>
                Продолжить
              </Button>
            </div>
          </StickyBottomBar>
        )}

        {stage === 'date' && (
          <>
            <div className="flex flex-col gap-8 px-6 pt-6">
              <div className="flex flex-col gap-2">
                <h1 className="font-sans text-xl font-semibold leading-6 text-content">
                  Дата окончания аренды
                </h1>
                <p className="text-base leading-[18px] text-content-secondary">
                  Аренда закончилась по плану или раньше?
                </p>
              </div>
              <div className="flex flex-col gap-3">
                <PickerTriggerBox
                  title="Окончание аренды"
                  value={
                    completedDate === undefined
                      ? undefined
                      : formatDayMonthWithYear(completedDate, today)
                  }
                  placeholder="Выбрать дату"
                  icon={<Calendar className="h-6 w-6" />}
                  onClick={() => setPickerOpen(true)}
                />
                <div className="flex flex-wrap gap-2">
                  <ChipButton onClick={() => setCompletedDate(today)}>Сегодня</ChipButton>
                  {planEnd !== undefined && (
                    <ChipButton onClick={() => setCompletedDate(planEnd)}>По плану</ChipButton>
                  )}
                </div>
              </div>
            </div>
            {completedDate !== undefined && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => setStage('deposit')}>
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}

        {stage === 'deposit' && (
          <>
            <div className="flex flex-col gap-8 px-6 pt-6">
              <div className="flex flex-col gap-2">
                <h1 className="font-sans text-xl font-semibold leading-6 text-content">
                  Возвращение залога
                </h1>
                <p className="text-base leading-[18px] text-content-secondary">
                  Выберите сумму, которая возвращается арендатору
                </p>
              </div>
              <div className="flex flex-col gap-3">
                <MoneyField
                  title="Залог"
                  raw={depositRaw}
                  onRawChange={setDepositRaw}
                  onClear={() => setDepositRaw('')}
                  ariaLabel="Сумма возврата залога"
                />
                <div className="flex flex-wrap gap-2">
                  {depositKopecks !== null && (
                    <ChipButton
                      onClick={() =>
                        setDepositRaw(kopecksToAmountInputString(depositKopecks))
                      }
                    >
                      {formatMoneyKopecks(depositKopecks)}
                    </ChipButton>
                  )}
                  <ChipButton onClick={() => setDepositRaw('0')}>
                    {formatMoneyKopecks(0)}
                  </ChipButton>
                </div>
              </div>
              <Textarea
                title="Комментарий"
                value={comment}
                onChange={(event) => setComment(event.target.value)}
                maxLength={RENTAL_COMMENT_MAX}
                aria-label="Комментарий к возврату залога"
              />
            </div>
            {depositAmount !== undefined && (
              <StickyBottomBar>
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => setStage('summary')}>
                    Продолжить
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
          </>
        )}

        {stage === 'summary' && (
          <>
            {/* Мобильный hero (Figma 1433:60681) — только <561 и до
                «Подвести итоги». */}
            {!summaryRevealed && (
              <div className="flex min-[561px]:hidden flex-col">
                <Image
                  src="/images/rentals/complete-hero.png"
                  alt=""
                  width={1024}
                  height={1024}
                  className="h-auto w-full"
                  aria-hidden
                />
                <div className="flex flex-col items-center gap-4 px-6 pt-6">
                  <h1 className="text-center font-sans text-[40px] font-semibold leading-[44px] text-content">
                    Подведем итоги аренды
                  </h1>
                  <p className="text-center text-base leading-[18px] text-content-secondary">
                    Итоги вашей аренды по объекту «{propertyName}»
                  </p>
                </div>
              </div>
            )}

            <div className={summaryRevealed ? 'block pt-6' : 'hidden min-[561px]:block'}>
              {/* Hero обзора на планшете и десктопе (Figma 1433:61927):
                  ключ аренды 128, заголовок 40/44. */}
              <div className="hidden min-[561px]:flex flex-col items-center gap-12 px-6 pb-12 pt-6">
                <Image
                  src="/images/rentals/rental-hero.png"
                  alt=""
                  width={128}
                  height={128}
                  className="h-32 w-32"
                  aria-hidden
                />
                <div className="flex flex-col items-center gap-4">
                  <h1 className="text-center font-sans text-[40px] font-semibold leading-[44px] text-content">
                    Подведем итоги аренды
                  </h1>
                  <p className="text-center text-base leading-[18px] text-content-secondary">
                    Итоги вашей аренды по объекту «{propertyName}»
                  </p>
                </div>
              </div>

              {summaryQuery.isPending && <RentalSummarySkeleton />}
              {summaryQuery.isError && (
                <div className="flex flex-col items-center gap-4 px-6 pt-6">
                  <p className="text-center text-base leading-[18px] text-content-secondary">
                    Не удалось загрузить итоги аренды
                  </p>
                  <Button
                    variant="secondary"
                    size="small"
                    onClick={() => void summaryQuery.refetch()}
                  >
                    Повторить
                  </Button>
                </div>
              )}
              {summaryQuery.isSuccess && completedDate !== undefined && (
                <RentalSummaryContent
                  rental={rental}
                  summary={summaryQuery.data}
                  endDate={completedDate}
                  propertyName={propertyName}
                  depositReturnKopecks={depositAmount ?? 0}
                  depositReturnComment={comment.trim()}
                />
              )}
            </div>

            {/* Панель hero-экрана — только <561; панель обзора — после
                «Подвести итоги» и на ≥561 всегда. */}
            {!summaryRevealed && (
              <StickyBottomBar className="min-[561px]:hidden">
                <WizardBottomBar>
                  <Button className="w-full" onClick={() => setSummaryRevealed(true)}>
                    Подвести итоги
                  </Button>
                </WizardBottomBar>
              </StickyBottomBar>
            )}
            <StickyBottomBar
              className={summaryRevealed ? undefined : 'hidden min-[561px]:block'}
            >
              <WizardBottomBar>
                <Button
                  className="w-full"
                  loading={completeRental.isPending}
                  onClick={() => void complete()}
                >
                  Завершить аренду
                </Button>
              </WizardBottomBar>
            </StickyBottomBar>
          </>
        )}
      </PageContent>

      {pickerOpen && (
        <CalendarDatePicker
          title="Дата окончания аренды"
          today={today}
          value={completedDate ?? null}
          required
          minDate={rental.startDate}
          maxDate={today}
          allowPast
          onClose={() => setPickerOpen(false)}
          onConfirm={(date) => {
            if (date !== null) {
              setCompletedDate(date);
            }
            setPickerOpen(false);
          }}
        />
      )}
    </>
  );

  function goToPreviousStage(): void {
    if (stage === 'summary') {
      setStage('deposit');
      return;
    }
    if (stage === 'deposit') {
      setStage('date');
      return;
    }
    setStage('confirm');
  }

  async function complete(): Promise<void> {
    const command = buildRentalCompleteCommand({
      completedDate,
      depositRaw,
      comment,
    });
    if (command === undefined) {
      return;
    }
    try {
      const updated = await completeRental.mutateAsync(command);
      setCompleted(updated);
    } catch (error: unknown) {
      notify.scenarios.rentals.completeError(error);
    }
  }

  function finishFlow(): void {
    // Поток завершён — история назад (прецедент «Хорошо» визарда #530).
    goBack(router, ROUTES.property(rental.propertyId));
  }

  function openRental(): void {
    // Поток завершён — маршрут мастера заменяется завершённой детализацией
    // (#535; CODING_STANDARDS, Navigation).
    router.replace(ROUTES.propertyRentalCompleted(rental.propertyId, rental.id));
  }
}

