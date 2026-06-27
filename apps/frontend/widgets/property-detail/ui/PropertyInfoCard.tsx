'use client';

import type { JSX } from 'react';
import { LinkButton } from '@/shared/ui/link-button';
import { ROUTES } from '@/shared/config/routes';
import { PropertyDetailSection } from './PropertyDetailSection';
import styles from './PropertyInfoCard.module.css';

export type PropertyInfoCardProps = {
  readonly description: string | undefined;
  readonly propertyId: string;
};

export function PropertyInfoCard({
  description,
  propertyId,
}: PropertyInfoCardProps): JSX.Element {
  return (
    <PropertyDetailSection>
      <h2 className={styles.title}>Информация об объекте</h2>

      {description ? (
        <p className={styles.description}>{description}</p>
      ) : (
        <LinkButton
          href={ROUTES.propertyEdit(propertyId)}
          variant="primary"
          fullWidth
        >
          Добавить описание
        </LinkButton>
      )}
    </PropertyDetailSection>
  );
}
