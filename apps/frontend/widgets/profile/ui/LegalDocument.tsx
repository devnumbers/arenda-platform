import type { JSX, ReactNode } from 'react';
import type { LegalParagraphSegment } from '../lib/legal-documents';
import { placeholderLegalSection } from '../lib/legal-documents';

/** Типовая страница правового документа «Информации» (макет 2349-68011,
 * решение владельца 28.09 после аудита #877): крупный h1 28/32 в контенте
 * (в баре тайтла нет — только «Назад»), секции H2 20/24 с номером в тексте,
 * текст 14/16; ссылки синим #2B7FFF с подчёркиванием (аннотация
 * макета «Ссылки синим и подчеркивание»). Рендерер секций из модуля
 * lib/legal-documents: текст пока один типовой для всех трёх документов
 * (политика/соглашение/оферта) — заглушка из макета, боевые тексты лягут
 * в модуль (per-document) без правки рендерера. */
export function LegalDocument({ title }: { readonly title: string }): JSX.Element {
  const { heading, paragraphs } = placeholderLegalSection;
  return (
    <article className="flex flex-col gap-6 pb-6 [word-break:break-word] text-content">
      <h1 className="m-0 text-[28px] font-semibold leading-8">{title}</h1>
      <section className="flex flex-col items-start gap-3">
        <h2 className="m-0 w-full text-xl font-semibold leading-6">{heading}</h2>
        <div className="text-sm leading-4">
          {paragraphs.map((paragraph, index) => (
            <p key={index} className={index === paragraphs.length - 1 ? 'm-0' : 'm-0 mb-4'}>
              {paragraph.map(renderSegment)}
            </p>
          ))}
        </div>
      </section>
    </article>
  );
}

function renderSegment(segment: LegalParagraphSegment, index: number): ReactNode {
  if (segment.href === undefined) {
    return segment.text;
  }
  return (
    <a
      key={index}
      href={segment.href}
      target="_blank"
      rel="noopener noreferrer"
      className="text-primary underline decoration-from-font"
    >
      {segment.text}
    </a>
  );
}
