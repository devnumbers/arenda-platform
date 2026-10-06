'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { ArrowLeft, BoldPerson, Cancel, Search, VerticalMenu } from '@/shared/assets/icons';
import {
    Button,
    DesktopMenuButton,
    fullscreenSurfaceClass,
    HubTitle,
    IconButton,
    MoreSheet,
    PageContent,
    SearchField,
    StepsChip,
    SubScreenShell,
    TopNav,
    TopNavBackButton,
    TopNavTitle,
} from '@/shared/ui/design';
import { cn } from '@/shared/lib/cn';
import { navSectionById, supportNavSection } from '@/shared/config/navigation';
import styles from '../../page.module.css';

type TopNavSectionProps = {
    readonly value: string;
    /** Демо-значение шарится между секциями — живёт на сборке (#820). */
    readonly onValueChange: (value: string) => void;
};

export function TopNavSection({ value, onValueChange }: TopNavSectionProps): JSX.Element {
    const [overlayOpen, setOverlayOpen] = useState(false);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>TopNav</h3>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                <TopNav
                    leading={
                        <span className="px-5 text-lg font-bold text-primary">Рентли</span>
                    }
                    trailing={
                        <span className="flex items-center gap-3 pr-2">
                            <span className="text-sm font-medium text-content">Даниил</span>
                            <span className="flex h-11 w-11 items-center justify-center rounded-pill bg-surface-muted text-content">
                                <BoldPerson className="h-6 w-6" />
                            </span>
                        </span>
                    }
                />
                <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
                    <TopNavTitle title="Избранные платежи" subtitle="На паузе" />
                </TopNav>
                <TopNav
                    leading={<IconButton icon={<ArrowLeft />} label="Назад" />}
                    trailing={<IconButton icon={<Search />} label="Поиск" />}
                >
                    <StepsChip step={1} total={5} size="m" />
                </TopNav>
                <TopNav leading={<IconButton icon={<ArrowLeft />} label="Назад" />}>
                    <div className="w-full">
                        <SearchField
                            placeholder="Поиск операций"
                            value={value}
                            onChange={(event) => onValueChange(event.target.value)}
                            onClear={() => onValueChange('')}
                        />
                    </div>
                </TopNav>
                {/* Хаб-вариант (#523): «крылья» и на мобайле; вживую
                 * смотрится на экране «Задачи» (/tasks). У подэкранов
                 * с leading-кнопкой крылья на планшете (561–1023)
                 * скрываются — наложение на колонку 560 (аудит #563),
                 * на ПК ≥1024 — всегда. */}
                <TopNav mobileWings />
            </div>
            {/* Поверхностный вариант и постоянный слот бара: TopNav с overlay
             * живёт внутри полноэкранной поверхности (fullscreenSurfaceClass,
             * §1 — хром ПК не исчезает: крылья на ПК рисует сама поверхность,
             * на мобайле/планшете их нет — анатомия подэкрана); barTrailing —
             * постоянное действие бара в правом слоте (§2: кебаб), на ПК —
             * правый край колонки 560, на мобайле/планшете — левее крыла-
             * аватара. Вживую: шит фильтров истории
             * (widgets/history/ui/history-filters-sheet.tsx), пикеры и визарды-
             * оверлеи; бар-кебаб — «Уведомления»
             * (widgets/notifications/ui/notifications-feed-screen.tsx). */}
            <p className={styles.groupTitle}>
                TopNav overlay + barTrailing — сборка полноэкранной поверхности
                (§1 «хром ПК постоянен», §2 «кебаб в правом слоте бара»).
            </p>
            <div className={styles.grid}>
                <Button onClick={() => setOverlayOpen(true)}>Открыть поверхность</Button>
            </div>
            {overlayOpen && (
                <div
                    role="dialog"
                    aria-modal="true"
                    aria-label="Демо поверхности с TopNav overlay"
                    className={cn(fullscreenSurfaceClass, 'font-sans')}
                >
                    <TopNav
                        overlay
                        leading={
                            <IconButton
                                icon={<Cancel />}
                                label="Закрыть поверхность"
                                onClick={() => setOverlayOpen(false)}
                            />
                        }
                        barTrailing={<IconButton icon={<VerticalMenu />} label="Действия поверхности" />}
                    >
                        <TopNavTitle title="Фильтры" subtitle="Поверхностный вариант бара" />
                    </TopNav>
                    <div className="relative min-h-0 flex-1 overflow-y-auto">
                        <div className="mx-auto w-full max-w-column px-6 pt-6 tablet:mt-[72px]">
                            <p className="text-sm text-content-secondary">
                                Контент поверхности в колонке 560. На ПК ≥1024 лого и профиль
                                рисует сама поверхность — хром не исчезает; на мобайле/планшете
                                бар — анатомия подэкрана, кебаб стоит левее края бара.
                            </p>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

export function TopNavBackButtonSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>TopNavBackButton · HubTitle · анатомия подэкрана и хаба (#566)</h3>
            <p className={styles.groupTitle}>
                Канонические куски хаб-шапки и подэкранного хедера: TopNavBackButton —
                ведущая кнопка «Назад» (history-first goBack с фолбэком, сериализуемые
                пропы — страницу-серверный компонент можно не делать клиентской);
                HubTitle — заголовок раздела хаба 28/32 со своим паддингом 24
                (PageContent горизонталей не вкладывает). Вживую: любой подэкран
                дерева профиля и хаб «Уведомления» (/profile/notifications).
            </p>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                <TopNav leading={<TopNavBackButton fallbackHref="/ui-kit" />}>
                    <TopNavTitle title="Профиль" />
                </TopNav>
                <div className="rounded-button bg-surface-muted p-4">
                    <HubTitle>Уведомления</HubTitle>
                </div>
            </div>
        </div>
    );
}

export function SubScreenShellSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>SubScreenShell · каркас подэкрана (#568)</h3>
            <p className={styles.groupTitle}>
                Один компонент вместо ручной сборки подэкранной шапки: TopNav с
                ведущим «Назад» (TopNavBackButton), TopNavTitle в центре и
                PageContent с боковым паддингом 24. trailing — действия экрана
                (кебаб объекта), subtitle — серый подзаголовок под заголовком
                (#587, счётчик архива), contentClassName переопределяет паддинг
                контента. Шапка в этой сборке — fixed на планшете и ПК, поэтому
                вживую она видна на любом подэкране (дерево профиля, детализация
                и правка объекта); ниже — контентная колонка каркаса.
            </p>
            <div className={styles.column} style={{ maxWidth: 480 }}>
                <SubScreenShell
                    title="Информация об объекте"
                    subtitle="7 объектов"
                    fallbackHref="/ui-kit"
                    contentClassName="pt-4 pb-4"
                >
                    <p className="text-sm text-content-secondary">
                        Контент подэкрана в колонке 560.
                    </p>
                </SubScreenShell>
            </div>
        </div>
    );
}

export function PageContentSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>PageContent</h3>
            <div className="rounded-button bg-surface-muted p-4">
                <PageContent className="pt-4 pb-4">
                    <p className="text-sm text-content-secondary">
                        Центрированная колонка max-width 560 — контейнер новых экранов
                        (отступы 72/136 уменьшены для витрины).
                    </p>
                </PageContent>
            </div>
        </div>
    );
}

type DesktopMenuButtonSectionProps = {
    readonly onSupportOpen: () => void;
};

export function DesktopMenuButtonSection({ onSupportOpen }: DesktopMenuButtonSectionProps): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>DesktopMenuButton · десктопная навигация (#561)</h3>
            <p className={styles.groupTitle}>
                Кнопка десктопного хрома (Figma 1675:54051): 200×44, radius 16, иконка 24 +
                подпись 14/16; активная — серая плашка bg-surface-muted, hover-фона нет.
                В продукте ScreenLayout рендерит из них DesktopSidebar (6 разделов слева под
                хедером, Figma 1675:54050) и DesktopNavPills («Уведомления» — левый-низ 200,
                «Поддержка» — правый-низ авто, Figma 1675:54098/54096) — только на ПК ≥1024
                (561–1023 — планшетный хром с TabBar); пилюли глушатся вместе с TabBar,
                пока открыт StickyBottomBar. С onClick рендерится кнопкой-действием
                вместо ссылки — так живёт пилюля «Поддержка»: открывает SupportModal (#766).
                Живой вид — на любом экране продукта при ширине ≥1024.
            </p>
            <div className={styles.grid}>
                <div className="flex flex-col gap-0.5">
                    <DesktopMenuButton section={navSectionById('properties')} active />
                    <DesktopMenuButton section={navSectionById('payments')} />
                    <DesktopMenuButton section={navSectionById('operations')} />
                </div>
                <div className="flex flex-col items-start gap-2">
                    <DesktopMenuButton section={navSectionById('notifications')} active />
                    <DesktopMenuButton
                        section={supportNavSection}
                        onClick={onSupportOpen}
                        className="w-fit"
                    />
                </div>
            </div>
        </div>
    );
}

export function MoreSheetSection(): JSX.Element {
    const [moreSheetOpen, setMoreSheetOpen] = useState(false);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>MoreSheet · шит «Еще» мобильного TabBar</h3>
            <p className={styles.groupTitle}>
                Выезжающий снизу шит навигации: ручка 48×4, два ряда разделов из нав-модели,
                шестая ячейка — «Поддержка»-действие (TabNavAction, открывает SupportModal —
                #766), нижний ряд — сам TabBar с активным «Еще» (Figma 1721:57140, #560). Выезд
                400ms на кривой vaul, оверлей — fade 250ms; закрытие — оверлей, свайп вниз,
                повторный тап «Еще». В продукте живёт в TabBar (мобайл/планшет ≤768), здесь —
                с ручным триггером.
            </p>
            <div className={styles.grid}>
                <Button onClick={() => setMoreSheetOpen(true)}>Открыть шит «Еще»</Button>
            </div>
            <MoreSheet open={moreSheetOpen} onOpenChange={setMoreSheetOpen} />
        </div>
    );
}

export function StickyBottomBarSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>StickyBottomBar</h3>
            <p className={styles.groupTitle}>
                Живой образец закреплён внизу окна — вариант из визарда (кнопка «Далее»).
                На мобайле/планшете кнопка ездит над клавиатурой (#1151): Android — мета
                interactive-widget=resizes-content (app/layout.tsx), iOS Safari — инсет
                visual viewport и translateY на панели (useVisualKeyboardInset); на ПК
                хук бездействует.
            </p>
        </div>
    );
}
