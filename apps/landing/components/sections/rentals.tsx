import { Reveal } from "@/components/reveal";
import { RentalsCarousel } from "@/components/sections/rentals-carousel";

// «Управляйте арендой» — макеты 2967-75818/3005-78042/3008-79124:
// заголовок по центру на ПК и влево на планшете-мобиле, лента без
// точек и «полки» (механика — components/sections/rentals-carousel.tsx).
export function Rentals() {
  return (
    <section id="rentals" className="mt-24 scroll-mt-[88px] desk:mt-[156px] desk:scroll-mt-[104px]">
      <Reveal className="mx-auto mb-8 w-full max-w-[1048px] px-6 desk:mb-14 desk:max-w-[1000px] desk:px-10">
        <h2 className="text-[28px] font-semibold leading-8 desk:text-center desk:text-h2 desk:leading-[60px]">
          Управляйте арендой
        </h2>
      </Reveal>
      <RentalsCarousel />
    </section>
  );
}
