'use client';

import type { JSX } from 'react';
import {
    EmptyState,
    InfiniteQueryHead,
    InfiniteQueryTail,
    ResendCodeTile,
    Skeleton,
    SkeletonButton,
    SkeletonCard,
    SkeletonFormField,
    SkeletonListRow,
    SkeletonMedia,
    SkeletonSection,
} from '@/shared/ui/design';
import styles from '../../page.module.css';

/** Заглушка запроса для витрины хвоста: фаза «едет следующая порция» —
 * индикатор виден постоянно, fetchNextPage никуда не ходит. */
const STUB_FETCHING_QUERY = {
    hasNextPage: true,
    isFetchingNextPage: true,
    fetchNextPage: () => Promise.resolve(),
} as const;

/** Холостая фаза той же ленты: продолжение есть (sentinel в DOM), порция
 * не едет — хвост безмолвен, пока sentinel не войдёт во вьюпорт. */
const STUB_IDLE_QUERY = {
    hasNextPage: true,
    isFetchingNextPage: false,
    fetchNextPage: () => Promise.resolve(),
} as const;

export function EmptyStateSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>EmptyState · пустое состояние</h3>
            <p className={styles.groupTitle}>
                Иллюстрация 128 + заголовок + серое описание (максимум 360) и действие.
                Полноэкранные «пусто» — задачи, контакты, операции (Figma 1535-75363,
                1527:74479, 1510-77308).
            </p>
            <div className={styles.grid}>
                <EmptyState
                    imageSrc="/images/contacts/empty-contacts.png"
                    title="Контактов нет"
                    description="Добавьте контакты арендатора, мастеров и других специалистов"
                />
            </div>
        </div>
    );
}

export function ResendCodeTileSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>ResendCodeTile · повторная отправка кода</h3>
            <p className={styles.groupTitle}>
                Resend-канон шага кода (#733, Figma 1869-68137/2343-51004): Secondary-кнопка
                «Отправить новый код», на таймере disabled с подписью ММ:СС (Roboto Mono),
                по истечении подпись скрыта. Таймер-хранение — за фичей (useCountdown).
            </p>
            <div className={styles.grid}>
                <div className="flex w-full flex-col gap-6">
                    <ResendCodeTile remainingSeconds={42} onResend={() => {}} />
                    <ResendCodeTile remainingSeconds={0} onResend={() => {}} />
                </div>
            </div>
        </div>
    );
}

export function SkeletonShowcaseSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>Skeleton · скелетон загрузки</h3>
            <p className={styles.groupTitle}>
                Пульс на bg-surface-muted, размер и форма — через className; внутри
                серой карточки — bg-surface-muted-hover.
            </p>
            <div className={styles.grid}>
                <div className="flex w-full flex-col gap-2">
                    <Skeleton className="h-11 w-3/5 bg-surface-muted-hover" />
                    <Skeleton className="h-11 w-4/5 bg-surface-muted-hover" />
                    <Skeleton className="h-11 w-2/5 bg-surface-muted-hover" />
                </div>
            </div>
        </div>
    );
}

export function SkeletonPrimitivesSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>
                Skeleton-примитивы · составные заглушки загрузки
            </h3>
            <p className={styles.groupTitle}>
                Композиции канона Skeleton под анатомию реальных блоков (#604,
                паритет — §7 DESIGN.md): контент занимает место скелетона без
                сдвига. API финализирован на хабах карты #603 (#605).
            </p>
            <div className={styles.grid}>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>SkeletonListRow · строка списка</h4>
                    <div className="rounded-card border border-dashed border-content-tertiary">
                        <SkeletonListRow widths={{title: 'w-2/5', subtitle: 'w-3/5'}} />
                        <SkeletonListRow
                            value
                            description
                            widths={{title: 'w-1/2', subtitle: 'w-2/5'}}
                        />
                        <SkeletonListRow value widths={{title: 'w-3/5', subtitle: 'w-1/2'}} />
                        <SkeletonListRow
                            value
                            trailing
                            leading={false}
                            widths={{title: 'w-2/5', subtitle: 'w-3/5'}}
                        />
                        <SkeletonListRow
                            subtitle={false}
                            value
                            trailing
                            widths={{title: 'w-2/5', subtitle: 'w-3/5'}}
                        />
                    </div>
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>SkeletonSection · серая секция</h4>
                    <SkeletonSection rows={3} />
                    <SkeletonSection rows={2} className="mx-6" />
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>SkeletonCard · карточка-плитка</h4>
                    <div className="flex gap-2 overflow-hidden">
                        <SkeletonCard />
                        <SkeletonCard className="w-40" />
                    </div>
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>SkeletonMedia · фото/баннер</h4>
                    <SkeletonMedia />
                    <div className="flex items-center gap-2">
                        <SkeletonMedia className="h-20 w-20 rounded-full" />
                        <SkeletonMedia className="h-20 flex-1" />
                    </div>
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>SkeletonButton · CTA-кнопка</h4>
                    <SkeletonButton />
                    <SkeletonButton className="w-2/3" />
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>SkeletonFormField · поле формы</h4>
                    <div className="flex flex-col gap-6 rounded-card border border-dashed border-content-tertiary p-6">
                        <SkeletonFormField />
                        <SkeletonFormField labelWidth="w-36" />
                    </div>
                </div>
            </div>
        </div>
    );
}

export function InfiniteQueryTailSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>InfiniteQueryTail · хвост бесконечной ленты</h3>
            <p className={styles.groupTitle}>
                Sentinel дозагрузки и индикатор «Загружаем еще» одной строкой на
                экран (#633): компонент сам держит sentinel, подключает
                useInfiniteScroll и жив при тёплом кэше (#631). Ниже — оба тона
                индикатора догрузки (LoadingMoreIndicator) на статичной заглушке
                запроса; в приложении хвост ставится последним элементом ленты.
                В мессенджерских лентах «новые снизу» (#709) тот же компонент
                ставится НАД списком под именем InfiniteQueryHead — prepend
                старых при прокрутке вверх.
            </p>
            <div className={styles.grid}>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>Хвост без догрузки · sentinel ждёт</h4>
                    <div className="rounded-card border border-dashed border-content-tertiary p-4 text-center text-sm text-content-secondary">
                        <InfiniteQueryTail query={STUB_IDLE_QUERY} />
                        ↑ sentinel (пустой div) — невидим, продолжение появится при подходе к краю
                    </div>
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>LoadingMoreIndicator · тон base</h4>
                    <div className="rounded-card border border-dashed border-content-tertiary">
                        <InfiniteQueryTail query={STUB_FETCHING_QUERY} />
                    </div>
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>Голова · InfiniteQueryHead (#709)</h4>
                    <div className="rounded-card border border-dashed border-content-tertiary">
                        <InfiniteQueryHead query={STUB_FETCHING_QUERY} />
                    </div>
                </div>
                <div className="flex w-full flex-col gap-2">
                    <h4 className={styles.groupTitle}>LoadingMoreIndicator · тон muted</h4>
                    <div className="flex flex-col gap-2 rounded-card bg-surface-muted p-3">
                        <InfiniteQueryTail query={STUB_FETCHING_QUERY} tone="muted" />
                    </div>
                </div>
            </div>
        </div>
    );
}
