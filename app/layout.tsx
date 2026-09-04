import type { Metadata } from 'next';
import { Geist, Geist_Mono } from 'next/font/google';
import './globals.css';
import { AppPlatformProvider } from './app-platform-store';

const geistSans = Geist({
  variable: '--font-geist-sans',
  subsets: ['latin'],
});

const geistMono = Geist_Mono({
  variable: '--font-geist-mono',
  subsets: ['latin'],
});

export const metadata: Metadata = {
  metadataBase: new URL(
    process.env.NEXT_PUBLIC_SITE_URL ?? 'http://localhost:3003',
  ),
  title: {
    default: 'Emisell App Platform',
    template: '%s · Emisell App Platform',
  },
  description:
    'Build, manage, and release commerce apps and extensions across the Emisell ecosystem.',
  openGraph: {
    title: 'Emisell App Platform',
    description: 'Build. Extend. Scale.',
    images: [{ url: '/og.png', width: 1200, height: 630 }],
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Emisell App Platform',
    description: 'Build. Extend. Scale.',
    images: ['/og.png'],
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <AppPlatformProvider>{children}</AppPlatformProvider>
      </body>
    </html>
  );
}
