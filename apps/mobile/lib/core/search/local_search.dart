/// Recherche dans le cache local.
///
/// Le serveur cherche mieux : trigrammes, plein texte, tolérance aux fautes de
/// frappe. Mais il faut le réseau, et une application de lecture sert surtout
/// dans un train. Une recherche qui répond « hors ligne » là où l'utilisateur a
/// justement tout son cache sous la main serait absurde.
///
/// Cette recherche-ci est donc volontairement plus modeste : pliage des accents
/// et correspondance par sous-chaîne, sur les données déjà en cache. Elle ne
/// pardonne pas « asterics », mais elle trouve « Astérix » depuis « asterix » —
/// ce qui couvre le geste réel, taper sans accents.
library;

/// Replie une chaîne pour la comparaison : minuscules, sans accents, sans
/// ponctuation ni espaces multiples.
///
/// La table est explicite plutôt que dérivée d'une normalisation Unicode :
/// `dart:core` n'expose pas NFD, et importer `intl` pour une trentaine de
/// caractères latins pèserait plus que la table.
String fold(String input) {
  final buffer = StringBuffer();
  var lastWasSpace = true;

  for (final rune in input.toLowerCase().runes) {
    final char = String.fromCharCode(rune);
    final folded = _accents[char] ?? char;

    if (_isAlphanumeric(folded)) {
      buffer.write(folded);
      lastWasSpace = false;
    } else if (!lastWasSpace) {
      buffer.write(' ');
      lastWasSpace = true;
    }
  }

  return buffer.toString().trim();
}

bool _isAlphanumeric(String char) {
  final code = char.codeUnitAt(0);
  return (code >= 0x30 && code <= 0x39) || (code >= 0x61 && code <= 0x7a);
}

const _accents = <String, String>{
  'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
  'ç': 'c',
  'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
  'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
  'ñ': 'n',
  'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o',
  'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
  'ý': 'y', 'ÿ': 'y',
  'œ': 'oe', 'æ': 'ae', 'ß': 'ss',
};

/// Score d'un candidat face à une requête déjà repliée.
///
/// `null` quand il ne correspond pas. Sinon, plus le score est bas, meilleur
/// est le résultat — l'ordre naturel d'un tri.
///
/// Trois rangs seulement, parce qu'un classement plus fin serait du bruit sur
/// une liste qu'on lit d'un coup d'œil : le titre exact, puis ce qui commence
/// par la requête, puis ce qui la contient. À rang égal, le plus court gagne :
/// « Astérix » avant « Astérix et les Normands » quand on a tapé « asterix ».
int? score(String candidate, String foldedQuery) {
  if (foldedQuery.isEmpty) return null;

  final folded = fold(candidate);
  if (folded == foldedQuery) return 0;
  if (folded.startsWith(foldedQuery)) return 1000 + folded.length;

  // Un début de mot vaut mieux qu'un fragment au milieu : « rix » ne devrait
  // pas remonter « Astérix » au même rang que « Rixe ».
  if (folded.contains(' $foldedQuery')) return 2000 + folded.length;
  if (folded.contains(foldedQuery)) return 3000 + folded.length;

  return null;
}

/// Trie et filtre une liste selon le meilleur de ses champs consultables.
///
/// Le titre et la série comptent tous les deux : on cherche « astérix » en
/// pensant à la série, et le tome s'appelle « Le Combat des chefs ».
List<T> rank<T>(
  Iterable<T> items,
  String query,
  List<String> Function(T) fieldsOf, {
  int limit = 50,
}) {
  final folded = fold(query);
  if (folded.length < 2) return const [];

  final scored = <({T item, int score})>[];

  for (final item in items) {
    int? best;
    for (final field in fieldsOf(item)) {
      final value = score(field, folded);
      if (value != null && (best == null || value < best)) best = value;
    }
    if (best != null) scored.add((item: item, score: best));
  }

  scored.sort((a, b) => a.score.compareTo(b.score));
  return scored.take(limit).map((e) => e.item).toList();
}
