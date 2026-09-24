import { describe, expect, it } from 'vitest';
import { emailReminderCaption } from './email-reminder-caption';

describe('emailReminderCaption', () => {
  it('арендный предмет с почтой — хвост с адресом', () => {
    expect(emailReminderCaption('об оплате', 'a@b.ru')).toBe(
      'Будем напоминать об оплате на вашу почту a@b.ru',
    );
  });

  it('арендный предмет без почты — хвост обрывается после «почту»', () => {
    expect(emailReminderCaption('об оплате', null)).toBe(
      'Будем напоминать об оплате на вашу почту',
    );
  });

  it('платёжный предмет с почтой — хвост с адресом', () => {
    expect(emailReminderCaption('о платеже', 'a@b.ru')).toBe(
      'Будем напоминать о платеже на вашу почту a@b.ru',
    );
  });

  it('платёжный предмет без почты — хвост обрывается после «почту»', () => {
    expect(emailReminderCaption('о платеже', null)).toBe(
      'Будем напоминать о платеже на вашу почту',
    );
  });
});
