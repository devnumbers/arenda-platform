'use client';

import type { JSX } from 'react';
import { Cancel } from '@/shared/assets/icons';
import { IconButton, Modal, ModalContent, StatusIcon } from '@/shared/ui/design';

/**
 * Попап успеха поверх поверхностей участника (#698): зелёная «печать» 48
 * и зелёный текст по центру (макеты 2010-131859 «Доступ выдан», 2177-59799
 * «Права изменены», 2008-83135 «У участника больше нет доступа к объекту»,
 * 2008-83716 «Участник удален»). Канон попапов #697 (прецедент продления
 * аренды): крестик — явно в углу карточки на десктопе, на мобайле закрытие
 * — свайп/оверлей шита.
 */
export function ParticipantSuccessPopup({
  title,
  onClose,
}: {
  readonly title: string;
  readonly onClose: () => void;
}): JSX.Element {
  return (
    <Modal open onOpenChange={(open) => open || onClose()}>
      <ModalContent title={title} titleSrOnly>
        <IconButton
          icon={<Cancel />}
          label="Закрыть"
          onClick={onClose}
          className="hidden desktop:absolute desktop:right-3 desktop:top-3 desktop:block"
        />
        <div className="flex flex-col items-center gap-2">
          <StatusIcon status="good" className="h-12 w-12" />
          <p className="text-center text-base font-medium leading-[18px] text-success">
            {title}
          </p>
        </div>
      </ModalContent>
    </Modal>
  );
}
