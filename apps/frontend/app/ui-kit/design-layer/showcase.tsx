'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import { Button, StickyBottomBar } from '@/shared/ui/design';
import styles from '../page.module.css';
import { ButtonSection, ChipButtonSection, IconButtonSection } from './sections/actions';
import {
    AmountFieldSection,
    CheckboxRadioSwitchSection,
    SearchFieldSection,
    TextFieldTitleInSection,
    TextFieldTitleOutSection,
    TextareaSection,
} from './sections/fields';
import {
    CalendarButtonSection,
    ErrorCardSection,
    ListRowSection,
    PickerMenuSection,
    StatusIconSection,
    StepsChipSection,
    UserButtonSection,
} from './sections/widgets';
import {
    CalendarDatePickerSection,
    CalendarMonthSection,
    CalendarRangePickerEmptySection,
    CalendarRangePickerSection,
    MonthDaysGridSection,
    MonthYearPickerSection,
} from './sections/calendar';
import { WheelPickerSheetSection } from './sections/wheels';
import { CategoryIconSection, PaymentButtonsSection } from './sections/payments';
import {
    DesktopMenuButtonSection,
    MoreSheetSection,
    PageContentSection,
    StickyBottomBarSection,
    SubScreenShellSection,
    TopNavBackButtonSection,
    TopNavSection,
} from './sections/navigation';
import { PickerFieldSection } from './sections/pickers';
import {
    ConfirmDialogSection,
    ModalSection,
    SuccessPopupSection,
    SupportModalSection,
} from './sections/dialogs';
import {
    EmptyStateSection,
    InfiniteQueryTailSection,
    ResendCodeTileSection,
    SkeletonPrimitivesSection,
    SkeletonShowcaseSection,
} from './sections/feedback';

/** Витрина дизайн-слоя (ADR 0050, тикет #455): шадкн/ui поверх Radix,
 * Tailwind на токенах, шрифт Onest. Внешний вид сверен с экспортами
 * Figma-фреймов «Рентли. Новые экраны сервиса» (node-id — резолюция #449).
 * Секции — по компоненту на группу, демо-состояние живёт в своей секции
 * (#820); здесь только сборка в исходном порядке и два демо-значения,
 * шарящиеся между секциями: searchValue связывает SearchField и демо-поиск
 * в TopNav, supportModalOpen — модалку «Связаться с нами» и пилюлю
 * «Поддержка» десктопной навигации. */
export function DesignLayerShowcase(): JSX.Element {
    const [searchValue, setSearchValue] = useState('');
    const [supportModalOpen, setSupportModalOpen] = useState(false);

    return (
        <>
            <section className={styles.section}>
                <h2 className={styles.sectionTitle}>Новые компоненты — дизайн-слой (ADR 0050)</h2>
                <p className={styles.groupTitle}>
                    shadcn/ui поверх Radix · Tailwind на токенах · шрифт Onest — источник правды для новых экранов
                </p>
                <ButtonSection />
                <IconButtonSection />
                <TextFieldTitleOutSection />
                <TextFieldTitleInSection />
                <CheckboxRadioSwitchSection />
                <SearchFieldSection value={searchValue} onValueChange={setSearchValue} />
                <ChipButtonSection />
                <PickerMenuSection />
                <StepsChipSection />
                <ErrorCardSection />
                <StatusIconSection />
                <UserButtonSection />
                <CalendarButtonSection />
                <CalendarMonthSection />
                <MonthYearPickerSection />
                <WheelPickerSheetSection />
                <CalendarDatePickerSection />
                <CalendarRangePickerSection />
                <CalendarRangePickerEmptySection />
                <MonthDaysGridSection />
                <AmountFieldSection />
                <ListRowSection />
                <CategoryIconSection />
                <PaymentButtonsSection />
                <TopNavSection value={searchValue} onValueChange={setSearchValue} />
                <TopNavBackButtonSection />
                <SubScreenShellSection />
                <PageContentSection />
                <ModalSection />
                <SupportModalSection open={supportModalOpen} onOpenChange={setSupportModalOpen} />
                <MoreSheetSection />
                <DesktopMenuButtonSection onSupportOpen={() => setSupportModalOpen(true)} />
                <TextareaSection />
                <PickerFieldSection />
                <EmptyStateSection />
                <ResendCodeTileSection />
                <SkeletonShowcaseSection />
                <SkeletonPrimitivesSection />
                <InfiniteQueryTailSection />
                <ConfirmDialogSection />
                <StickyBottomBarSection />
                <SuccessPopupSection />
            </section>

            <StickyBottomBar>
                <Button>Далее</Button>
            </StickyBottomBar>
        </>
    );
}
