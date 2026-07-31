import { FlatCompat } from "@eslint/eslintrc";

/**
 * Configuration ESLint.
 *
 * Elle manquait, et `npm run lint` échouait donc depuis le début du projet. Le
 * CI ne l'exécutait pas, si bien que rien ne cassait — c'est exactement le
 * genre de dette qui se remarque le jour où un contributeur lance la commande
 * documentée dans CONTRIBUTING.md et tombe sur une erreur.
 *
 * `FlatCompat` fait le pont vers `eslint-config-next`, qui n'est pas encore
 * publié au format plat d'ESLint 9. Le pont disparaîtra quand Next le sera.
 */
const compat = new FlatCompat({ baseDirectory: import.meta.dirname });

const config = [
  {
    // Le code généré n'est pas relu à la main : le corriger n'aurait aucun
    // effet, la prochaine génération l'écrasant.
    ignores: [
      ".next/**",
      "out/**",
      "node_modules/**",
      "src/lib/api/schema.d.ts",
      "next-env.d.ts",
      "src/styles/tokens.css",
    ],
  },

  ...compat.extends("next/core-web-vitals", "next/typescript"),

  {
    rules: {
      /*
        Trois écarts au défaut, chacun pour une raison précise.

        `<img>` plutôt que `next/image` : les pages de bande dessinée sont
        servies par notre propre API, à une largeur que le lecteur décide et
        que le composant de Next ne sait pas suivre. Son optimiseur exigerait
        de surcroît un runtime Node, que l'export statique n'a pas.

        Les variables inutilisées préfixées d'un souligné sont tolérées : c'est
        la convention pour un paramètre qu'une signature impose mais dont le
        corps n'a que faire.

        Les entités non échappées restent une ERREUR. Une apostrophe française
        écrite nue dans du JSX est le défaut le plus courant de ce projet, et
        celui qui casse le rendu le plus discrètement.
      */
      "@next/next/no-img-element": "off",
      "@typescript-eslint/no-unused-vars": [
        "error",
        { argsIgnorePattern: "^_", varsIgnorePattern: "^_" },
      ],
      "react/no-unescaped-entities": "error",
    },
  },
];

export default config;
