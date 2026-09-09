import type { Metadata } from 'next';
import { Geist, Geist_Mono } from 'next/font/google';
import './globals.css';
import './portal.css';
import './admin-redesign.css';
import './developer-redesign.css';
import './catalog-preview.css';
import './install-picker.css';
import './documentation.css';

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
});

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
});

export const metadata: Metadata = {
  title: 'Emisell Docs — Bangun aplikasi untuk Emisell',
  description:
    'Panduan Emisell CLI, credential aplikasi, scope, dan pengajuan versi untuk developer.',
  robots: { index: true, follow: true },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="id">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
