'use client';

import type { JSX } from 'react';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';

export function PropertiesArchiveLink(): JSX.Element {
  return (
    <LinkButton href={ROUTES.propertyArchive} variant="secondary" size="large" fullWidth>
      Архивные объекты
    </LinkButton>
  );
}
