'use client';

import type { JSX } from 'react';
import styles from './LeaseProgress.module.css';

export type LeaseProgressProps = {
  readonly startDate: string;
  readonly endDate?: string | null;
};

const TRACKS = 4;

export function LeaseProgress({ startDate, endDate }: LeaseProgressProps): JSX.Element {
  if (!endDate) {
    return (
      <div className={styles.root}>
        {Array.from({ length: TRACKS }).map((_, index) => (
          <div key={index} className={styles.track} />
        ))}
      </div>
    );
  }

  const start = new Date(startDate).getTime();
  const end = new Date(endDate).getTime();
  const now = new Date().getTime();

  const ratio = Math.min(Math.max((now - start) / (end - start), 0), 1);
  const progressWidthPercent = ratio * 100;

  return (
    <div className={styles.root}>
      {Array.from({ length: TRACKS }).map((_, index) => {
        const trackStartPercent = (index / TRACKS) * 100;
        const trackEndPercent = ((index + 1) / TRACKS) * 100;

        const fillWidth = Math.max(0, Math.min(progressWidthPercent, trackEndPercent) - trackStartPercent);

        return (
          <div key={index} className={styles.track}>
            {fillWidth > 0 && (
              <div className={styles.fill} style={{ width: `${(fillWidth / (100 / TRACKS)) * 100}%` }} />
            )}
          </div>
        );
      })}
      <div
        className={styles.marker}
        style={{ left: `calc(${progressWidthPercent}% - 6px)` }}
      />
    </div>
  );
}
