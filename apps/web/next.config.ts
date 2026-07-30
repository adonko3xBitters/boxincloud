import type { NextConfig } from "next";

// Export statique : le bundle est embarqué dans le binaire Go (ADR-003).
//
// Le SSR n'apporterait rien ici — l'application est entièrement derrière
// authentification et fortement interactive — et imposerait un runtime Node à
// côté du serveur Go, donc un second conteneur et une seconde surface de mise
// à jour.
const config: NextConfig = {
  output: "export",

  // Le serveur Go replie toute route inconnue sur index.html ; le routeur
  // client prend ensuite le relais.
  trailingSlash: false,

  // next/image suppose un optimiseur côté serveur, qui n'existe pas en export
  // statique. Les vignettes sont déjà produites aux bonnes tailles par le
  // serveur : rien à optimiser côté client.
  images: { unoptimized: true },

  eslint: { ignoreDuringBuilds: true },
  typescript: { ignoreBuildErrors: false },
};

export default config;
