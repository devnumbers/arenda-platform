// JSON-LD структурированные данные единственной индексируемой страницы
// (тикет T6 карты #888): Organization + WebSite + FAQPage (вопросы и ответы
// секции FAQ). Данные — наши константы; в script текстовым ребёнком, без
// dangerouslySetInnerHTML (запрещён контуром безопасности).
import { FAQ } from "@/lib/content";

export function JsonLd() {
  const data = [
    {
      "@context": "https://schema.org",
      "@type": "Organization",
      name: "Рентли",
      url: "https://rentlee.ru/",
      email: "hello@rentlee.ru",
    },
    {
      "@context": "https://schema.org",
      "@type": "WebSite",
      name: "Рентли",
      url: "https://rentlee.ru/",
      inLanguage: "ru-RU",
    },
    {
      "@context": "https://schema.org",
      "@type": "FAQPage",
      mainEntity: FAQ.flatMap((group) =>
        group.items.map((item) => ({
          "@type": "Question",
          name: item.question,
          acceptedAnswer: { "@type": "Answer", text: item.answer },
        })),
      ),
    },
  ];

  return (
    <>
      {data.map((entry) => (
        <script key={entry["@type"]} type="application/ld+json">
          {JSON.stringify(entry)}
        </script>
      ))}
    </>
  );
}
