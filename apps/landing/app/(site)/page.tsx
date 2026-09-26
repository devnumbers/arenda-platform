// Главная лендинга — порядок секций по макету 2814-728. Хедер, футер
// и маркер id="landing-root" приходят из (site)/layout.tsx.

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
import { Tariffs } from "@/components/sections/tariffs";
import { Testimonials } from "@/components/sections/testimonials";

export default function LandingPage() {
  return (
    <>
      <Hero />
      <Showcase />
      <Rentals />
      <Finance />
      <Organization />
      <Sharing />
      <Audience />
      <Testimonials />
      <Steps />
      <Tariffs />
      <Faq />
      <Contact />
      <Cta />
    </>
  );
}
