import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { PhoneChangeSkeleton } from '@/widgets/profile';

/** Route-loading «Изменения телефона» (#609): шапка — вне фазы загрузки,
 * форма — выключенным каркасом (§7). */
export default function PhoneChangeLoading(): JSX.Element {
  return (
    <SubScreenShell title="Изменение телефона" fallbackHref={ROUTES.profileAccount}>
      <PhoneChangeSkeleton />
    </SubScreenShell>
  );
}
