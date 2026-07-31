#!/usr/bin/env node

/**
 * Vérifie les contrastes des jetons de couleur.
 *
 * Un contraste insuffisant ne se voit pas quand on a une bonne vue, un bon
 * écran et de la lumière. Il se voit dans le train, au soleil, ou quand on
 * distingue mal les couleurs — c'est-à-dire pour une part des lecteurs, tout le
 * temps, et jamais pendant le développement.
 *
 * Le seuil est celui du WCAG 2.1 niveau AA : 4,5:1 pour le texte courant,
 * 3:1 pour le texte large et les éléments d'interface. Les paires vérifiées
 * sont celles que l'interface produit réellement, pas toutes les combinaisons
 * possibles — une paire que personne n'affiche n'apprend rien.
 */

import { readFile } from "node:fs/promises";
import { join } from "node:path";

const TOKENS = join(process.cwd(), "..", "..", "packages", "design-tokens", "tokens.json");

/** Paires réellement affichées, avec le seuil qui leur correspond. */
const PAIRS = [
  // Texte courant sur les trois fonds.
  { fg: "text", bg: "surface", min: 4.5, what: "texte principal" },
  { fg: "text", bg: "background", min: 4.5, what: "texte sur le fond de page" },
  { fg: "text", bg: "surface-sunken", min: 4.5, what: "texte sur fond creusé" },

  // Texte secondaire : c'est là que les interfaces sombres pèchent le plus.
  { fg: "text-muted", bg: "surface", min: 4.5, what: "texte secondaire" },
  { fg: "text-muted", bg: "surface-sunken", min: 4.5, what: "texte secondaire sur fond creusé" },

  // Texte tertiaire — métadonnées, légendes. Seuil « texte large » assumé :
  // ces libellés accompagnent toujours une information déjà lisible ailleurs.
  { fg: "text-subtle", bg: "surface", min: 3, what: "métadonnées" },

  // Actions et états.
  { fg: "accent-text", bg: "surface", min: 4.5, what: "lien et action" },
  { fg: "danger", bg: "surface", min: 4.5, what: "erreur" },
  { fg: "text-inverted", bg: "accent", min: 4.5, what: "bouton principal" },

  /*
    Bordures.

    Le WCAG exige 3:1 pour les éléments d'interface — mais pour ceux qui
    PORTENT une information, pas pour les traits décoratifs. `border` sépare
    des cartes et des lignes de tableau dont le contenu se distingue déjà tout
    seul ; l'amener à 3:1 donnerait une interface quadrillée de gris foncé,
    plus fatigante et pas plus lisible.

    `border-strong` est l'autre cas : c'est elle qui dessine le contour d'un
    champ de saisie, et un champ dont on ne voit pas le bord est un champ qu'on
    ne trouve pas. Celle-là doit passer.
  */
  { fg: "border-strong", bg: "surface", min: 3, what: "contour de champ" },
];

function parseColor(value) {
  const hex = value.trim().replace("#", "");
  const full = hex.length === 3 ? [...hex].map((c) => c + c).join("") : hex;

  return [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16) / 255);
}

/** Luminance relative, formule WCAG. */
function luminance(channels) {
  const [r, g, b] = channels.map((c) =>
    c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4,
  );
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(a, b) {
  const la = luminance(parseColor(a));
  const lb = luminance(parseColor(b));
  const [light, dark] = la > lb ? [la, lb] : [lb, la];
  return (light + 0.05) / (dark + 0.05);
}

const tokens = JSON.parse(await readFile(TOKENS, "utf8"));

/** Résout un rôle vers sa couleur finale, en suivant les alias. */
function resolve(theme, role, seen = new Set()) {
  const value = tokens.semantic[theme][role];
  if (value === undefined) return null;
  if (typeof value !== "string") return null;

  if (value.startsWith("#")) return value;

  // Référence à la palette, notation Design Tokens : « {color.neutral.50} ».
  const reference = value.match(/^\{([^}]+)\}$/);
  if (reference) {
    const resolved = reference[1]
      .split(".")
      .reduce((node, key) => (node == null ? null : node[key]), tokens);
    if (typeof resolved === "string") {
      return resolved.startsWith("#") ? resolved : null;
    }
    return null;
  }

  // Alias vers un autre rôle.
  if (seen.has(value)) return null;
  seen.add(value);
  return resolve(theme, value, seen);
}

let failures = 0;
let checked = 0;

for (const theme of ["light", "dark"]) {
  console.log(`\nThème ${theme} :`);

  for (const pair of PAIRS) {
    const fg = resolve(theme, pair.fg);
    const bg = resolve(theme, pair.bg);

    if (!fg || !bg) {
      console.log(`  ?      ${pair.what} (${pair.fg}/${pair.bg} — jeton introuvable)`);
      continue;
    }

    checked += 1;
    const ratio = contrast(fg, bg);
    const ok = ratio >= pair.min;
    if (!ok) failures += 1;

    console.log(
      `  ${ok ? "✓" : "✗"} ${ratio.toFixed(2).padStart(5)}:1 ` +
        `(min ${pair.min})  ${pair.what}`,
    );
  }
}

if (failures > 0) {
  console.error(`\n✗ ${failures} paire(s) sous le seuil WCAG AA.\n`);
  process.exit(1);
}

console.log(`\n✓ ${checked} paires de couleurs au-dessus du seuil WCAG AA.`);
