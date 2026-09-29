import { useEffect } from "react";
import { useLocation } from "react-router";
import { Button } from "./button";
import { Faq } from "./faq";
import { UserIcon, CaseIcon, BuildingIcon } from "./icons";
import { typo } from "./typo";

import imgLogo3D from "../../imports/19201201/6ca2192c5340d8c4ec8da7503b140525f192952c.png";
import imgContent from "../../imports/19201201/6a995eb615e1a87d9110b739d0bb4abe19b55a5c.png";
import img1Fff1 from "../../imports/19201201/b743f11406b4f929c43cde7189887ebc7702d86b.png";
import img2Fff1 from "../../imports/19201201/935889dd7b693e42173170364d152e372c5c2dc2.png";
import img1Fff2 from "../../imports/19201201/a1a879927734df9f245667aa7181eaf3e4d42a03.png";
import img1Fff3 from "../../imports/19201201/56d707c488927e68a7f1ad26e00a7da0f18ca478.png";
import imgRrr21 from "../../imports/19201201/d3f0e6470dccea0da27e64bffc91873b0985b96a.png";
import imgCtaBg from "../../imports/19201201/253b396c7d7d2615db55598738e44267b889c848.png";
import imgContentMobile from "../../imports/599320/aa607edcb9f7189fd09ebaee2a433cbcf18d8f6a.png";

const FEATURES = [
  {
    img: img1Fff1,
    title: "Добавьте договор аренды",
    text: "Укажите объект, арендатора, срок и дату оплаты. Напоминание придет автоматически",
    imageLeft: true,
  },
  {
    img: img2Fff1,
    title: "Напомним об окончании договора",
    text: "Заранее узнавайте, когда договор подходит к концу. Продлите, измените или завершите аренду",
    imageLeft: false,
  },
  {
    img: img1Fff2,
    title: "Добавляйте платежи",
    text: "Создайте регулярный платеж один раз. Следующие оплаты появятся автоматически",
    imageLeft: true,
  },
  {
    img: img1Fff3,
    title: "Уведомим о просроченном платеже",
    text: "Если платеж просрочен, вы увидите сумму и объект",
    imageLeft: false,
  },
  {
    img: imgRrr21,
    title: "Отслеживайте доходы и расходы",
    text: "Добавляйте доходы и расходы, чтобы видеть прибыль по каждому объекту",
    imageLeft: true,
  },
];

function Feature({ img, title, text, imageLeft }: (typeof FEATURES)[number]) {
  return (
    <div
      className={`flex flex-col max-[767px]:gap-[8px] min-[768px]:flex-row min-[768px]:gap-[96px] min-[768px]:items-center justify-center w-full ${
        imageLeft ? "" : "min-[768px]:flex-row-reverse"
      }`}
    >
      <div className="w-full min-[768px]:flex-1 min-[768px]:min-w-px aspect-[3360/2520] rounded-[24px] min-[768px]:rounded-[32px] overflow-hidden">
        <img alt="" className="size-full object-cover pointer-events-none" src={img} />
      </div>
      <div className="w-full min-[768px]:flex-1 min-[768px]:min-w-px flex flex-col gap-[8px] min-[768px]:gap-[16px] items-start text-[#222] max-[767px]:bg-[#f1f3f6] max-[767px]:rounded-[24px] max-[767px]:p-[24px]">
        <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[20px] min-[768px]:text-[40px] w-full">
          {typo(title)}
        </p>
        <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 text-[14px] min-[768px]:text-[18px] w-full">
          {typo(text)}
        </p>
      </div>
    </div>
  );
}

const AUDIENCE = [
  {
    Icon: UserIcon,
    iconBg: "bg-[#2b7fff]",
    cardBg: "bg-[#d9ebff]",
    title: "Собственники",
    text: "Сдаете квартиру, дом, комнату, гараж или помещение",
    titleColor: "text-[#222]",
    textColor: "text-[#2b7fff]",
  },
  {
    Icon: CaseIcon,
    iconBg: "bg-white",
    cardBg: "bg-[#222]",
    title: "Бизнес",
    text: "Сдаете офисы, склады, торговые помещения, гаражи",
    titleColor: "text-white",
    textColor: "text-white",
  },
  {
    Icon: BuildingIcon,
    iconBg: "bg-[#222]",
    cardBg: "bg-[#f1f3f6]",
    title: "Агентства",
    text: "Сдаете десятки или сотни объектов",
    titleColor: "text-[#222]",
    textColor: "text-[#222]",
  },
];

export function LandingPage({
  onContact,
  onLogin,
}: {
  onContact: () => void;
  onLogin: () => void;
}) {
  const location = useLocation();

  // Smooth-scroll to a section when arriving with a hash (e.g. from another page)
  useEffect(() => {
    if (location.hash) {
      const id = location.hash.slice(1);
      const el = document.getElementById(id);
      if (el) {
        setTimeout(() => el.scrollIntoView({ behavior: "smooth", block: "start" }), 60);
      }
    } else {
      window.scrollTo(0, 0);
    }
  }, [location]);

  return (
    <div id="top" className="bg-white w-full flex flex-col items-center">
      {/* Hero */}
      <section className="w-full flex justify-center px-[20px] min-[768px]:px-[40px] min-[1201px]:px-[80px] pt-[180px] pb-[128px] min-[1201px]:py-[220px]">
        <div className="w-full max-w-[1200px] flex flex-col items-center gap-[24px] min-[768px]:gap-[40px] min-[1201px]:flex-row-reverse min-[1201px]:items-start min-[1201px]:gap-[100px]">
          {/* Logo */}
          <div className="size-[100px] min-[768px]:size-[200px] min-[1201px]:size-[300px] shrink-0">
            <img alt="" className="size-full object-cover pointer-events-none" src={imgLogo3D} />
          </div>
          {/* Text + button */}
          <div className="flex flex-col gap-[24px] min-[768px]:gap-[40px] items-center min-[1201px]:items-start min-[1201px]:flex-1">
            <div className="flex flex-col gap-[8px] min-[768px]:gap-[24px] items-center min-[1201px]:items-start text-center min-[1201px]:text-left w-full">
              <div className="font-['Involve:Bold',sans-serif] text-[#222] text-[28px] min-[768px]:text-[52px] w-full">
                <p className="leading-[1.15]">Сервис управления</p>
                <p className="leading-[1.15]">арендой недвижимости</p>
              </div>
              <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-40 text-[#222] text-[14px] min-[768px]:text-[24px] w-full">
                {typo("Объекты, арендаторы, платежи и договоры в одном месте")}
              </p>
            </div>
            <Button variant="primary" onClick={onLogin}>
              {typo("Войти в сервис")}
            </Button>
          </div>
        </div>
      </section>

      {/* Black content block */}
      <section className="w-full">
        <div className="bg-[#222] rounded-[24px] min-[1201px]:rounded-[64px] w-full flex flex-col items-center px-[20px] py-[96px] min-[768px]:p-[64px] min-[1201px]:p-[128px]">
          <div className="w-full max-w-[1200px] flex flex-col gap-[32px] min-[1201px]:gap-[52px] items-start">
            <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[24px] min-[1201px]:text-[48px] text-center text-white w-full">
              {typo("Создайте карточку своей недвижимости и управляйте арендой")}
            </p>
            {/* landscape on tablet/desktop, portrait on mobile */}
            <div className="hidden min-[768px]:block w-full aspect-[1200/750] rounded-[32px] overflow-hidden">
              <img alt="" className="size-full object-cover pointer-events-none" src={imgContent} />
            </div>
            <div className="min-[768px]:hidden w-full aspect-[1843/4096] rounded-[32px] overflow-hidden">
              <img alt="" className="size-full object-cover pointer-events-none" src={imgContentMobile} />
            </div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section
        id="features"
        className="scroll-mt-[110px] w-full flex justify-center px-[20px] min-[768px]:px-[40px] min-[1201px]:px-[80px] pt-[96px] min-[1201px]:pt-[128px]"
      >
        <div className="w-full max-w-[1200px] flex flex-col gap-[32px] min-[1201px]:gap-[52px] items-start">
          <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[#222] text-[24px] min-[1201px]:text-[48px] text-center w-full">
            {typo("Возможности")}
          </p>
          <div className="flex flex-col gap-[64px] w-full">
            {FEATURES.map((f) => (
              <Feature key={f.title} {...f} />
            ))}
          </div>
        </div>
      </section>

      {/* Audience */}
      <section
        id="audience"
        className="scroll-mt-[110px] w-full flex justify-center px-[20px] min-[768px]:px-[40px] min-[1201px]:px-[80px] py-[96px] min-[1201px]:py-[128px]"
      >
        <div className="w-full max-w-[1200px] flex flex-col gap-[32px] min-[1201px]:gap-[52px] items-start">
          <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[#222] text-[24px] min-[1201px]:text-[48px] text-center w-full">
            {typo("Для кого сервис")}
          </p>
          <div className="flex flex-col gap-[8px] min-[768px]:gap-[24px] min-[1201px]:flex-row min-[1201px]:items-stretch w-full">
            {AUDIENCE.map(({ Icon, iconBg, cardBg, title, text, titleColor, textColor }) => (
              <div
                key={title}
                className={`${cardBg} rounded-[24px] min-[1201px]:rounded-[32px] min-[1201px]:flex-1 min-[1201px]:min-w-px flex flex-col gap-[24px] min-[1201px]:gap-[64px] items-start p-[24px] min-[1201px]:p-[40px]`}
              >
                <div className={`${iconBg} flex items-center justify-center p-[12px] rounded-[12px] min-[1201px]:rounded-[16px]`}>
                  <Icon />
                </div>
                <div className="flex flex-col gap-[8px] items-start w-full">
                  <p className={`font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[20px] min-[1201px]:text-[32px] w-full ${titleColor}`}>
                    {typo(title)}
                  </p>
                  <p className={`font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 text-[14px] min-[1201px]:text-[18px] w-full ${textColor}`}>
                    {typo(text)}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* FAQ */}
      <section
        id="faq"
        className="scroll-mt-[110px] w-full flex justify-center px-[20px] min-[768px]:px-[40px] min-[1201px]:px-[80px] pb-[96px] min-[1201px]:pb-[128px]"
      >
        <div className="w-full max-w-[1200px] flex flex-col gap-[32px] min-[1201px]:gap-[52px] items-start">
          <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[#222] text-[24px] min-[1201px]:text-[48px] text-center w-full">
            {typo("Частые вопросы")}
          </p>
          <Faq />
        </div>
      </section>

      {/* CTA */}
      <section id="cta" className="w-full">
        <div className="relative rounded-[24px] min-[1201px]:rounded-[64px] w-full overflow-hidden">
          <img alt="" className="absolute inset-0 size-full object-cover pointer-events-none" src={imgCtaBg} />
          <div className="relative flex flex-col items-center px-[20px] py-[128px] min-[1201px]:px-[80px] min-[1201px]:py-[256px]">
            <div className="w-full max-w-[1200px] flex flex-col gap-[52px] items-center">
              <div className="flex flex-col gap-[8px] items-center text-center text-white w-full">
                <p className="font-['Involve:SemiBold',sans-serif] leading-[1.15] text-[32px] min-[1201px]:text-[48px] w-full">
                  {typo("Попробуйте Рентли в деле")}
                </p>
                <p className="font-['Manrope:Medium',sans-serif] font-medium leading-[1.35] opacity-70 text-[18px] w-full">
                  {typo("Добавьте объект и начните отслеживать аренду")}
                </p>
              </div>
              <div className="flex gap-[8px] items-center justify-center max-[767px]:flex-col max-[767px]:w-full">
                <Button variant="white" onClick={onLogin} className="max-[767px]:w-full">
                  {typo("Начать бесплатно")}
                </Button>
                <Button variant="ghost" onClick={onContact} className="max-[767px]:w-full">
                  {typo("Связаться с нами")}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
