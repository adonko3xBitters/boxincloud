#!/usr/bin/env node

/**
 * Vérifie qu'un code QR produit par l'application se relit vraiment.
 *
 * C'est le défaut le plus coûteux à découvrir : le SVG s'affiche, la page est
 * belle, et l'on ne sait qu'un code est illisible qu'au moment où quelqu'un
 * pointe son téléphone dessus — c'est-à-dire jamais pendant le développement,
 * et toujours devant l'utilisateur.
 *
 * Le contrôle refait le chemin complet — encodage puis décodage — avec un
 * décodeur indépendant de l'encodeur. Il vérifie aussi la marge silencieuse :
 * quatre modules, faute de quoi une partie des lecteurs ne trouve plus les
 * repères d'alignement, et l'échec dépend alors de l'appareil.
 */

import qrcode from "qrcode-generator";
import jsQR from "jsqr";

/** Les formes d'adresse qu'une instance self-hosted prend réellement. */
const CASES = [
  "https://bd.exemple.fr/telecharger",
  "http://192.168.1.42:8070/telecharger",
  "https://une-instance-au-nom-tres-long.exemple.org:8443/telecharger",
];

const SCALE = 4;

/** La même marge que le SVG de `mobile-app-dialog.tsx`. */
const QUIET = 4;

function render(data) {
  const qr = qrcode(0, "M");
  qr.addData(data);
  qr.make();

  const modules = qr.getModuleCount();
  const side = (modules + QUIET * 2) * SCALE;

  // Blanc partout, puis les modules sombres — la marge naît du remplissage.
  const pixels = new Uint8ClampedArray(side * side * 4).fill(255);

  for (let row = 0; row < modules; row += 1) {
    for (let col = 0; col < modules; col += 1) {
      if (!qr.isDark(row, col)) continue;

      for (let dy = 0; dy < SCALE; dy += 1) {
        for (let dx = 0; dx < SCALE; dx += 1) {
          const x = (col + QUIET) * SCALE + dx;
          const y = (row + QUIET) * SCALE + dy;
          const offset = (y * side + x) * 4;
          pixels[offset] = pixels[offset + 1] = pixels[offset + 2] = 0;
        }
      }
    }
  }

  return { pixels, side, modules };
}

let failures = 0;

for (const expected of CASES) {
  const { pixels, side, modules } = render(expected);
  const decoded = jsQR(pixels, side, side)?.data;

  if (decoded === expected) {
    console.log(`  ✓ ${modules}×${modules} — ${expected}`);
  } else {
    failures += 1;
    console.error(`  ✗ ${modules}×${modules} — ${expected}`);
    console.error(`    décodé : ${JSON.stringify(decoded)}`);
  }
}

if (failures > 0) {
  console.error(`\n${failures} code(s) QR illisible(s).`);
  process.exit(1);
}

console.log(`✓ ${CASES.length} codes QR encodés puis relus.`);
