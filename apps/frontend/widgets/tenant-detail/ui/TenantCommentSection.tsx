'use client';

import type { JSX } from 'react';
import { PropertyDetailSection } from '@/widgets/property-detail';
import styles from './TenantCommentSection.module.css';

export type TenantCommentSectionProps = {
  readonly comment: string;
};

export function TenantCommentSection({ comment }: TenantCommentSectionProps): JSX.Element {
  return (
    <PropertyDetailSection>
      <h2 className={styles.title}>Комментарий</h2>
      <p className={styles.text}>{comment}</p>
    </PropertyDetailSection>
  );
}
