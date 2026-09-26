import { Reveal } from "@/components/reveal";
import { RentalsCarousel } from "@/components/sections/rentals-carousel";

// «Управляйте арендой» — макет 2814-884: центрированный H2 + карусель
// из шести карточек с точками.
export function Rentals() {
  return (
    <section id="rentals" className="mt-20 desk:mt-[156px]">
      <Reveal className="mx-auto mb-14 w-full max-w-[1000px] px-5 desk:mb-14 desk:px-10">
        <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
          Управляйте арендой
        </h2>
      </Reveal>
      <RentalsCarousel />
    </section>
  );
}
