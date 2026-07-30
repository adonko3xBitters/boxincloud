#!/usr/bin/env node
//
// tokens.json → variables CSS (web) + constantes Dart (mobile).
//
// Le web et Flutter lisent la même source. C'est peu de code, et c'est ce qui
// évite que les deux clients divergent visuellement en quelques mois — alors
// que l'UX est le différenciateur revendiqué du projet.
//
//   node build.mjs

import { readFileSync, writeFileSync, mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, "..", "..");

const tokens = JSON.parse(readFileSync(join(here, "tokens.json"), "utf8"));

// ─── Helpers ─────────────────────────────────────────────────────────────────

/** Écarte les clés de documentation du fichier de tokens. */
const isComment = (key) => key.startsWith("$");

/** Résout une référence `{color.brand.600}` vers sa valeur. */
function resolve(value) {
  if (typeof value !== "string") return value;

  return value.replace(/\{([^}]+)\}/g, (_, path) => {
    const found = path.split(".").reduce((acc, key) => acc?.[key], tokens);
    if (found === undefined) {
      throw new Error(`token introuvable : {${path}}`);
    }
    return found;
  });
}

/** Aplatit un objet imbriqué en paires [nom-tiret, valeur]. */
function flatten(obj, prefix = []) {
  const out = [];
  for (const [key, value] of Object.entries(obj)) {
    if (isComment(key)) continue;
    const path = [...prefix, key];
    if (value && typeof value === "object") {
      out.push(...flatten(value, path));
    } else {
      out.push([path.join("-"), resolve(value)]);
    }
  }
  return out;
}

// ─── CSS ─────────────────────────────────────────────────────────────────────

function buildCSS() {
  const lines = [
    "/* Généré depuis packages/design-tokens/tokens.json — ne pas éditer. */",
    "/* Régénérer avec : make generate-tokens */",
    "",
    "@layer theme {",
    "  :root {",
  ];

  const section = (label, pairs) => {
    lines.push(`    /* ${label} */`);
    for (const [name, value] of pairs) {
      lines.push(`    --${name}: ${value};`);
    }
    lines.push("");
  };

  section("Palette", flatten(tokens.color, ["color"]));
  section("Espacement", flatten(tokens.space, ["space"]));
  section("Rayons", flatten(tokens.radius, ["radius"]));
  section("Typographie", flatten(tokens.font, ["font"]));
  section("Ombres", flatten(tokens.shadow, ["shadow"]));
  section("Mouvement", flatten(tokens.motion, ["motion"]));
  section("Mise en page", flatten(tokens.layout, ["layout"]));

  // Le thème clair est la valeur par défaut du document…
  lines.push("    /* Rôles — thème clair */");
  for (const [name, value] of flatten(tokens.semantic.light)) {
    lines.push(`    --${name}: ${value};`);
  }
  lines.push("  }", "");

  // …et le thème sombre s'applique soit par préférence système, soit par
  // l'attribut posé explicitement par le sélecteur de thème.
  lines.push("  @media (prefers-color-scheme: dark) {");
  lines.push("    :root:not([data-theme='light']) {");
  for (const [name, value] of flatten(tokens.semantic.dark)) {
    lines.push(`      --${name}: ${value};`);
  }
  lines.push("    }", "  }", "");

  lines.push("  :root[data-theme='dark'] {");
  for (const [name, value] of flatten(tokens.semantic.dark)) {
    lines.push(`    --${name}: ${value};`);
  }
  lines.push("  }", "");

  // Le choix de l'utilisateur prime toujours sur la préférence système.
  lines.push("  :root[data-theme='light'] {");
  for (const [name, value] of flatten(tokens.semantic.light)) {
    lines.push(`    --${name}: ${value};`);
  }
  lines.push("  }", "");

  // Accessibilité : une animation est un confort, jamais une nécessité.
  //
  // La liste est dérivée des tokens, pas recopiée : une durée ajoutée au
  // fichier source échapperait sinon à l'annulation sans que rien ne le
  // signale, et l'animation continuerait de tourner chez qui l'a désactivée.
  lines.push("  @media (prefers-reduced-motion: reduce) {", "    :root {");
  for (const [name] of flatten(tokens.motion.duration, ["motion", "duration"])) {
    lines.push(`      --${name}: 0ms;`);
  }
  lines.push("    }", "  }", "}", "");

  return lines.join("\n");
}

// ─── Dart ────────────────────────────────────────────────────────────────────

/** `accent-hover` → `accentHover` */
const camel = (s) => s.replace(/-([a-z0-9])/g, (_, c) => c.toUpperCase());

/** `#4f46e5` → `Color(0xFF4F46E5)` */
function dartColor(hex) {
  const m = /^#([0-9a-f]{6})$/i.exec(hex.trim());
  if (!m) return null;
  return `Color(0xFF${m[1].toUpperCase()})`;
}

function buildDart() {
  const lines = [
    "// Généré depuis packages/design-tokens/tokens.json — ne pas éditer.",
    "// Régénérer avec : make generate-tokens",
    "",
    "import 'package:flutter/material.dart';",
    "",
    "/// Rôles de couleur d'un thème.",
    "///",
    "/// Les deux instances ci-dessous sont dérivées du même fichier que les",
    "/// variables CSS du web : les deux clients ne peuvent pas diverger.",
    "class BoxColors {",
  ];

  const roles = flatten(tokens.semantic.light)
    .map(([name]) => name)
    .filter((name) => dartColor(resolve(tokens.semantic.light[name])) !== null);

  for (const name of roles) {
    lines.push(`  final Color ${camel(name)};`);
  }

  lines.push("", "  const BoxColors({");
  for (const name of roles) {
    lines.push(`    required this.${camel(name)},`);
  }
  lines.push("  });", "}", "");

  const themeConst = (label, scheme) => {
    lines.push(`/// Rôles du thème ${label}.`);
    lines.push(`const boxColors${label === "clair" ? "Light" : "Dark"} = BoxColors(`);
    for (const name of roles) {
      const color = dartColor(resolve(scheme[name]));
      lines.push(`  ${camel(name)}: ${color},`);
    }
    lines.push(");", "");
  };

  themeConst("clair", tokens.semantic.light);
  themeConst("sombre", tokens.semantic.dark);

  // Espacement et rayons : les valeurs CSS sont en rem, Flutter travaille en
  // pixels logiques. La base de 16 est celle du navigateur.
  const toPx = (rem) => {
    const m = /^([\d.]+)rem$/.exec(rem);
    if (m) return `${parseFloat(m[1]) * 16}`;
    const px = /^([\d.]+)px$/.exec(rem);
    if (px) return px[1];
    return null;
  };

  lines.push("/// Échelle d'espacement, en pixels logiques.", "class BoxSpace {");
  for (const [name, value] of flatten(tokens.space)) {
    const px = toPx(value) ?? "0";
    lines.push(`  static const double s${name} = ${px};`);
  }
  lines.push("}", "");

  lines.push("/// Rayons de bordure.", "class BoxRadius {");
  for (const [name, value] of flatten(tokens.radius)) {
    const px = toPx(value);
    if (px === null) continue;
    lines.push(`  static const double ${camel(name)} = ${px};`);
  }
  lines.push("}", "");

  return lines.join("\n");
}

// ─── Écriture ────────────────────────────────────────────────────────────────

const targets = [
  {
    path: join(root, "apps", "web", "src", "styles", "tokens.css"),
    content: buildCSS(),
    label: "variables CSS",
  },
  {
    path: join(root, "apps", "mobile", "lib", "shared", "tokens.dart"),
    content: buildDart(),
    label: "constantes Dart",
  },
];

for (const { path, content, label } of targets) {
  try {
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, content, "utf8");
    console.log(`  ✓ ${label} → ${path.replace(root + "/", "")}`);
  } catch (err) {
    // L'application mobile n'existe pas avant M5 : ne pas faire échouer le
    // build du web pour autant.
    console.warn(`  → ${label} ignorées (${err.message})`);
  }
}
