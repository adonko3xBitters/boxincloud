#!/usr/bin/env node

/**
 * Vérifie qu'aucun calque ne se referme sur la seule foi d'un `stopPropagation`.
 *
 * Un menu, une popover, une palette : tous se referment quand on clique
 * ailleurs, et tous le font en écoutant `pointerdown` sur `document`. La façon
 * naïve d'épargner le calque lui-même est d'y poser
 * `onPointerDown={(e) => e.stopPropagation()}`. Elle ne marche pas ici.
 *
 * Sous l'App Router, React délègue ses écouteurs à `document`. Le gestionnaire
 * du calque et celui qui le referme vivent donc sur le MÊME nœud, et
 * `stopPropagation()` n'arrête que les nœuds suivants — jamais les colisteners
 * du même nœud (il faudrait `stopImmediatePropagation`, que React n'appelle
 * pas). Conséquence : le calque se démonte au `pointerdown`, l'élément visé
 * disparaît, et le `click` qui suit ne trouve plus personne. Le menu se ferme
 * sans avoir rien fait — exactement ce qu'un utilisateur décrit par « je clique
 * et rien ne s'affiche ».
 *
 * Rien ne signale cette faute : elle compile, elle passe le typage, elle ne
 * lève aucune erreur à l'exécution. Elle ne se voit qu'en cliquant. D'où ce
 * contrôle : la fermeture au clic extérieur passe par `useDismissOnOutside`,
 * qui teste l'appartenance de la cible et ne dépend d'aucun ordre d'inscription.
 */

import { readdir, readFile } from "node:fs/promises";
import { join, relative } from "node:path";

const SRC = join(process.cwd(), "src");
const ALLOWED = new Set([join(SRC, "lib", "dismiss.ts")]);

/** Écouter `pointerdown`/`mousedown` sur `document` pour refermer un calque. */
const GLOBAL_DISMISS = /document\.addEventListener\(\s*["'](pointerdown|mousedown)["']/g;

async function* walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) yield* walk(full);
    else if (/\.(ts|tsx)$/.test(entry.name)) yield full;
  }
}

const offences = [];

for await (const file of walk(SRC)) {
  if (ALLOWED.has(file)) continue;

  const source = await readFile(file, "utf8");
  for (const match of source.matchAll(GLOBAL_DISMISS)) {
    const line = source.slice(0, match.index).split("\n").length;
    offences.push({ file: relative(process.cwd(), file), line, event: match[1] });
  }
}

if (offences.length > 0) {
  console.error("Fermeture au clic extérieur écrite à la main :\n");
  for (const { file, line, event } of offences) {
    console.error(`  ${file}:${line}  document.addEventListener("${event}", …)`);
  }
  console.error(
    "\nUtiliser useDismissOnOutside(open, ref, onClose) depuis @/lib/dismiss.",
    "\nUn stopPropagation posé sur le calque ne protège rien : React délègue",
    "\nses écouteurs à document, donc les deux gestionnaires sont sur le même",
    "\nnœud. Le calque se démonte avant le click, et l'action ne part jamais.\n",
  );
  process.exit(1);
}

console.log("check-overlays : aucune fermeture au clic extérieur écrite à la main.");
