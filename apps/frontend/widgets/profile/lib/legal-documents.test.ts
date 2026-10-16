import { describe, expect, it } from 'vitest';
import { placeholderLegalSection } from './legal-documents';

describe('placeholderLegalSection', () => {
  it('заголовок секции — «1.» с номером в тексте', () => {
    expect(placeholderLegalSection.heading).toBe('1. Общие положения');
  });

  it('три абзаца; ссылка alterix.ru — единственный сегмент с href', () => {
    const { paragraphs } = placeholderLegalSection;
    expect(paragraphs).toHaveLength(3);

    const linked = paragraphs
      .flat()
      .filter((segment) => segment.href !== undefined);
    expect(linked).toStrictEqual([{ text: 'https://alterix.ru/', href: 'https://alterix.ru/' }]);
  });

  it('склейка сегментов абзаца с ссылкой даёт исходное предложение', () => {
    const [paragraph] = placeholderLegalSection.paragraphs.slice(-1);
    const text = (paragraph ?? []).map((segment) => segment.text).join('');
    expect(text).toBe(
      '1.2. Настоящая политика Оператора в отношении обработки персональных данных (далее — Политика) применяется ко всей информации, которую Оператор может получить о посетителях веб-сайта https://alterix.ru/.',
    );
  });
});
