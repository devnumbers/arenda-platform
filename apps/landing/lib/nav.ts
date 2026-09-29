// Якоря навигации лендинга — пункты хедера по макету 2814-728 (7 ссылок,
// порядок как в Figma). href — id секций на главной.
//
// Секции с id несут scroll-mt-[88px] desk:scroll-mt-[104px]: sticky-плашка
// хедера занимает 72px (pt-2 + h-16), без scroll-margin прыжок по якорю
// прячет заголовок секции под ней; 88/104 = 72 + воздух 16/32 (решение
// владельца 29.09: отступ единый на всех секциях, прыжок мгновенный).
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
