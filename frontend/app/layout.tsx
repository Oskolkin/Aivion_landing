import "./globals.css";
import type { Metadata } from "next";
import { SentryInit } from "@/components/sentry-init";

export const metadata: Metadata = {
  title: "Aivion — AI-ассистент для Ozon-бизнеса",
  description: "Aivion помогает понять, что происходит в Ozon-бизнесе, превращает анализ в конкретные Действия и позволяет поручать аналитические задачи AI-чату.",
  icons: {
    icon: "/icon.svg",
  },
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="ru" className="overflow-x-hidden">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossOrigin="anonymous" />
        <link
          href="https://fonts.googleapis.com/css2?family=Manrope:wght@400..800&display=swap"
          rel="stylesheet"
        />
      </head>
      <body className="max-w-full overflow-x-hidden font-sans antialiased">
        <SentryInit />
        {children}
      </body>
    </html>
  );
}
