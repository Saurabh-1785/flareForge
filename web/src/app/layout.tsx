import type { Metadata } from "next";
import { Providers } from "./providers";
import { Header } from "@/components/Header";
import { Footer } from "@/components/Footer";
import "./globals.css";

export const metadata: Metadata = {
  title: "Continuity Vault — Non-Custodial Estate & Business Continuity",
  description:
    "A non-custodial digital-estate and business-continuity protocol for XRP, BTC, and DOGE holders. Built on Flare with FAssets, FDC, and FCC.",
  keywords: [
    "continuity vault",
    "digital estate",
    "crypto inheritance",
    "flare network",
    "fassets",
    "fdc",
    "fcc",
    "xrp",
    "business continuity",
  ],
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <head>
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link
          rel="preconnect"
          href="https://fonts.gstatic.com"
          crossOrigin="anonymous"
        />
        <link
          href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=Outfit:wght@500;600;700;800&family=JetBrains+Mono:wght@400;500;600&display=swap"
          rel="stylesheet"
        />
      </head>
      <body>
        <Providers>
          <Header />
          <main>
            {children}
          </main>
          <Footer />
        </Providers>
      </body>
    </html>
  );
}
