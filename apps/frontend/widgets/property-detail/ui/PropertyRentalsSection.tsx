'use client';

import type {JSX} from 'react';
import {ROUTES} from '@/shared/config/routes';
import {SectionHeader} from '@/shared/ui/section-header';
import { DetailSection } from '@/shared/ui/detail-section';

/** Секция-ссылка «Аренда» на странице объекта (#531): вход на экран «Аренда»
 * (пустое состояние или детализация текущей аренды). Паттерн секции
 * «Платежей» — счётчик не рисуем, список ради числа строк не запрашивается. */
export function PropertyRentalsSection({
    propertyId,
}: {
    readonly propertyId: string;
}): JSX.Element {
    return (
        <DetailSection>
            <SectionHeader title="Аренда" href={ROUTES.propertyRental(propertyId)}/>
        </DetailSection>
    );
}
