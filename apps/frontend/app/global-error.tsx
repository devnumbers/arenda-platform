'use client';

import type { JSX } from 'react';
import { useEffect } from 'react';
import { reportClientError } from '@/shared/lib/error-reporting/report-client-error';

export default function GlobalError({
    error,
    unstable_retry,
}: {
    error: Error & { digest?: string };
    unstable_retry: () => void;
}): JSX.Element {
    useEffect(() => {
        reportClientError(error.message, error.stack);
    }, [error]);

    return (
        <html lang="ru">
            <body>
                <h2>Что-то пошло не так</h2>
                <button type="button" onClick={() => unstable_retry()}>
                    Попробовать снова
                </button>
            </body>
        </html>
    );
}
