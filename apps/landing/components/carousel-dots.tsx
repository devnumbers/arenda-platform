// Общий ряд точек-кнопок каруселей лент («Управляйте арендой», «Опыт
// пользователей»): count круглых кнопок, активная — тёмная, остальные —
// бледные с подсветкой при наведении. Подписи и переход — за вызывающей
// стороной: aria-label задаёт labelFor (у аренды — заголовок карточки,
// у отзывов — «Слайд N»), клик уходит в onSelect.
export function CarouselDots({
  count,
  active,
  onSelect,
  labelFor,
}: {
  count: number;
  active: number;
  onSelect: (index: number) => void;
  labelFor: (index: number) => string;
}) {
  return (
    <>
      {Array.from({ length: count }, (_, i) => (
        <button
          key={i}
          type="button"
          aria-label={labelFor(i)}
          aria-current={active === i}
          onClick={() => onSelect(i)}
          className={`size-3 cursor-pointer rounded-full transition-colors duration-200 ${
            active === i ? "bg-ink" : "bg-line hover:bg-gray-3"
          }`}
        />
      ))}
    </>
  );
}
