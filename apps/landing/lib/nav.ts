// Якоря навигации лендинга — пункты хедера по макету 2814-728 (7 ссылок,
// порядок как в Figma). href — id секций на главной.
export type NavItem = { label: string; href: string };

export const NAV: NavItem[] = [
  { label: "Возможности", href: "#showcase" },
  { label: "Для кого сервис", href: "#audience" },
  { label: "Как начать", href: "#steps" },
  { label: "Отзывы", href: "#testimonials" },
  { label: "Тарифы", href: "#tariffs" },
  { label: "Вопросы", href: "#faq" },
  { label: "Поддержка", href: "#contact" },
];
