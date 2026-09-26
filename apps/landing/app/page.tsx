// Плейсхолдеры хедера/футера и секций фундамента (тикет T1 карты #888):
// каркас страницы в порядке макета 2814-728. Реальные секции приходят
// тикетами T2–T5 — каждая заменяет свою заглушку компонентом из components/.

type SectionStub = { id: string; title: string };

const SECTIONS: SectionStub[] = [
  { id: "hero", title: "Хиро" },
  { id: "showcase", title: "Стройте арендный бизнес" },
  { id: "rentals", title: "Управляйте арендой" },
  { id: "finance", title: "Управляйте финансами" },
  { id: "organization", title: "Организуйте дела" },
  { id: "sharing", title: "Делитесь объектом" },
  { id: "audience", title: "Для кого сервис" },
  { id: "testimonials", title: "Опыт пользователей" },
  { id: "steps", title: "Как начать пользоваться" },
  { id: "tariffs", title: "Тарифы" },
  { id: "faq", title: "Ответы на вопросы" },
  { id: "contact", title: "Остались вопросы?" },
  { id: "cta", title: "Попробуйте Рентли в деле" },
];

export default function LandingPage() {
  return (
    <div id="landing-root" className="flex min-h-screen w-full flex-col">
      <header className="flex h-20 items-center justify-between px-5 md:px-10">
        <span className="text-m font-semibold">Рентли</span>
        {/* CTA кабинета: /login — единственная публичная страница кабинета. */}
        <a
          className="rounded-2xl bg-surface px-6 py-3 text-r font-medium transition-colors hover:bg-line"
          href="/login"
        >
          Войти в сервис
        </a>
      </header>
      <main className="flex-1">
        {SECTIONS.map((section) => (
          <section
            key={section.id}
            id={section.id}
            className="mx-auto w-full max-w-[1000px] px-5 py-16 md:px-10 lg:px-0"
          >
            <h2 className="text-h4 font-semibold md:text-h3">{section.title}</h2>
            <p className="mt-3 text-r text-gray-2">
              Секция верстается тикетом карты #888.
            </p>
          </section>
        ))}
      </main>
      <footer className="border-t border-line px-5 py-10 md:px-10">
        <p className="text-xs text-gray-2">© 2026 Рентли</p>
      </footer>
    </div>
  );
}
