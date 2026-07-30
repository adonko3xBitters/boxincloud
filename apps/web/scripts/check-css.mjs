#!/usr/bin/env node

/**
 * Vérifie que la feuille de style compilée ne contient pas de valeur invalide.
 *
 * Il existe une faute qu'aucun outil ne signale : écrire une classe Tailwind qui
 * référence une variable CSS avec la mauvaise syntaxe. `duration-[--ma-duree]`
 * compile sans avertissement et produit `transition-duration: --ma-duree`, une
 * valeur que le navigateur écarte en silence. La transition disparaît, le build
 * passe, les tests passent, et le défaut ne se voit qu'à l'œil — sur une machine
 * où quelqu'un pense à regarder.
 *
 * D'où ce contrôle : une déclaration dont la valeur commence par `--` sans
 * `var()` est nécessairement morte. La forme correcte est `duration-(--x)`, ou
 * mieux, un utilitaire nommé déclaré dans `@theme`.
 */

import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";

const CSS_DIR = join(process.cwd(), "out", "_next", "static", "css");

/**
 * Une valeur de propriété réduite à `--nom`.
 *
 * On exclut les déclarations de variables (`--x: --y` est licite comme jeton
 * arbitraire) en exigeant que la propriété soit une propriété CSS standard.
 */
const DEAD_VALUE = /(?<![-\w])([a-z-]+)\s*:\s*(--[a-z0-9-]+)\s*[;}]/gi;

async function main() {
  let files;
  try {
    files = (await readdir(CSS_DIR)).filter((name) => name.endsWith(".css"));
  } catch {
    console.error(`✗ aucune feuille compilée dans ${CSS_DIR} — lancer le build d'abord`);
    process.exit(1);
  }

  const problems = [];

  for (const file of files) {
    const css = await readFile(join(CSS_DIR, file), "utf8");

    for (const match of css.matchAll(DEAD_VALUE)) {
      const [, property, value] = match;
      // Les propriétés personnalisées peuvent légitimement porter ce genre de
      // valeur ; seules les propriétés standard sont concernées.
      if (property.startsWith("--")) continue;

      problems.push({ file, property, value, at: match.index ?? 0 });
    }
  }

  if (problems.length > 0) {
    console.error("✗ valeurs CSS mortes : la variable n'est pas déréférencée.\n");
    for (const { file, property, value } of problems.slice(0, 20)) {
      console.error(`  ${file} → ${property}: ${value}`);
    }
    if (problems.length > 20) {
      console.error(`  … et ${problems.length - 20} autres`);
    }
    console.error(
      "\n  Dans une classe Tailwind, une variable se référence entre parenthèses :",
      "\n    duration-[--ma-duree]  ✗   valeur invalide, écartée en silence",
      "\n    duration-(--ma-duree)  ✓",
      "\n    duration-slow          ✓   utilitaire nommé, déclaré dans @theme",
    );
    process.exit(1);
  }

  console.log(`✓ ${files.length} feuille(s) CSS sans valeur morte`);
}

await main();
