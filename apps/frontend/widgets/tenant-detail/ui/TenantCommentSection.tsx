'use client';

import type { JSX } from 'react';
import { DetailSection } from '@/shared/ui/detail-section';
import styles from './TenantCommentSection.module.css';

export type TenantCommentSectionProps = {
  readonly comment: string;
};

export function TenantCommentSection({ comment }: TenantCommentSectionProps): JSX.Element {
  return (
    <DetailSection>
      <h2 className={styles.title}>Комментарий</h2>
      <p className={styles.text}>{comment}</p>
    </DetailSection>
  );
}
