// Главная лендинга — порядок секций по макету 2814-728. Хедер, футер
// и маркер id="landing-root" приходят из (site)/layout.tsx.

import { Suspense } from "react";
import { JsonLd } from "@/components/json-ld";
import { Audience } from "@/components/sections/audience";
import { Contact } from "@/components/sections/contact";
import { Cta } from "@/components/sections/cta";
import { Faq } from "@/components/sections/faq";
import { Finance } from "@/components/sections/finance";
import { Hero } from "@/components/sections/hero";
import { Organization } from "@/components/sections/organization";
import { Rentals } from "@/components/sections/rentals";
import { Sharing } from "@/components/sections/sharing";
import { Showcase } from "@/components/sections/showcase";
import { Steps } from "@/components/sections/steps";
import { Tariffs, TariffsFallback } from "@/components/sections/tariffs";
import { Testimonials } from "@/components/sections/testimonials";

export default function LandingPage() {
  return (
    <>
      <JsonLd />
      <Hero />
      <Showcase />
      <Rentals />
      <Finance />
      <Organization />
      <Sharing />
      <Audience />
      <Testimonials />
      <Steps />
      {/* Дырка под <Suspense> (ADR 0063): async-«Тарифы» с фетчем /me не
          задерживают shell страницы — сначала уходит гостевой фолбэк той же
          геометрии, контент подменяется по готовности. */}
      <Suspense fallback={<TariffsFallback />}>
        <Tariffs />
      </Suspense>
      <Faq />
      <Contact />
      <Cta />
    </>
  );
}
