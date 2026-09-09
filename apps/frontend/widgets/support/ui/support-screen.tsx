import type { JSX } from 'react';
import { SmallArrowRight } from '@/shared/assets/icons';
import { PageContent, TopNav } from '@/shared/ui/design';

type SupportContact = {
    readonly label: string;
    readonly value: string;
    readonly href: string;
};

const supportContacts: ReadonlyArray<SupportContact> = [
    {
        label: 'Почта поддержки',
        value: 'hello@rentlee.ru',
        href: 'mailto:hello@rentlee.ru',
    },
    {
        label: 'Телефон',
        value: '8 (800) 555-35-35',
        href: 'tel:88005553535',
    },
];

/**
 * Экран «Поддержка» на едином хроме (карта #556, тикет #567): страница
 * пункта вторичной навигации — хаб-шапка с крыльями (TopNav mobileWings),
 * заголовок раздела 28, дальше прежний состав без макета в Figma —
 * карточка «Нужна помощь?» и строки-ссылки (почта/телефон) на канонических
 * поверхностях design-слоя: одна серая карточка на группу, строка —
 * анатомия Row Button (заголовок, значение под ним, SmallArrowRight 24
 * справа). Активность пункта — из нав-модели (#558): на ПК подсвечена
 * пилюля, на мобайле — строка «Поддержка» в шите «Еще».
 */
export function SupportScreen(): JSX.Element {
    return (
        <>
            <TopNav mobileWings />
            <PageContent>
                <h1 className="pl-6 text-[28px] font-semibold leading-8 text-content">
                    Поддержка
                </h1>

                <section className="mx-6 mt-6 flex flex-col gap-3 rounded-card bg-surface-muted p-6">
                    <h2 className="text-base font-semibold leading-6 text-content">
                        Нужна помощь?
                    </h2>
                    <p className="text-sm leading-5 text-content-secondary">
                        Напишите нам на почту или позвоните. Мы на связи ежедневно с 9:00 до
                        21:00 по московскому времени.
                    </p>
                </section>

                <nav
                    aria-label="Контакты поддержки"
                    className="mx-6 mt-4 flex flex-col rounded-card bg-surface-muted p-6"
                >
                    {/* Значение контакта — под заголовком строки, не справа:
                     * почта не сходится в одну строку с подписью уже на
                     * минимальной ширине 320. */}
                    {supportContacts.map((contact) => (
                        <a
                            key={contact.href}
                            href={contact.href}
                            className="group/row flex w-full items-center gap-3 py-2 text-left outline-none transition-opacity hover:opacity-80 focus-visible:ring-4 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-white active:opacity-80"
                        >
                            <span className="flex min-w-0 flex-1 flex-col gap-1">
                                <span className="truncate text-base font-medium text-content group-hover/row:text-content-secondary">
                                    {contact.label}
                                </span>
                                <span className="truncate text-sm text-content-secondary">
                                    {contact.value}
                                </span>
                            </span>
                            <SmallArrowRight className="h-6 w-6 shrink-0" aria-hidden />
                        </a>
                    ))}
                </nav>
            </PageContent>
        </>
    );
}
