import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "../../dashboard/app/globals.css";
import "../../dashboard/app/portal.css";
import "../../dashboard/app/developer-redesign.css";
import "../../dashboard/app/catalog-preview.css";
import "../../dashboard/app/install-picker.css";
import "../../dashboard/app/documentation.css";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Emisell Docs — Bangun aplikasi untuk Emisell",
  description: "Panduan CLI, credential, scope, dan pengajuan aplikasi.",
  robots: { index: true, follow: true },
};
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased`}>{children}</body>
    </html>
  );
}
