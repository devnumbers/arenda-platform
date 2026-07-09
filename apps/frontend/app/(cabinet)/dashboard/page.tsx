import type {Metadata} from 'next';
import {DashboardPage} from '@/widgets/dashboard';

export const metadata: Metadata = {
    title: 'Главная — Рентли',
    description: 'Главная страница личного кабинета',
};

export default function DashboardRoutePage() {
    return (
        <>
            <DashboardPage/>
        </>
    );
}
