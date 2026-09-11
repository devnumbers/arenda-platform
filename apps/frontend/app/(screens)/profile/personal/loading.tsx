import type { JSX } from 'react';
import { ROUTES } from '@/shared/config/routes';
import { SubScreenShell } from '@/shared/ui/design';
import { PersonalDataSkeleton } from '@/widgets/profile';

/** Route-loading «Моих данных» (#609): шапка — вне фазы загрузки, форма —
 * выключенным каркасом (§7). */
export default function PersonalDataLoading(): JSX.Element {
  return (
    <SubScreenShell title="Мои данные" fallbackHref={ROUTES.profile}>
      <PersonalDataSkeleton />
    </SubScreenShell>
  );
}
