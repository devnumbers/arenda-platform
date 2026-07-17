'use client';

import {useEffect} from 'react';
import {initClientErrorReporting} from './report-client-error';

export function ErrorReporter(): null {
    useEffect(() => initClientErrorReporting(), []);

    return null;
}
