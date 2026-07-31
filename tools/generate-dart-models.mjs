#!/usr/bin/env node

/**
 * Génère les modèles Dart depuis api/openapi.yaml.
 *
 * Pourquoi un générateur maison plutôt qu'openapi-generator : ce dernier exige
 * une machine virtuelle Java, dépendance lourde à imposer à quiconque veut
 * contribuer au client mobile — et absente de bien des postes. Le dépôt écrit
 * déjà son propre générateur pour les tokens de design ; celui-ci suit le même
 * principe, avec zéro dépendance nouvelle.
 *
 * Il ne couvre volontairement PAS tout OpenAPI. Il couvre ce que ce contrat-ci
 * utilise : objets, tableaux, $ref, énumérations de chaînes, types scalaires et
 * champs requis. Une construction non gérée fait ÉCHOUER la génération plutôt
 * que de produire un modèle silencieusement faux — c'est la seule façon qu'un
 * contrat qui évolue ne parte pas en divergence sans qu'on le sache.
 */

import { readFile, writeFile, mkdir } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const specPath = join(root, "api", "openapi.yaml");
const outPath = join(root, "apps", "mobile", "lib", "core", "api", "models.dart");

// ─── Lecture du contrat ──────────────────────────────────────────────────────

/**
 * Analyseur YAML minimal, limité à ce que produit ce contrat.
 *
 * Écrire quarante lignes ici plutôt que d'ajouter une dépendance : le fichier
 * est généré par nous, son style est stable, et un analyseur généraliste
 * apporterait surtout des cas que nous n'écrivons jamais.
 */
function parseYaml(text) {
  const root = {};
  const stack = [{ indent: -1, value: root }];

  const lines = text.split("\n");

  for (let i = 0; i < lines.length; i++) {
    const raw = lines[i];
    if (!raw.trim() || raw.trim().startsWith("#")) continue;

    const indent = raw.length - raw.trimStart().length;
    let line = raw.trim();

    while (stack.length > 1 && indent <= stack[stack.length - 1].indent) {
      stack.pop();
    }
    const parent = stack[stack.length - 1].value;

    // Élément de liste.
    if (line.startsWith("- ")) {
      const item = line.slice(2).trim();
      if (!Array.isArray(parent.__list)) parent.__list = [];
      parent.__list.push(parseScalar(item));
      continue;
    }

    const colon = splitKey(line);
    if (!colon) continue;

    const [key, rest] = colon;

    if (rest === "") {
      // Bloc littéral ou objet imbriqué : on regarde la ligne suivante.
      const next = nextMeaningful(lines, i);
      if (next && next.indent > indent && next.text.startsWith("- ")) {
        const list = [];
        parent[key] = list;
        stack.push({ indent, value: { __list: list, __asList: true } });
        // Les listes de scalaires suffisent ici (enum, required).
        for (let j = i + 1; j < lines.length; j++) {
          const l = lines[j];
          if (!l.trim()) continue;
          const ind = l.length - l.trimStart().length;
          if (ind <= indent) break;
          const t = l.trim();
          if (t.startsWith("- ")) list.push(parseScalar(t.slice(2).trim()));
        }
        stack.pop();
        continue;
      }

      const child = {};
      parent[key] = child;
      stack.push({ indent, value: child });
      continue;
    }

    if (rest.startsWith("|") || rest.startsWith(">")) {
      // Bloc de texte : ignoré, seules les descriptions en usent.
      parent[key] = "";
      continue;
    }

    if (rest.startsWith("{")) {
      parent[key] = parseInline(rest);
      continue;
    }
    if (rest.startsWith("[")) {
      parent[key] = rest
        .slice(1, -1)
        .split(",")
        .map((v) => parseScalar(v.trim()))
        .filter((v) => v !== "");
      continue;
    }

    parent[key] = parseScalar(rest);
  }

  return root;
}

function splitKey(line) {
  // Une clé ne contient ni espace ni deux-points ; le premier « : » suivi d'un
  // espace ou de rien la termine.
  const match = /^("?[^":]+"?)\s*:\s*(.*)$/.exec(line);
  if (!match) return null;
  return [match[1].replace(/^"|"$/g, ""), match[2]];
}

function nextMeaningful(lines, from) {
  for (let i = from + 1; i < lines.length; i++) {
    if (!lines[i].trim() || lines[i].trim().startsWith("#")) continue;
    return {
      indent: lines[i].length - lines[i].trimStart().length,
      text: lines[i].trim(),
    };
  }
  return null;
}

/** Analyse un objet en notation compacte : `{ type: string, format: uuid }`. */
function parseInline(text) {
  const out = {};
  const body = text.replace(/^\{|\}$/g, "").trim();
  if (!body) return out;

  let depth = 0;
  let current = "";
  const parts = [];

  for (const char of body) {
    if (char === "{" || char === "[") depth++;
    if (char === "}" || char === "]") depth--;
    if (char === "," && depth === 0) {
      parts.push(current);
      current = "";
      continue;
    }
    current += char;
  }
  if (current.trim()) parts.push(current);

  for (const part of parts) {
    const pair = splitKey(part.trim());
    if (!pair) continue;
    const [key, value] = pair;
    out[key] = value.startsWith("{")
      ? parseInline(value)
      : value.startsWith("[")
        ? value.slice(1, -1).split(",").map((v) => parseScalar(v.trim()))
        : parseScalar(value);
  }
  return out;
}

function parseScalar(value) {
  const trimmed = value.replace(/\s+#.*$/, "").trim();
  if (trimmed === "true") return true;
  if (trimmed === "false") return false;
  if (trimmed === "null" || trimmed === "~") return null;
  if (/^-?\d+$/.test(trimmed)) return Number(trimmed);
  return trimmed.replace(/^["']|["']$/g, "");
}

// ─── Génération ──────────────────────────────────────────────────────────────

/**
 * Schémas générés.
 *
 * Énumérés plutôt que déduits de la totalité du contrat : le client mobile n'a
 * pas besoin des schémas d'administration, et générer ce qu'on n'utilise pas
 * ferait grossir le fichier à chaque ajout côté serveur.
 */
const WANTED = [
  "User",
  "Tokens",
  "Library",
  "Comic",
  "ComicPage",
  "Series",
  "SeriesPage",
  "SearchResults",
  "Manifest",
  "ManifestPage",
  "Progress",
  "Folder",
];

const SCALARS = {
  string: "String",
  integer: "int",
  number: "double",
  boolean: "bool",
};

/**
 * Résout un $ref vers le schéma qu'il désigne.
 *
 * Nécessaire pour distinguer un objet — qui devient une classe — d'une simple
 * énumération de chaînes, qui n'en mérite pas une.
 */
function deref(schema, schemas) {
  if (!schema?.$ref) return schema;
  const name = schema.$ref.split("/").pop();
  const target = schemas[name];
  if (!target) fail(`référence inconnue : ${schema.$ref}`);
  return target;
}

/** Un schéma désigne-t-il un objet, donc une classe Dart ? */
function isObject(schema) {
  return Boolean(schema?.properties) || schema?.type === "object";
}

function dartType(schema, name, schemas) {
  if (!schema) fail(`${name} : schéma absent`);

  if (schema.$ref) {
    const target = deref(schema, schemas);

    /*
      Une énumération de chaînes devient un String, pas une classe ni un enum
      Dart.

      Un enum imposerait de connaître toutes les valeurs à la compilation. Or
      une application installée sur un téléphone vit plus longtemps que le
      serveur qu'elle interroge : le jour où une valeur s'ajoute côté serveur,
      un enum ferait planter la lecture d'une réponse par ailleurs valide.
      Une chaîne traverse.
    */
    if (!isObject(target)) {
      return dartType({ ...target, $ref: undefined }, name, schemas);
    }
    return schema.$ref.split("/").pop();
  }

  if (schema.type === "array") {
    return `List<${dartType(schema.items, `${name}[]`, schemas)}>`;
  }

  if (schema.type === "object" || schema.properties) {
    // Les objets anonymes ne sont pas générés : leur donner un nom serait
    // arbitraire, et le contrat en déclare peu.
    if (schema.additionalProperties) {
      const value = dartType(schema.additionalProperties, `${name}{}`, schemas);
      return `Map<String, ${value}>`;
    }
    fail(`${name} : objet anonyme non pris en charge — le nommer dans le contrat`);
  }

  const dart = SCALARS[schema.type];
  if (!dart) fail(`${name} : type « ${schema.type} » non pris en charge`);
  return dart;
}

function fieldName(key) {
  return key.replace(/[^A-Za-z0-9]/g, "");
}

function decodeExpr(schema, source, type, schemas) {
  if (schema.$ref) {
    // Une énumération résolue en String se décode comme un scalaire.
    if (!isObject(deref(schema, schemas))) {
      return `${source} as ${type}`;
    }
    return `${type}.fromJson(${source} as Map<String, dynamic>)`;
  }
  if (schema.type === "array") {
    const item = schema.items;
    const itemType = dartType(item, "item", schemas);
    const inner = decodeExpr(item, "e", itemType, schemas);
    return `(${source} as List<dynamic>).map((e) => ${inner}).toList()`;
  }
  if (type.startsWith("Map<")) {
    const valueType = type.slice(12, -1);
    return `(${source} as Map<String, dynamic>).map((k, v) => MapEntry(k, v as ${valueType}))`;
  }
  if (type === "double") {
    // JSON rend un entier quand la valeur est ronde : `1` et non `1.0`.
    return `(${source} as num).toDouble()`;
  }
  return `${source} as ${type}`;
}

function generate(spec) {
  const schemas = spec.components?.schemas ?? {};
  const out = [];

  out.push(
    "// GÉNÉRÉ depuis api/openapi.yaml par tools/generate-dart-models.mjs.",
    "// Ne pas éditer : toute modification serait perdue à la régénération.",
    "//",
    "// Le contrat est la source de vérité des trois clients — Go, TypeScript et",
    "// Dart. Écrire ces modèles à la main les ferait diverger dès la première",
    "// évolution du serveur, et la divergence ne se verrait qu'à l'exécution.",
    "",
    "// ignore_for_file: prefer_const_constructors_in_immutables",
    "",
  );

  for (const name of WANTED) {
    const schema = schemas[name];
    if (!schema) fail(`schéma « ${name} » absent du contrat`);

    const properties = schema.properties ?? {};
    const required = new Set(schema.required ?? []);

    const fields = Object.entries(properties).map(([key, prop]) => {
      const type = dartType(prop, `${name}.${key}`, schemas);
      const isRequired = required.has(key);
      return { key, name: fieldName(key), type, prop, isRequired };
    });

    out.push(`/// ${name}, tel que décrit par le contrat.`);
    out.push(`class ${name} {`);

    for (const f of fields) {
      out.push(`  final ${f.type}${f.isRequired ? "" : "?"} ${f.name};`);
    }
    out.push("");

    out.push(`  const ${name}({`);
    for (const f of fields) {
      out.push(`    ${f.isRequired ? "required " : ""}this.${f.name},`);
    }
    out.push("  });");
    out.push("");

    out.push(`  factory ${name}.fromJson(Map<String, dynamic> json) => ${name}(`);
    for (const f of fields) {
      const expr = decodeExpr(f.prop, `json['${f.key}']`, f.type, schemas);
      if (f.isRequired) {
        out.push(`        ${f.name}: ${expr},`);
      } else {
        out.push(`        ${f.name}: json['${f.key}'] == null ? null : ${expr},`);
      }
    }
    out.push("      );");
    out.push("");

    out.push("  Map<String, dynamic> toJson() => {");
    for (const f of fields) {
      out.push(`        '${f.key}': ${encodeExpr(f, schemas)},`);
    }
    out.push("      };");
    out.push("}");
    out.push("");
  }

  return out.join("\n");
}

function encodeExpr(field, schemas) {
  const { name, prop, type } = field;

  if (prop.$ref) {
    if (!isObject(deref(prop, schemas))) return name;
    return `${name}${field.isRequired ? "" : "?"}.toJson()`;
  }
  if (prop.type === "array") {
    const item = prop.items;
    if (item.$ref && isObject(deref(item, schemas))) {
      return `${name}${field.isRequired ? "" : "?"}.map((e) => e.toJson()).toList()`;
    }
    return name;
  }
  if (type.startsWith("Map<")) return name;
  return name;
}

function fail(message) {
  console.error(`✗ génération Dart impossible : ${message}`);
  process.exit(1);
}

// ─── Point d'entrée ──────────────────────────────────────────────────────────

const spec = parseYaml(await readFile(specPath, "utf8"));
const code = generate(spec);

await mkdir(dirname(outPath), { recursive: true });
await writeFile(outPath, code, "utf8");

console.log(`  ✓ modèles Dart → apps/mobile/lib/core/api/models.dart (${WANTED.length} types)`);
