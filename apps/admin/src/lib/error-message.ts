/** Сообщение об ошибке операции для тоста: текст Error или дефолт. */
export const errorMessage = (error: unknown): string =>
  error instanceof Error ? error.message : 'Ошибка операции';
