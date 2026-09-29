import type { Metadata } from "next";
import { TERMS_SECTIONS } from "@/lib/legal-content";

export const metadata: Metadata = {
  title: "Пользовательское соглашение",
  description:
    "Условия использования сервиса учёта аренды недвижимости Рентли.",
  // Собственный canonical: без него действует canonical "/" корневого layout,
  // и страница объявляет канонической главную.
  alternates: {
    canonical: "/terms",
  },
};

export default function TermsPage() {
  return (
    <main className="mx-auto w-full max-w-[800px] px-5 pt-32 pb-24 md:px-10 lg:px-0">
      <h1 className="text-h4 font-semibold md:text-h3">
        Пользовательское соглашение
      </h1>
      <div className="mt-8 flex flex-col gap-6">
        {TERMS_SECTIONS.map((section) => (
          <section key={section.heading} className="flex flex-col gap-2">
            <h2 className="text-m font-medium">{section.heading}</h2>
            <p className="text-r text-gray-2">{section.body}</p>
          </section>
        ))}
      </div>
    </main>
  );
}
