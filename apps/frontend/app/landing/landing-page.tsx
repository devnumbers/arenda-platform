'use client';

import { useState } from 'react';
import { Header } from './components/header';
import { HeroAndCreate } from './components/hero-and-create';
import { FeaturesAndForWhom } from './components/features-and-for-whom';
import { Pricing } from './components/pricing';
import { Faq } from './components/faq';
import { Cta } from './components/cta';
import { Footer } from './components/footer';
import { ContactModal } from './components/contact-modal';

export default function LandingPage() {
    const [contactOpen, setContactOpen] = useState(false);
    const openContact = () => setContactOpen(true);

    return (
        <div className="bg-white relative w-full min-h-screen text-[#34343c]">
            <Header />
            <main>
                <HeroAndCreate />
                <FeaturesAndForWhom />
                <Pricing openContact={openContact} />
                <Faq />
                <Cta openContact={openContact} />
            </main>
            <Footer openContact={openContact} />
            <ContactModal open={contactOpen} onClose={() => setContactOpen(false)} />
        </div>
    );
}
