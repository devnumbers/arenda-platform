'use client';

import { useState } from 'react';
import type { JSX } from 'react';
import {
    Button,
    ConfirmDialog,
    Modal,
    ModalClose,
    ModalContent,
    ModalTrigger,
    SupportModal,
    SuccessPopup,
} from '@/shared/ui/design';
import styles from '../../page.module.css';

export function ModalSection(): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>Modal</h3>
            <div className={styles.grid}>
                <Modal>
                    <ModalTrigger asChild>
                        <Button>Открыть модалку</Button>
                    </ModalTrigger>
                    <ModalContent title="Что создать?" description="Выберите тип регулярного платежа">
                        <div className={styles.column} style={{ alignItems: 'stretch' }}>
                            <Button variant="secondary">Платеж</Button>
                            <Button variant="secondary">Автоплатеж</Button>
                            <ModalClose asChild>
                                <Button variant="clear">Отмена</Button>
                            </ModalClose>
                        </div>
                    </ModalContent>
                </Modal>
            </div>
        </div>
    );
}

type SupportModalSectionProps = {
    readonly open: boolean;
    /** Модалка одна на витрину: пилюля десктопной навигации открывает её же —
     * состояние живёт на сборке (#820). */
    readonly onOpenChange: (open: boolean) => void;
};

export function SupportModalSection({ open, onOpenChange }: SupportModalSectionProps): JSX.Element {
    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>SupportModal · «Связаться с нами» (#766)</h3>
            <p className={styles.groupTitle}>
                Единая поверхность поддержки вместо страницы /support (карта #761):
                канва Modal — карточка ≥768 / vaul-шит ниже; лого HeaderLogo 112×28,
                заголовок H1 28/32 + подпись 16/18, Primary «Написать в Телеграм»
                (внешняя ссылка в новой вкладке) и Secondary-строка почты с копированием
                в буфер (Figma 2355:52709 — десктоп, 2355:52684 — планшет, 2355:52746 —
                мобайл). Карточка макета уже канона — max-w-400. Триггеры в продукте:
                пилюля «Поддержка» десктопа, шестая ячейка шита «Еще», кнопка экрана
                «Тариф», пилюли «Написать в поддержку» шагов входа.
            </p>
            <div className={styles.grid}>
                <Button onClick={() => onOpenChange(true)}>
                    Открыть «Связаться с нами»
                </Button>
            </div>
            <SupportModal open={open} onOpenChange={onOpenChange} />
        </div>
    );
}

export function ConfirmDialogSection(): JSX.Element {
    const [confirmOpen, setConfirmOpen] = useState(false);
    const [deleteOpen, setDeleteOpen] = useState(false);
    const [confirmLargeOpen, setConfirmLargeOpen] = useState(false);
    const [deletePropertyOpen, setDeletePropertyOpen] = useState(false);

    return (
        <div className={styles.group}>
            <h3 className={styles.groupTitle}>ConfirmDialog · подтверждение</h3>
            <p className={styles.groupTitle}>
                Карточка на десктопе, нижний шит на мобиле: отмена secondary +
                подтверждение primary; разрушительное — confirmVariant=&quot;danger&quot;.
                Закрытие — на потребителе.
            </p>
            <div className={styles.grid}>
                <Button onClick={() => setConfirmOpen(true)}>Подтвердить действие</Button>
                <Button variant="danger" onClick={() => setDeleteOpen(true)}>
                    Удалить (danger)
                </Button>
                <Button onClick={() => setConfirmLargeOpen(true)}>
                    Подтвердить (описание 16)
                </Button>
                <Button variant="danger" onClick={() => setDeletePropertyOpen(true)}>
                    Удалить объект (столбиком)
                </Button>
            </div>
            <ConfirmDialog
                open={confirmOpen}
                onOpenChange={setConfirmOpen}
                title="Сменить тариф?"
                description="Новые условия применятся со следующего периода"
                confirmLabel="Сменить"
                onConfirm={() => setConfirmOpen(false)}
            />
            <ConfirmDialog
                open={deleteOpen}
                onOpenChange={setDeleteOpen}
                title="Удалить контакт?"
                description="Контакт исчезнет из книги контактов объекта"
                confirmLabel="Удалить"
                confirmVariant="danger"
                onConfirm={() => setDeleteOpen(false)}
            />
            {/* #627: подпись макета R/400 16/18 — descriptionClassName
             * поверх каноничных 14px. */}
            <ConfirmDialog
                open={confirmLargeOpen}
                onOpenChange={setConfirmLargeOpen}
                title="Завершить аренду?"
                description="Объект станет свободным, арендный платеж завершится. Данные аренды сохранятся в разделе «Прошлые аренды»"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Завершить"
                cancelLabel="Отменить"
                onConfirm={() => setConfirmLargeOpen(false)}
            />
            {/* #629: stacked + children + titleClassName — кнопки
             * столбиком (danger сверху, решение владельца 12.09),
             * заголовок H1 28/32, красное предупреждение о
             * последствиях (danger-soft) между описанием и кнопками.
             * Figma 1583:56558. */}
            <ConfirmDialog
                open={deletePropertyOpen}
                onOpenChange={setDeletePropertyOpen}
                title="Удалить объект?"
                titleClassName="text-[28px] leading-8"
                description="Объект будет удален. Вместо удаления объект можно перевести в архив"
                descriptionClassName="text-base leading-[18px]"
                confirmLabel="Удалить"
                cancelLabel="Отменить"
                confirmVariant="danger"
                stacked
                onConfirm={() => setDeletePropertyOpen(false)}
            >
                <p className="text-sm leading-4 text-danger-soft">
                    Будут удалены данные аренд объекта, все операции объекта, платежи, контакты и задачи, связанные с объектом. Это действие нельзя отменить
                </p>
            </ConfirmDialog>
        </div>
    );
}

export function SuccessPopupSection(): JSX.Element {
    const [successPopupOpen, setSuccessPopupOpen] = useState(false);

    return (
        <>
            <div className={styles.group}>
                <h3 className={styles.groupTitle}>SuccessPopup · успех-попап</h3>
                <p className={styles.groupTitle}>
                    Центрированная карточка без затемнения на любой ширине (#744, Figma
                    2329:148661): иконка Icon/Color/GoodWhite 48, зелёное сообщение 16/18
                    Medium (#00A63E из макета), крестик закрытия. Потребители — действия
                    центра уведомлений.
                </p>
                <div className={styles.grid}>
                    <Button onClick={() => setSuccessPopupOpen(true)}>Показать попап</Button>
                </div>
            </div>
            <SuccessPopup
                open={successPopupOpen}
                onOpenChange={setSuccessPopupOpen}
                message="Все уведомления прочитаны"
            />
        </>
    );
}
