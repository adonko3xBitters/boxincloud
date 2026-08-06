#!/usr/bin/env node

/**
 * Vérifie qu'aucun élément cliquable n'en contient un autre.
 *
 * # Ce que ça coûte, concrètement
 *
 * Une ligne de tableau cliquable qui porte des boutons d'action donne un
 * `<button>` dans un `<button>`. Le HTML l'interdit, mais React construit le
 * DOM par `createElement` et non par l'analyseur : rien ne proteste. Ce qu'on
 * obtient est pire qu'une erreur — un comportement dont on ne comprend rien.
 *
 * Le clic sur « Pause » déclenche l'action ET le gestionnaire de la ligne : le
 * panneau de détail s'ouvre à chaque tentative. Comme le bouton se désactive le
 * temps de la commande, le clic suivant n'atteint plus que la ligne, qui se
 * referme. On finit par mettre en pause sans savoir comment — et c'est ainsi
 * que le défaut a été signalé, après avoir été publié.
 *
 * # Pourquoi un contrôle, et pas un linter
 *
 * Rien ne le voit. Ça compile, ça passe le typage, ça ne lève rien à
 * l'exécution, et les règles d'accessibilité d'ESLint ne suivent pas
 * l'imbrication à travers un composant. Seul un clic le révèle.
 *
 * # Ce qui est cherché
 *
 * Le contrôle procède en DEUX PASSES, et la première est celle qui compte.
 *
 * L'imbrication ne se voit presque jamais sur place : personne n'écrit un
 * `<button>` littéral dans un autre. Elle traverse un composant — `<Row>` rend
 * un bouton autour de ses enfants, et l'appelant y met des boutons d'action
 * sans jamais voir le premier. Une lecture fichier par fichier ne trouve rien.
 *
 * Passe 1 : repérer les composants qui rendent un élément cliquable AUTOUR de
 * leurs enfants. Ce sont eux, les pièges.
 * Passe 2 : chercher un élément cliquable à l'intérieur de l'un d'eux.
 *
 * La règle : ce qui se déplie a son propre bouton, dans une cellule, jamais la
 * ligne entière.
 */

import { readdir, readFile } from "node:fs/promises";
import { join, relative } from "node:path";

const SRC = join(process.cwd(), "src");

/*
  Les composants qui rendent un élément cliquable.

  Une liste tenue à la main, parce qu'aucune analyse du texte ne peut savoir ce
  que rend `<Foo>`. Elle est courte et le restera : un projet qui multiplie les
  façons d'être cliquable a un problème plus grave que celui-ci.
*/
const CLICKABLE = ["button", "a", "Button", "ActionButton", "ConfirmButton", "PanelAction", "Disclosure"];

/*
  Passe 1 : les composants qui enveloppent leurs enfants d'un élément cliquable.

  Le motif cherché est précis : une balise cliquable dont le contenu comprend
  `{children}`. C'est ce qui distingue un conteneur — qui accueillera n'importe
  quoi, y compris des boutons — d'un bouton ordinaire, dont le contenu est
  connu de celui qui l'écrit.
*/
function clickableContainers(sources) {
  const found = [];

  for (const [, source] of sources) {
    for (const match of source.matchAll(/<(button|a)[\s>]/g)) {
      const open = endOfOpeningTag(source, match.index);
      if (!open || open.selfClosing) continue;

      const close = source.indexOf(`</${match[1]}>`, open.end);
      if (close === -1) continue;
      if (!source.slice(open.end, close).includes("{children}")) continue;

      // Le composant englobant est la dernière déclaration au-dessus.
      const before = source.slice(0, match.index);
      const declarations = [...before.matchAll(/function\s+([A-Z]\w*)\s*[(<]/g)];
      const owner = declarations.at(-1)?.[1];
      if (owner) found.push(owner);
    }
  }

  return found;
}



/*
  Les commentaires sont NEUTRALISÉS, pas ignorés.

  Ce fichier explique le défaut en citant du JSX ; le contrôle se signalait
  lui-même. Remplacer par des espaces plutôt que supprimer garde les numéros de
  ligne justes, ce qui est tout l'intérêt du message d'erreur.
*/
function withoutComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, (block) => block.replace(/[^\n]/g, " "));
}

/*
  Trouve le `>` qui ferme une balise ouvrante.

  Pas le premier rencontré : une flèche de fonction en attribut —
  `onClick={() => …}` — en contient un, et s'y arrêter faisait croire à une
  balise auto-fermante là où il n'y en avait pas. C'est ce qui rendait le
  contrôle bruyant sur les vrais fichiers, et donc inutilisable.

  On suit donc les accolades et les chaînes, et on ne retient qu'un `>` de
  niveau zéro.
*/
function endOfOpeningTag(source, start) {
  let depth = 0;
  let quote = null;

  for (let i = start; i < source.length; i++) {
    const c = source[i];

    if (quote) {
      if (c === quote) quote = null;
      continue;
    }

    if (c === '"' || c === "'" || c === "`") quote = c;
    else if (c === "{") depth++;
    else if (c === "}") depth--;
    else if (c === ">" && depth === 0) {
      return { end: i, selfClosing: source[i - 1] === "/" };
    }
  }

  return null;
}

async function* walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) yield* walk(full);
    else if (/\.tsx$/.test(entry.name)) yield full;
  }
}

const sources = new Map();
for await (const file of walk(SRC)) {
  sources.set(file, withoutComments(await readFile(file, "utf8")));
}

const containers = [...new Set(clickableContainers(sources))];
const inner = [...new Set([...CLICKABLE, ...containers])];
const opens = new RegExp(
  String.raw`<(button|a|${[...CLICKABLE.filter((n) => /^[A-Z]/.test(n)), ...containers].join("|")})[\s>]`,
  "g",
);

const offences = [];

for (const [file, source] of sources) {
  for (const outer of source.matchAll(opens)) {
    const start = outer.index;
    const tag = outer[1];

    const open = endOfOpeningTag(source, start);
    if (!open) continue;

    const attributes = source.slice(start, open.end);
    if (tag === "a" && !attributes.includes("onClick")) continue;

    // Une balise auto-fermante n'a pas de contenu.
    if (open.selfClosing) continue;

    const attributesEnd = open.end;

    // Portée : jusqu'à la balise fermante correspondante. Approximation
    // assumée — la première fermeture du même nom — qui suffit tant qu'un
    // composant ne s'imbrique pas dans lui-même, ce qu'aucun ici ne fait.
    const close = source.indexOf(`</${tag}>`, attributesEnd);
    if (close === -1) continue;
    const body = source.slice(attributesEnd, close);

    for (const name of inner) {
      if (!new RegExp(String.raw`<${name}[\s>/]`).test(body)) continue;
      offences.push({
        file: relative(process.cwd(), file),
        line: source.slice(0, start).split("\n").length,
        outer: tag,
        inner: name,
      });
      break;
    }
  }
}

if (offences.length > 0) {
  console.error("Élément cliquable imbriqué dans un autre :\n");
  for (const { file, line, outer, inner } of offences) {
    console.error(`  ${file}:${line}  <${outer}> contient <${inner}>`);
  }
  console.error(
    "\nUn <button> dans un <button> est du HTML invalide, et React le construit",
    "\nquand même. Le clic intérieur déclenche les DEUX gestionnaires : l'action",
    "\npart, et le conteneur bascule par-dessus. Rien ne le signale — ni le",
    "\ntypage, ni l'exécution.",
    "\n\nDonner son propre bouton à ce qui se déplie, au lieu de rendre le",
    "\nconteneur cliquable en entier.\n",
  );
  process.exit(1);
}

console.log("check-nested-buttons : aucun élément cliquable imbriqué.");
