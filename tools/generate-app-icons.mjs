#!/usr/bin/env node
//
// apps/web/public/icon.svg → icônes de lancement Android.
//
//   node tools/generate-app-icons.mjs
//
// Une seule source pour le web et le mobile. L'application Android a été
// publiée en 0.1.0 et 0.1.1 avec le logo bleu du gabarit Flutter : le projet
// avait une icône, elle n'était simplement branchée nulle part.
//
// Ce script n'est PAS lancé par la CI, contrairement aux autres générateurs du
// dépôt : il dépend de `sharp`, qui vit dans les dépendances du web et qui
// n'est pas installé dans le travail mobile. Les PNG produits sont donc
// versionnés, et la CI vérifie seulement qu'ils ne sont pas redevenus ceux du
// gabarit — ce qui est le défaut qu'on veut interdire, pas la seule dérive
// concevable.
//
// À relancer quand `icon.svg` change.

import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, "..");

// sharp s'appuie sur librsvg. Le moteur SVG interne d'ImageMagick a été essayé
// d'abord : il place mal la planche pivotée et laisse tomber le tracé qui
// dessine les arêtes de la boîte, ce qui donne un hexagone plat. Un rendu faux
// mais plausible est pire qu'une erreur — celui-ci se serait vu à l'install.
const require = createRequire(join(root, "apps", "web", "package.json"));
let sharp;
try {
    sharp = require("sharp");
} catch {
    console.error(
        "sharp est introuvable. Il vient des dépendances du web :\n" +
            "    cd apps/web && npm install\n",
    );
    process.exit(1);
}

const source = join(root, "apps", "web", "public", "icon.svg");
const resDir = join(root, "apps", "mobile", "android", "app", "src", "main", "res");

const svg = readFileSync(source, "utf8");

// ─── Séparer le fond du motif ────────────────────────────────────────────────
//
// Une icône adaptative (Android 8+) est faite de deux couches que le système
// anime et masque indépendamment. Le fond doit donc être une couleur, et le
// motif un calque transparent — les aplatir ensemble donnerait un carré collé
// au milieu du masque rond.

const backgroundRect = /<rect\s+width="32"\s+height="32"[^>]*fill="(#[0-9a-fA-F]{6})"[^>]*\/>/;
const match = svg.match(backgroundRect);
if (!match) {
    console.error(
        "Le rectangle de fond 32×32 est introuvable dans icon.svg.\n" +
            "Le SVG a changé de structure : adaptez ce script plutôt que de\n" +
            "produire une icône adaptative dont le fond serait faux.",
    );
    process.exit(1);
}
const backgroundColor = match[1];
const artwork = svg.replace(backgroundRect, "");

// La zone sûre d'une icône adaptative est un disque de 66 dp au centre d'un
// canevas de 108 : le système rogne tout le reste, et le rognage varie selon
// le constructeur. Le motif de 32 unités est donc ramené à 66.
const SAFE = 66;
const CANVAS = 108;
const scale = SAFE / 32;
const offset = (CANVAS - SAFE) / 2;

const foregroundSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${CANVAS} ${CANVAS}">
  <g transform="translate(${offset} ${offset}) scale(${scale})">
${artwork.replace(/<svg[^>]*>|<\/svg>/g, "").trim()}
  </g>
</svg>`;

// ─── Les cinq densités ───────────────────────────────────────────────────────

const densities = [
    { dir: "mipmap-mdpi", launcher: 48, foreground: 108 },
    { dir: "mipmap-hdpi", launcher: 72, foreground: 162 },
    { dir: "mipmap-xhdpi", launcher: 96, foreground: 216 },
    { dir: "mipmap-xxhdpi", launcher: 144, foreground: 324 },
    { dir: "mipmap-xxxhdpi", launcher: 192, foreground: 432 },
];

// Rendre grand puis réduire : librsvg rastérise au plus près de la taille
// demandée, et une icône de 48 px rendue directement perd ses arêtes fines.
const render = (input, size) =>
    sharp(Buffer.from(input), { density: 2400 })
        .resize(size, size, { fit: "contain", background: { r: 0, g: 0, b: 0, alpha: 0 } })
        .png({ compressionLevel: 9 })
        .toBuffer();

let written = 0;
for (const { dir, launcher, foreground } of densities) {
    const target = join(resDir, dir);
    mkdirSync(target, { recursive: true });

    writeFileSync(join(target, "ic_launcher.png"), await render(svg, launcher));
    writeFileSync(join(target, "ic_launcher_foreground.png"), await render(foregroundSvg, foreground));
    written += 2;
}

// ─── Les déclarations ────────────────────────────────────────────────────────

const anydpi = join(resDir, "mipmap-anydpi-v26");
mkdirSync(anydpi, { recursive: true });

// `monochrome` alimente les icônes thématiques d'Android 13. Le motif est déjà
// une silhouette lisible d'un seul tenant ; le système n'en garde que l'alpha.
const adaptiveIcon = `<?xml version="1.0" encoding="utf-8"?>
<!-- Généré par tools/generate-app-icons.mjs — ne pas modifier à la main. -->
<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android">
    <background android:drawable="@color/ic_launcher_background" />
    <foreground android:drawable="@mipmap/ic_launcher_foreground" />
    <monochrome android:drawable="@mipmap/ic_launcher_foreground" />
</adaptive-icon>
`;
writeFileSync(join(anydpi, "ic_launcher.xml"), adaptiveIcon);

const values = join(resDir, "values");
mkdirSync(values, { recursive: true });
writeFileSync(
    join(values, "ic_launcher_background.xml"),
    `<?xml version="1.0" encoding="utf-8"?>
<!-- Généré par tools/generate-app-icons.mjs — ne pas modifier à la main.
     Reprise du rectangle de fond d'apps/web/public/icon.svg. -->
<resources>
    <color name="ic_launcher_background">${backgroundColor}</color>
</resources>
`,
);

console.log(`${written} PNG écrits sur ${densities.length} densités.`);
console.log(`Icône adaptative déclarée, fond ${backgroundColor}.`);
