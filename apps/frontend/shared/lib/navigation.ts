import {useRouter} from 'next/navigation';

type AppRouter = ReturnType<typeof useRouter>;

export function goBack(router: AppRouter, fallbackHref: string): void {
    if (typeof window !== 'undefined' && window.history.length > 1) {
        router.back();
    } else {
        router.replace(fallbackHref);
    }
}
