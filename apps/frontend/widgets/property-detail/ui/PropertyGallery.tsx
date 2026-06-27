'use client';

import type { JSX } from 'react';
import type { PropertyPhoto } from '@/entities/property/model/types';
import styles from './PropertyGallery.module.css';

export type PropertyGalleryProps = {
  readonly photos?: PropertyPhoto[];
  readonly alt?: string;
};

export function PropertyGallery({ photos, alt = '' }: PropertyGalleryProps): JSX.Element {
  const photo = photos?.[0];
  const src = photo?.url;
  return (
    <div className={styles.root}>
      {src ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={src}
          alt={alt}
          loading="lazy"
          className={styles.image}
        />
      ) : (
        <div className={styles.placeholder} />
      )}
    </div>
  );
}
