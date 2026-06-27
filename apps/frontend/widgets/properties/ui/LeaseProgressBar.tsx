'use client';

import type { JSX } from 'react';
import clsx from 'clsx';
import styles from './LeaseProgressBar.module.css';

const SEGMENTS = 4;

export type LeaseProgressBarProps = {
  readonly startDate: string;
  readonly endDate: string;
  readonly active?: boolean;
};

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

export function LeaseProgressBar({
  startDate,
  endDate,
  active = true,
}: LeaseProgressBarProps): JSX.Element {
  const start = new Date(startDate).getTime();
  const end = new Date(endDate).getTime();
  const now = new Date().getTime();

  const total = end - start;
  const ratio = total > 0 ? clamp((now - start) / total, 0, 1) : 0;
  const segmentProgress = ratio * SEGMENTS;
  const filledSegments = Math.floor(segmentProgress);
  const partial = segmentProgress - filledSegments;

  return (
    <div className={styles.root}>
      {Array.from({ length: SEGMENTS }).map((_, index) => {
        let fill = 0;
        if (index < filledSegments) fill = 100;
        else if (index === filledSegments) fill = partial * 100;

        return (
          <div key={index} className={styles.track}>
            <div
              className={clsx(styles.indicator, !active && styles.inactive)}
              style={{ width: `${fill}%` }}
            />
          </div>
        );
      })}
      <span
        className={clsx(styles.marker, !active && styles.inactiveMarker)}
        style={{ left: `${ratio * 100}%` }}
      />
    </div>
  );
}
