// GÉNÉRÉ depuis api/openapi.yaml par tools/generate-dart-models.mjs.
// Ne pas éditer : toute modification serait perdue à la régénération.
//
// Le contrat est la source de vérité des trois clients — Go, TypeScript et
// Dart. Écrire ces modèles à la main les ferait diverger dès la première
// évolution du serveur, et la divergence ne se verrait qu'à l'exécution.

// ignore_for_file: prefer_const_constructors_in_immutables

/// User, tel que décrit par le contrat.
class User {
  final String id;
  final String username;
  final String? email;
  final String? displayName;
  final String role;
  final bool restricted;

  const User({
    required this.id,
    required this.username,
    this.email,
    this.displayName,
    required this.role,
    required this.restricted,
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
        id: json['id'] as String,
        username: json['username'] as String,
        email: json['email'] == null ? null : json['email'] as String,
        displayName: json['displayName'] == null ? null : json['displayName'] as String,
        role: json['role'] as String,
        restricted: json['restricted'] as bool,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'username': username,
        'email': email,
        'displayName': displayName,
        'role': role,
        'restricted': restricted,
      };
}

/// Tokens, tel que décrit par le contrat.
class Tokens {
  final String accessToken;
  final String refreshToken;
  final String expiresAt;
  final String deviceId;
  final User user;

  const Tokens({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresAt,
    required this.deviceId,
    required this.user,
  });

  factory Tokens.fromJson(Map<String, dynamic> json) => Tokens(
        accessToken: json['accessToken'] as String,
        refreshToken: json['refreshToken'] as String,
        expiresAt: json['expiresAt'] as String,
        deviceId: json['deviceId'] as String,
        user: User.fromJson(json['user'] as Map<String, dynamic>),
      );

  Map<String, dynamic> toJson() => {
        'accessToken': accessToken,
        'refreshToken': refreshToken,
        'expiresAt': expiresAt,
        'deviceId': deviceId,
        'user': user.toJson(),
      };
}

/// Library, tel que décrit par le contrat.
class Library {
  final String id;
  final String name;
  final String kind;
  final int comicCount;

  const Library({
    required this.id,
    required this.name,
    required this.kind,
    required this.comicCount,
  });

  factory Library.fromJson(Map<String, dynamic> json) => Library(
        id: json['id'] as String,
        name: json['name'] as String,
        kind: json['kind'] as String,
        comicCount: json['comicCount'] as int,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'name': name,
        'kind': kind,
        'comicCount': comicCount,
      };
}

/// Comic, tel que décrit par le contrat.
class Comic {
  final String id;
  final String libraryId;
  final String? seriesId;
  final String? seriesName;
  final String title;
  final String? number;
  final int? volume;
  final String? summary;
  final String format;
  final int pageCount;
  final String state;
  final int? ageRating;
  final String? language;
  final int fileSize;
  final String? releasedAt;
  final String createdAt;
  final String coverPath;
  final String? coverPlaceholder;
  final String fileName;
  final String folderPath;

  const Comic({
    required this.id,
    required this.libraryId,
    this.seriesId,
    this.seriesName,
    required this.title,
    this.number,
    this.volume,
    this.summary,
    required this.format,
    required this.pageCount,
    required this.state,
    this.ageRating,
    this.language,
    required this.fileSize,
    this.releasedAt,
    required this.createdAt,
    required this.coverPath,
    this.coverPlaceholder,
    required this.fileName,
    required this.folderPath,
  });

  factory Comic.fromJson(Map<String, dynamic> json) => Comic(
        id: json['id'] as String,
        libraryId: json['libraryId'] as String,
        seriesId: json['seriesId'] == null ? null : json['seriesId'] as String,
        seriesName: json['seriesName'] == null ? null : json['seriesName'] as String,
        title: json['title'] as String,
        number: json['number'] == null ? null : json['number'] as String,
        volume: json['volume'] == null ? null : json['volume'] as int,
        summary: json['summary'] == null ? null : json['summary'] as String,
        format: json['format'] as String,
        pageCount: json['pageCount'] as int,
        state: json['state'] as String,
        ageRating: json['ageRating'] == null ? null : json['ageRating'] as int,
        language: json['language'] == null ? null : json['language'] as String,
        fileSize: json['fileSize'] as int,
        releasedAt: json['releasedAt'] == null ? null : json['releasedAt'] as String,
        createdAt: json['createdAt'] as String,
        coverPath: json['coverPath'] as String,
        coverPlaceholder: json['coverPlaceholder'] == null ? null : json['coverPlaceholder'] as String,
        fileName: json['fileName'] as String,
        folderPath: json['folderPath'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'libraryId': libraryId,
        'seriesId': seriesId,
        'seriesName': seriesName,
        'title': title,
        'number': number,
        'volume': volume,
        'summary': summary,
        'format': format,
        'pageCount': pageCount,
        'state': state,
        'ageRating': ageRating,
        'language': language,
        'fileSize': fileSize,
        'releasedAt': releasedAt,
        'createdAt': createdAt,
        'coverPath': coverPath,
        'coverPlaceholder': coverPlaceholder,
        'fileName': fileName,
        'folderPath': folderPath,
      };
}

/// ComicPage, tel que décrit par le contrat.
class ComicPage {
  final List<Comic> items;
  final String? nextCursor;

  const ComicPage({
    required this.items,
    this.nextCursor,
  });

  factory ComicPage.fromJson(Map<String, dynamic> json) => ComicPage(
        items: (json['items'] as List<dynamic>).map((e) => Comic.fromJson(e as Map<String, dynamic>)).toList(),
        nextCursor: json['nextCursor'] == null ? null : json['nextCursor'] as String,
      );

  Map<String, dynamic> toJson() => {
        'items': items.map((e) => e.toJson()).toList(),
        'nextCursor': nextCursor,
      };
}

/// Series, tel que décrit par le contrat.
class Series {
  final String id;
  final String libraryId;
  final String name;
  final String? description;
  final String? publisher;
  final int comicCount;
  final String? coverComicId;
  final String? coverPath;

  const Series({
    required this.id,
    required this.libraryId,
    required this.name,
    this.description,
    this.publisher,
    required this.comicCount,
    this.coverComicId,
    this.coverPath,
  });

  factory Series.fromJson(Map<String, dynamic> json) => Series(
        id: json['id'] as String,
        libraryId: json['libraryId'] as String,
        name: json['name'] as String,
        description: json['description'] == null ? null : json['description'] as String,
        publisher: json['publisher'] == null ? null : json['publisher'] as String,
        comicCount: json['comicCount'] as int,
        coverComicId: json['coverComicId'] == null ? null : json['coverComicId'] as String,
        coverPath: json['coverPath'] == null ? null : json['coverPath'] as String,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'libraryId': libraryId,
        'name': name,
        'description': description,
        'publisher': publisher,
        'comicCount': comicCount,
        'coverComicId': coverComicId,
        'coverPath': coverPath,
      };
}

/// SeriesPage, tel que décrit par le contrat.
class SeriesPage {
  final List<Series> items;
  final String? nextCursor;

  const SeriesPage({
    required this.items,
    this.nextCursor,
  });

  factory SeriesPage.fromJson(Map<String, dynamic> json) => SeriesPage(
        items: (json['items'] as List<dynamic>).map((e) => Series.fromJson(e as Map<String, dynamic>)).toList(),
        nextCursor: json['nextCursor'] == null ? null : json['nextCursor'] as String,
      );

  Map<String, dynamic> toJson() => {
        'items': items.map((e) => e.toJson()).toList(),
        'nextCursor': nextCursor,
      };
}

/// SearchResults, tel que décrit par le contrat.
class SearchResults {
  final List<Comic> comics;
  final List<Series> series;

  const SearchResults({
    required this.comics,
    required this.series,
  });

  factory SearchResults.fromJson(Map<String, dynamic> json) => SearchResults(
        comics: (json['comics'] as List<dynamic>).map((e) => Comic.fromJson(e as Map<String, dynamic>)).toList(),
        series: (json['series'] as List<dynamic>).map((e) => Series.fromJson(e as Map<String, dynamic>)).toList(),
      );

  Map<String, dynamic> toJson() => {
        'comics': comics.map((e) => e.toJson()).toList(),
        'series': series.map((e) => e.toJson()).toList(),
      };
}

/// Manifest, tel que décrit par le contrat.
class Manifest {
  final String comicId;
  final int pageCount;
  final List<ManifestPage> pages;

  const Manifest({
    required this.comicId,
    required this.pageCount,
    required this.pages,
  });

  factory Manifest.fromJson(Map<String, dynamic> json) => Manifest(
        comicId: json['comicId'] as String,
        pageCount: json['pageCount'] as int,
        pages: (json['pages'] as List<dynamic>).map((e) => ManifestPage.fromJson(e as Map<String, dynamic>)).toList(),
      );

  Map<String, dynamic> toJson() => {
        'comicId': comicId,
        'pageCount': pageCount,
        'pages': pages.map((e) => e.toJson()).toList(),
      };
}

/// ManifestPage, tel que décrit par le contrat.
class ManifestPage {
  final int index;
  final int? width;
  final int? height;
  final bool isDouble;

  const ManifestPage({
    required this.index,
    this.width,
    this.height,
    required this.isDouble,
  });

  factory ManifestPage.fromJson(Map<String, dynamic> json) => ManifestPage(
        index: json['index'] as int,
        width: json['width'] == null ? null : json['width'] as int,
        height: json['height'] == null ? null : json['height'] as int,
        isDouble: json['isDouble'] as bool,
      );

  Map<String, dynamic> toJson() => {
        'index': index,
        'width': width,
        'height': height,
        'isDouble': isDouble,
      };
}

/// Progress, tel que décrit par le contrat.
/// Une page de changements rendue par `GET /sync`.
class SyncChanges {
  final List<Progress> changes;

  /// À renvoyer au prochain appel. C'est lui qui rend la synchronisation
  /// incrémentale : sans le conserver, chaque démarrage retéléchargerait
  /// l'historique entier.
  final String cursor;

  /// Une page est disponible IMMÉDIATEMENT. Un client qui rattrape une longue
  /// absence doit boucler sans attendre le prochain réveil.
  final bool hasMore;

  const SyncChanges({
    required this.changes,
    required this.cursor,
    required this.hasMore,
  });

  factory SyncChanges.fromJson(Map<String, dynamic> json) => SyncChanges(
        changes: ((json['changes'] as List<dynamic>?) ?? const [])
            .map((e) => Progress.fromJson(e as Map<String, dynamic>))
            .toList(),
        cursor: (json['cursor'] as String?) ?? '',
        hasMore: (json['hasMore'] as bool?) ?? false,
      );
}

class Progress {
  final String comicId;
  final int page;
  final int pageCount;
  final double percent;
  final String status;
  final int readCount;
  final int version;
  final String? deviceId;
  final String? startedAt;
  final String? finishedAt;
  final String updatedAt;

  const Progress({
    required this.comicId,
    required this.page,
    required this.pageCount,
    required this.percent,
    required this.status,
    required this.readCount,
    required this.version,
    this.deviceId,
    this.startedAt,
    this.finishedAt,
    required this.updatedAt,
  });

  factory Progress.fromJson(Map<String, dynamic> json) => Progress(
        comicId: json['comicId'] as String,
        page: json['page'] as int,
        pageCount: json['pageCount'] as int,
        percent: (json['percent'] as num).toDouble(),
        status: json['status'] as String,
        readCount: json['readCount'] as int,
        version: json['version'] as int,
        deviceId: json['deviceId'] == null ? null : json['deviceId'] as String,
        startedAt: json['startedAt'] == null ? null : json['startedAt'] as String,
        finishedAt: json['finishedAt'] == null ? null : json['finishedAt'] as String,
        updatedAt: json['updatedAt'] as String,
      );

  Map<String, dynamic> toJson() => {
        'comicId': comicId,
        'page': page,
        'pageCount': pageCount,
        'percent': percent,
        'status': status,
        'readCount': readCount,
        'version': version,
        'deviceId': deviceId,
        'startedAt': startedAt,
        'finishedAt': finishedAt,
        'updatedAt': updatedAt,
      };
}

/// Folder, tel que décrit par le contrat.
class Folder {
  final String id;
  final String libraryId;
  final String path;
  final String name;
  final int depth;
  final int comicCount;
  final bool explicit;
  final bool readOnly;
  final bool hasCode;
  final bool unlocked;

  const Folder({
    required this.id,
    required this.libraryId,
    required this.path,
    required this.name,
    required this.depth,
    required this.comicCount,
    required this.explicit,
    required this.readOnly,
    required this.hasCode,
    required this.unlocked,
  });

  factory Folder.fromJson(Map<String, dynamic> json) => Folder(
        id: json['id'] as String,
        libraryId: json['libraryId'] as String,
        path: json['path'] as String,
        name: json['name'] as String,
        depth: json['depth'] as int,
        comicCount: json['comicCount'] as int,
        explicit: json['explicit'] as bool,
        readOnly: json['readOnly'] as bool,
        hasCode: json['hasCode'] as bool,
        unlocked: json['unlocked'] as bool,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'libraryId': libraryId,
        'path': path,
        'name': name,
        'depth': depth,
        'comicCount': comicCount,
        'explicit': explicit,
        'readOnly': readOnly,
        'hasCode': hasCode,
        'unlocked': unlocked,
      };
}
