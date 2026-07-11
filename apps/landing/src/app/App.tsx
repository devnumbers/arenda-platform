import { useState, useRef } from "react";
import { BrowserRouter, Routes, Route } from "react-router";
import { Header } from "./components/header";
import { Footer } from "./components/footer";
import { ContactModal } from "./components/contact-modal";
import { LandingPage } from "./components/landing-page";
import { PrivacyPage, TermsPage } from "./components/legal-pages";
import { Toast } from "./components/toast";

export default function App() {
  const [modalOpen, setModalOpen] = useState(false);
  const [toastVisible, setToastVisible] = useState(false);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const openModal = () => setModalOpen(true);
  const handleLogin = () => window.location.assign('/dashboard');

  const copyEmail = () => {
    navigator.clipboard?.writeText("hello@rentlee.ru").catch(() => {});
    setToastVisible(true);
    if (toastTimer.current) clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToastVisible(false), 2500);
  };

  return (
    <BrowserRouter>
      <div className="bg-white min-h-screen w-full">
        <Header />
        <Routes>
          <Route path="/" element={<LandingPage onContact={openModal} onLogin={handleLogin} />} />
          <Route path="/privacy" element={<PrivacyPage />} />
          <Route path="/terms" element={<TermsPage />} />
        </Routes>
        <Footer onContact={openModal} onCopyEmail={copyEmail} />
        <ContactModal open={modalOpen} onClose={() => setModalOpen(false)} />
        <Toast visible={toastVisible} message="Почта скопирована" />
      </div>
    </BrowserRouter>
  );
}
