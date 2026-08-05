#!/usr/bin/env node

/**
 * Mesure la couverture de la traduction.
 *
 * L'internationalisation d'une interface écrite en français est un travail
 * mécanique et long. Le risque n'est pas de mal le faire, c'est de croire
 * l'avoir fini : une chaîne oubliée dans un panneau rarement ouvert ne se voit
 * qu'au moment où quelqu'un l'ouvre, dans une langue qu'il ne lit pas.
 *
 * Ce contrôle compte donc ce qui reste. Il ne bloque pas la construction — un
 * chantier en cours n'est pas une régression — mais il **échoue si le compte
 * augmente**. La traduction ne peut alors que progresser : une nouvelle chaîne
 * en dur dans un composant fait échouer l'intégration continue de celui qui
 * l'a écrite, au moment où il a le contexte pour la traduire.
 *
 * Le seuil est écrit dans `i18n-baseline.json`, à la racine du répertoire web.
 * Il se met à jour en même temps qu'on traduit, jamais dans l'autre sens.
 */

import { readdir, readFile, writeFile } from "node:fs/promises";
import { join, relative } from "node:path";

const SRC = join(process.cwd(), "src");
const BASELINE = join(process.cwd(), "i18n-baseline.json");

/*
Ce qui ne compte pas.

Le catalogue, français par construction. Et `schema.d.ts`, engendré depuis le
contrat OpenAPI : ses trois cents chaînes sont les descriptions du contrat, qui
restent en français comme le reste de la documentation du projet. Les laisser
dans le compte le rendrait insensible au travail réel — on aurait traduit toute
l'interface sans que le chiffre bouge de moitié.
*/
const IGNORED = new Set([join(SRC, "i18n", "fr.tsx")]);

/*
Seuls les fichiers `.tsx` sont parcourus.

C'est là que vit le texte affiché. Les `.ts` — client d'API, jetons, réglages —
contiennent surtout des identifiants que la majuscule initiale suffisait à
faire passer pour du français : « POST », « Tokens », « Device ». Les compter
noyait la mesure sous des faux positifs qu'aucune traduction ne concerne.

Les rares chaînes visibles qui y traînent — noms de plateformes, messages
d'erreur du client — sont traitées en même temps que le reste, sans que le
compteur ait besoin de les voir.
*/

/**
 * Une chaîne littérale contenant du texte destiné à être lu.
 *
 * L'heuristique : une majuscule ou un caractère accentué, au moins quatre
 * caractères, et pas de marqueur de code. Elle rate des cas et en invente
 * d'autres — c'est acceptable pour une mesure de tendance, qui n'a besoin que
 * d'être stable d'une exécution à l'autre.
 */
const TEXT = /["'`]([A-ZÀ-Üa-zà-ü][^"'`\n]{3,})["'`]/g;

/*
Texte nu dans le JSX : `<button>Continuer</button>`.

Il ne porte aucun guillemet et échappait donc entièrement au contrôle
précédent — qui annonçait zéro chaîne restante alors qu'il n'avait simplement
jamais regardé là. C'est le genre de trou qui rend une mesure rassurante et
fausse, ce qui est pire que pas de mesure du tout.

L'expression exige une majuscule ou un accent, et refuse ce qui contient une
accolade : `{t("clé")}` est déjà traduit, et le compter serait absurde.
*/
/*
  Le `>` d'une fermeture de balise, jamais celui d'une flèche.

  Sans la garde, `=> Promise<unknown>` est lu comme du texte JSX : le `>` de la
  flèche ouvre la capture, `Promise` la remplit, et le `<` du générique la
  ferme. Le contrôle signalait donc une signature TypeScript comme une chaîne
  française à traduire.
*/
const JSX_TEXT = /(?<!=)>\s*([A-ZÀ-Üa-zà-ü][^<>{}\n]{2,}?)\s*</g;

/** Ce qui ressemble à du texte mais n'en est pas. */
const NOT_TEXT = [
  /^[a-z-]+$/, // identifiants, clés, noms de classes simples
  /[<>{}]/, // fragments de balisage
  /^\//, // chemins
  /^https?:/, // URL
  /^@\//, // imports
  /^[a-z]+([A-Z][a-z]+)+$/, // camelCase
  /\b(flex|grid|rounded|border|text-|bg-|px-|py-|size-|gap-|font-)/, // Tailwind
  /^use (client|server)$/,
  /^[a-z]+\.[a-z.]+$/, // clés de catalogue déjà extraites

  // Tracés SVG : « M8 4v8M4 8h8 ». Ils commencent par une majuscule et
  // contiennent des espaces, ce qui les faisait passer pour des phrases.
  /^[MmLlHhVvCcSsQqTtAaZz][\d\s.,+-]*[MmLlHhVvCcSsQqTtAaZz\d\s.,+-]*$/,

  // Noms de touches et de balises, comparés tels quels par le DOM. Les
  // traduire casserait le clavier.
  /^(Arrow(Up|Down|Left|Right)|Page(Up|Down)|Home|End|Escape|Enter|Tab|Backspace|Delete|Shift|Control|Meta|Alt)$/,
  /^(INPUT|TEXTAREA|SELECT|BUTTON)$/,

  // Constante du DOM : `dataTransfer.types` contient littéralement « Files ».
  /^Files$/,

  // Noms de variables d'environnement, affichés tels quels dans un `<code>`
  // pour qu'on puisse les recopier : « BOXINCLOUD_ED2K_ENABLED=true ». Ce sont
  // des identifiants que le serveur lit ; les traduire produirait une
  // configuration qui ne marche pas, et la moitié de leur utilité est de
  // pouvoir être copiés au caractère près.
  /^[A-Z][A-Z0-9_]*(=\S*)?$/,

  // Noms de langues, écrits dans leur propre langue et jamais traduits :
  // quelqu'un qui ne lit pas la langue courante doit reconnaître la sienne,
  // ce que « Anglais » ne permet pas.
  /^(Français|English)$/,
];

/*
Deux exceptions, permanentes et assumées.

La description dans `layout.tsx` est une balise `<meta>` écrite à la
construction. L'export statique produit un seul HTML servi à tout le monde :
elle ne PEUT pas suivre la langue du visiteur, et la traduire à l'exécution ne
changerait rien à ce que lit un moteur de recherche. Elle reste en français,
comme la langue par défaut du projet.

Ce n'est pas un oubli qu'on cache : c'est une limite du rendu statique, nommée
ici pour qu'on ne la redécouvre pas dans six mois.
*/
const EXEMPT = new Set([join(SRC, "app", "layout.tsx")]);

/** Un caractère accentué ou une majuscule initiale suffit à trahir du français. */
const FRENCH = /[éèêëàâäîïôöùûüçœæÉÈÊÀÂÎÔÙÛÇ]|^[A-ZÀ-Ü]/;

async function* walk(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) yield* walk(full);
    else if (/\.tsx$/.test(entry.name)) yield full;
  }
}

const perFile = new Map();
let total = 0;

for await (const file of walk(SRC)) {
  if (IGNORED.has(file) || EXEMPT.has(file)) continue;

  const source = await readFile(file, "utf8");

  // Les commentaires sont retirés : ils restent en français, c'est la règle du
  // projet, et les compter noierait la mesure.
  const code = source
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");

  let count = 0;
  for (const pattern of [TEXT, JSX_TEXT]) {
    for (const match of code.matchAll(pattern)) {
      const value = match[1].trim();
      if (NOT_TEXT.some((rule) => rule.test(value))) continue;
      if (!FRENCH.test(value)) continue;
      count += 1;
    }
  }

  if (count > 0) {
    perFile.set(relative(process.cwd(), file), count);
    total += count;
  }
}

let baseline = Number.POSITIVE_INFINITY;
try {
  baseline = JSON.parse(await readFile(BASELINE, "utf8")).remaining;
} catch {
  // Premier passage : le compte courant devient la référence.
}

const ranked = [...perFile.entries()].sort((a, b) => b[1] - a[1]);

console.log(`Chaînes françaises encore en dur : ${total}`);
if (Number.isFinite(baseline)) console.log(`Référence : ${baseline}`);
console.log("\nLes dix fichiers les plus concernés :");
for (const [file, count] of ranked.slice(0, 10)) {
  console.log(`  ${String(count).padStart(4)}  ${file}`);
}

if (total > baseline) {
  console.error(
    `\n✗ ${total - baseline} chaîne(s) non traduite(s) de plus que la référence.` +
      `\n  Passez-les par le catalogue (src/i18n/fr.ts) plutôt que de les écrire en dur.\n`,
  );
  process.exit(1);
}

if (total < baseline || !Number.isFinite(baseline)) {
  await writeFile(BASELINE, JSON.stringify({ remaining: total }, null, 2) + "\n");
  console.log(`\n✓ Référence abaissée à ${total}.`);
} else {
  console.log("\n✓ Aucune régression.");
}
