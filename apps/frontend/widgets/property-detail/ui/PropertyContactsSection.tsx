'use client';

import type {JSX} from 'react';
import {ROUTES} from '@/shared/config/routes';
import {SectionHeader} from '@/shared/ui/section-header';
import { DetailSection } from '@/shared/ui/detail-section';

/** Секция-ссылка «Контакты» на странице объекта (#511): вход на экран
 * «Контакты объекта» нового хрома (карта #503 — вход как у «Платежей»).
 * Счётчик не рисуем — списка контактов ради числа строк страница объекта
 * не запрашивает, как и у «Платежей» (#463). */
export function PropertyContactsSection({
    propertyId,
}: {
    readonly propertyId: string;
}): JSX.Element {
    return (
        <DetailSection>
            <SectionHeader title="Контакты" href={ROUTES.propertyContacts(propertyId)}/>
        </DetailSection>
    );
}
