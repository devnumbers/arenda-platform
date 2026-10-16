import { ImageResponse } from "next/og";
import { readFileSync } from "node:fs";
import { join } from "node:path";

// OG-картинка (1200×630): градиент финального CTA (#88b7ff→#2b7fff),
// 3D-логотип и заголовок. Satori не читает woff2 — рядом лежат
// assets/onest-og-400.ttf / onest-og-600.ttf (срез cyrillic из
// public/fonts, конверт fonttools; шрифт OFL).
export const alt = "Рентли — сервис управления арендой недвижимости";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function OgImage() {
  const regular = readFileSync(join(process.cwd(), "assets/onest-og-400.ttf"));
  const semibold = readFileSync(join(process.cwd(), "assets/onest-og-600.ttf"));
  const logo = `data:image/png;base64,${readFileSync(
    join(process.cwd(), "app/icon.png"),
  ).toString("base64")}`;

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          gap: 40,
          background: "linear-gradient(180deg, #88b7ff 0%, #2b7fff 100%)",
          color: "#fff",
          fontFamily: "Onest",
        }}
      >
        {/* Данные из icon.png; next/image в OG-рендере недоступен */}
        <img
          src={logo}
          width={156}
          height={156}
          style={{ borderRadius: 36 }}
          alt=""
        />
        <div
          style={{
            display: "flex",
            fontSize: 64,
            fontWeight: 600,
            lineHeight: 1.1,
            textAlign: "center",
            maxWidth: 900,
          }}
        >
          Сервис управления арендой недвижимости
        </div>
        <div style={{ display: "flex", fontSize: 32, opacity: 0.85 }}>
          Без таблиц и заметок — объекты, платежи и договоры в одном месте
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [
        { name: "Onest", data: regular, weight: 400, style: "normal" },
        { name: "Onest", data: semibold, weight: 600, style: "normal" },
      ],
    },
  );
}
