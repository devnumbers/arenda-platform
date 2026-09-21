import { describe, expect, it } from 'vitest';
import { mapSessionListResponse } from './mappers';

describe('mapSessionListResponse', () => {
  it('маппит список сессий с нормализацией опциональных полей', () => {
    const sessions = mapSessionListResponse({
      sessions: [
        {
          id: '0f5a5895-1e57-4eff-8bf9-fjord-0001',
          deviceType: 'computer',
          browser: 'Chrome',
          browserMajor: 121,
          os: 'macOS',
          city: 'Москва',
          lastIp: '91.108.4.10',
          lastSeenAt: '2026-09-18T10:00:00Z',
          createdAt: '2026-09-01T08:00:00Z',
          current: true,
        },
        {
          id: '0f5a5895-1e57-4eff-8bf9-fjord-0002',
          deviceType: 'phone',
          browser: '',
          os: '',
          city: null,
          lastIp: null,
          lastSeenAt: '2026-09-17T09:30:00Z',
          createdAt: '2026-09-10T08:00:00Z',
          current: false,
        },
      ],
    });

    expect(sessions).toStrictEqual([
      {
        id: '0f5a5895-1e57-4eff-8bf9-fjord-0001',
        deviceType: 'computer',
        browser: 'Chrome',
        browserMajor: 121,
        os: 'macOS',
        city: 'Москва',
        lastIp: '91.108.4.10',
        lastSeenAt: '2026-09-18T10:00:00Z',
        createdAt: '2026-09-01T08:00:00Z',
        current: true,
      },
      {
        id: '0f5a5895-1e57-4eff-8bf9-fjord-0002',
        deviceType: 'phone',
        browser: '',
        browserMajor: null,
        os: '',
        city: null,
        lastIp: null,
        lastSeenAt: '2026-09-17T09:30:00Z',
        createdAt: '2026-09-10T08:00:00Z',
        current: false,
      },
    ]);
  });
});
