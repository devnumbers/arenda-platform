import { Reveal } from "@/components/reveal";
import { RentalsCarousel } from "@/components/sections/rentals-carousel";

// «Управляйте арендой» — макет 2814-884: центрированный H2 + карусель
// из шести карточек с точками.
export function Rentals() {
  return (
    <section id="rentals" className="mt-24 desk:mt-[156px]">
      <Reveal className="mx-auto mb-8 w-full max-w-[1048px] px-6 desk:mb-14 desk:max-w-[1000px] desk:px-10">
        <h2 className="text-center text-[28px] font-semibold leading-8 desk:text-h2 desk:leading-[60px]">
          Управляйте арендой
        </h2>
      </Reveal>
      <RentalsCarousel />
    </section>
  );
}
