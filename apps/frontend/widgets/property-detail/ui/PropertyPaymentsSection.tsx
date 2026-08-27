'use client';

import type {JSX} from 'react';
import {ROUTES} from '@/shared/config/routes';
import {SectionHeader} from '@/shared/ui/section-header';
import { DetailSection } from '@/shared/ui/detail-section';

/** Секция-ссылка «Платежи» на странице объекта (#463, история 13): вход на
 * экран «Платежи объекта» нового хрома. Счётчик не рисуем — списка платежей
 * ради числа строк страница объекта не запрашивает. */
export function PropertyPaymentsSection({
    propertyId,
}: {
    readonly propertyId: string;
}): JSX.Element {
    return (
        <DetailSection>
            <SectionHeader title="Платежи" href={ROUTES.propertyPayments(propertyId)}/>
        </DetailSection>
    );
}
