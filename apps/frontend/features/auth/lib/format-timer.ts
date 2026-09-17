/** ММ:СС таймера кулдауна логина — канон `formatCountdown` из
 * shared/lib/countdown (resend-канон #733); дубликат слит делегированием,
 * сам флоу логина не тронут (переезд на канон — будущая карта авторизации). */
export { formatCountdown as formatTimer } from '@/shared/lib/countdown';
