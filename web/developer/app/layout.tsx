import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "../../dashboard/app/globals.css";
import "../../dashboard/app/portal.css";

const geistSans = Geist({ variable: "--font-geist-sans", subsets: ["latin"] });
const geistMono = Geist_Mono({ variable: "--font-geist-mono", subsets: ["latin"] });

export const metadata: Metadata = {
  title: "Emisell — Portal Developer",
  description: "Kelola draft aplikasi dan pengajuan review.",
  robots: { index: false, follow: false },
};
export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="id">
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased`}>{children}</body>
    </html>
  );
}
