import type { Metadata, Viewport } from "next";
import "@/styles/globals.css";
import { Providers } from "@/components/providers";

export const metadata: Metadata = {
  title: "boxincloud",
  description: "Votre bibliothèque de BD, comics et mangas",
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  // Le lecteur gère son propre zoom ; le zoom navigateur y interférerait.
  // Le reste de l'application reste zoomable — c'est un besoin d'accessibilité.
  maximumScale: 5,
  themeColor: [
    { media: "(prefers-color-scheme: light)", color: "#f8fafc" },
    { media: "(prefers-color-scheme: dark)", color: "#020617" },
  ],
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="fr" suppressHydrationWarning>
      <head>
        {/*
          Applique le thème avant le premier rendu.
          Sans cela, une page sombre s'afficherait une fraction de seconde en
          clair avant que React ne prenne la main — le fameux « flash blanc »,
          particulièrement désagréable quand on ouvre l'application le soir.
        */}
        <script
          dangerouslySetInnerHTML={{
            __html: `
try {
  var t = localStorage.getItem('boxincloud.theme');
  if (t === 'light' || t === 'dark') {
    document.documentElement.dataset.theme = t;
  }
} catch (e) {}
`.trim(),
          }}
        />
      </head>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
