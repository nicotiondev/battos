import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
	title: "BattOS - Mission Control",
	description: "Orquestador de Sistema y Mission Control Conversacional Meta de BattOS",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="es" className="h-full antialiased">
      <body className="min-h-full flex flex-col">{children}</body>
    </html>
  );
}
