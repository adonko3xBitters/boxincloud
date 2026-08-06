import 'dart:convert';

import 'package:http/http.dart' as http;

import 'models.dart';

/// Erreur remontée par l'API, au format RFC 7807.
class ApiException implements Exception {
  final int status;
  final String title;
  final String detail;
  final Map<String, String> errors;

  const ApiException({
    required this.status,
    this.title = '',
    this.detail = '',
    this.errors = const {},
  });

  /// Message affichable, le plus précis disponible.
  ///
  /// Les erreurs de champ d'abord : « mot de passe trop court » aide, là où
  /// « Validation Failed » n'apprend rien à personne.
  String get message {
    if (errors.isNotEmpty) return errors.values.first;
    if (detail.isNotEmpty) return detail;
    if (title.isNotEmpty) return title;
    return 'Erreur $status';
  }

  bool get isUnauthorized => status == 401;

  @override
  String toString() => 'ApiException($status): $message';
}

/// Le serveur est injoignable — réseau coupé, adresse erronée, service arrêté.
///
/// Distinguée d'une erreur d'API : la première se résout en changeant de réseau,
/// la seconde en changeant la requête. Les confondre ferait afficher « erreur du
/// serveur » à quelqu'un qui est simplement dans un tunnel.
class NetworkException implements Exception {
  final String detail;
  const NetworkException(this.detail);

  @override
  String toString() => 'NetworkException: $detail';
}

/// Jetons d'une session, tels que conservés localement.
class SessionTokens {
  final String accessToken;
  final String refreshToken;

  const SessionTokens({required this.accessToken, required this.refreshToken});
}

/// Signature du rafraîchissement, fournie par la couche d'authentification.
typedef TokenRefresher = Future<SessionTokens?> Function();

/// Client HTTP de boxincloud.
///
/// Une instance par serveur : l'application gère plusieurs serveurs, et un
/// client global obligerait à réécrire son adresse et ses jetons à chaque
/// bascule — occasion parfaite d'envoyer le jeton d'un serveur à un autre.
class ApiClient {
  /// Adresse du serveur, sans barre finale : `https://bd.exemple.fr`.
  final String baseUrl;

  final http.Client _http;

  SessionTokens? _tokens;
  TokenRefresher? _refresher;

  ApiClient({required String baseUrl, http.Client? httpClient})
      : baseUrl = baseUrl.replaceAll(RegExp(r'/+$'), ''),
        _http = httpClient ?? http.Client();

  String get apiRoot => '$baseUrl/api/v1';

  void useTokens(SessionTokens? tokens) => _tokens = tokens;
  void onRefreshNeeded(TokenRefresher refresher) => _refresher = refresher;

  /// URL absolue d'une image servie par le serveur.
  ///
  /// Le jeton voyage en paramètre : une requête d'image ne peut pas porter
  /// d'en-tête `Authorization` quand elle part d'un widget de cache d'images.
  /// Le serveur ne l'accepte que sur ces routes-là, précisément pour cette
  /// raison.
  String imageUrl(String path, {int? width}) {
    final buffer = StringBuffer('$baseUrl$path');
    final params = <String, String>{};
    if (width != null) params['width'] = '$width';
    if (_tokens != null) params['token'] = _tokens!.accessToken;

    if (params.isNotEmpty) {
      buffer.write('?');
      buffer.write(params.entries
          .map((e) => '${e.key}=${Uri.encodeQueryComponent(e.value)}')
          .join('&'));
    }
    return buffer.toString();
  }

  // ─── Requêtes ──────────────────────────────────────────────────────────────

  Future<T> get<T>(String path, T Function(dynamic) decode,
          {Map<String, String>? query}) =>
      _send('GET', path, decode, query: query);

  Future<T> post<T>(String path, T Function(dynamic) decode,
          {Object? body, bool anonymous = false}) =>
      _send('POST', path, decode, body: body, anonymous: anonymous);

  Future<T> put<T>(String path, T Function(dynamic) decode, {Object? body}) =>
      _send('PUT', path, decode, body: body);

  Future<void> delete(String path) => _send<void>('DELETE', path, (_) {});

  /*
    Récupère des octets bruts — une image, pas du JSON.

    Ne passe pas par `_send`, qui décode systématiquement le corps : une page de
    bande dessinée n'est pas de l'UTF-8, et la décoder la détruirait. Le
    rafraîchissement de jeton est en revanche le même, à un essai près, pour la
    même raison qu'ailleurs.
  */
  Future<List<int>> bytes(String path, {Map<String, String>? query}) async {
    var response = await _perform('GET', path, null, query, false);

    if (response.statusCode == 401 && _refresher != null) {
      final fresh = await _refresher!();
      if (fresh != null) {
        _tokens = fresh;
        response = await _perform('GET', path, null, query, false);
      }
    }

    if (response.statusCode >= 200 && response.statusCode < 300) {
      return response.bodyBytes;
    }

    throw _problemFrom(response);
  }

  /*
    Exécute la requête, en rafraîchissant le jeton une fois si besoin.

    Le rejeu est limité à un essai : si le jeton fraîchement obtenu est refusé
    lui aussi, insister ne ferait que boucler — et sur un serveur qui a révoqué
    la session, marteler la même requête ressemble à une attaque.
  */
  Future<T> _send<T>(
    String method,
    String path,
    T Function(dynamic) decode, {
    Object? body,
    Map<String, String>? query,
    bool anonymous = false,
  }) async {
    var response = await _perform(method, path, body, query, anonymous);

    if (response.statusCode == 401 && !anonymous && _refresher != null) {
      final fresh = await _refresher!();
      if (fresh != null) {
        _tokens = fresh;
        response = await _perform(method, path, body, query, anonymous);
      }
    }

    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (response.body.isEmpty) return decode(null);
      return decode(jsonDecode(utf8.decode(response.bodyBytes)));
    }

    throw _problemFrom(response);
  }

  Future<http.Response> _perform(
    String method,
    String path,
    Object? body,
    Map<String, String>? query,
    bool anonymous,
  ) async {
    final uri = Uri.parse('$apiRoot$path').replace(
      queryParameters: query?.isEmpty ?? true ? null : query,
    );

    final headers = <String, String>{'Accept': 'application/json'};
    if (body != null) headers['Content-Type'] = 'application/json';
    if (!anonymous && _tokens != null) {
      headers['Authorization'] = 'Bearer ${_tokens!.accessToken}';
    }

    final encoded = body == null ? null : jsonEncode(body);

    try {
      switch (method) {
        case 'GET':
          return await _http.get(uri, headers: headers);
        case 'POST':
          return await _http.post(uri, headers: headers, body: encoded);
        case 'PUT':
          return await _http.put(uri, headers: headers, body: encoded);
        case 'DELETE':
          return await _http.delete(uri, headers: headers);
        default:
          throw ArgumentError('méthode non gérée : $method');
      }
    } on http.ClientException catch (e) {
      throw NetworkException(e.message);
    } on FormatException catch (e) {
      throw NetworkException('adresse de serveur invalide : ${e.message}');
    } catch (e) {
      throw NetworkException('$e');
    }
  }

  ApiException _problemFrom(http.Response response) {
    try {
      final decoded = jsonDecode(utf8.decode(response.bodyBytes));
      if (decoded is Map<String, dynamic>) {
        final rawErrors = decoded['errors'];
        return ApiException(
          status: response.statusCode,
          title: (decoded['title'] as String?) ?? '',
          detail: (decoded['detail'] as String?) ?? '',
          errors: rawErrors is Map<String, dynamic>
              ? rawErrors.map((k, v) => MapEntry(k, '$v'))
              : const {},
        );
      }
    } catch (_) {
      // Le corps n'est pas un problème JSON : le code suffit.
    }
    return ApiException(status: response.statusCode);
  }

  void close() => _http.close();
}

// ─── Opérations ──────────────────────────────────────────────────────────────

/// Appels nommés, un par opération du contrat.
///
/// Couche mince : elle donne un nom et un type à chaque route, sans logique.
/// C'est le pendant Dart de `apps/web/src/lib/api/endpoints.ts`.
extension BoxincloudApi on ApiClient {
  Future<Map<String, dynamic>> version() =>
      get('/version', (json) => json as Map<String, dynamic>);

  Future<bool> needsSetup() => post(
        '/auth/status',
        (json) => (json as Map<String, dynamic>)['needsSetup'] as bool,
        anonymous: true,
      );

  Future<Tokens> login({
    required String username,
    required String password,
    required String deviceName,
  }) =>
      post(
        '/auth/login',
        (json) => Tokens.fromJson(json as Map<String, dynamic>),
        anonymous: true,
        body: {
          'username': username,
          'password': password,
          'deviceName': deviceName,
          'platform': 'android',
        },
      );

  Future<Tokens> refresh(String refreshToken) => post(
        '/auth/refresh',
        (json) => Tokens.fromJson(json as Map<String, dynamic>),
        anonymous: true,
        body: {'refreshToken': refreshToken},
      );

  Future<void> logout(String refreshToken) => post(
        '/auth/logout',
        (_) {},
        anonymous: true,
        body: {'refreshToken': refreshToken},
      );

  Future<User> me() => get('/me', (json) => User.fromJson(json as Map<String, dynamic>));

  Future<List<Library>> libraries() => get(
        '/libraries',
        (json) => ((json as Map<String, dynamic>)['libraries'] as List<dynamic>)
            .map((e) => Library.fromJson(e as Map<String, dynamic>))
            .toList(),
      );

  Future<ComicPage> comics({
    String? libraryId,
    String? seriesId,
    String? folder,
    String? readStatus,
    String? sort,
    String? cursor,
    int limit = 50,
  }) =>
      get(
        '/comics',
        (json) => ComicPage.fromJson(json as Map<String, dynamic>),
        query: {
          'libraryId': ?libraryId,
          'seriesId': ?seriesId,
          'folder': ?folder,
          if (readStatus != null && readStatus.isNotEmpty) 'readStatus': readStatus,
          'sort': ?sort,
          'cursor': ?cursor,
          'limit': '$limit',
        },
      );

  Future<Comic> comic(String id) =>
      get('/comics/$id', (json) => Comic.fromJson(json as Map<String, dynamic>));

  /// Identifiants des albums mis en favori par le compte.
  ///
  /// L'endpoint retourne aussi les notes, dont l'application mobile ne fait
  /// rien pour l'instant : les décoder ici obligerait à un modèle généré pour
  /// un champ ignoré.
  Future<List<String>> favorites() => get(
        '/me/marks',
        (json) => ((json as Map<String, dynamic>)['favorites'] as List<dynamic>)
            .cast<String>(),
      );

  Future<bool> setFavorite(String comicId, bool favorite) => put(
        '/comics/$comicId/favorite',
        (json) => (json as Map<String, dynamic>)['favorite'] as bool,
        body: {'favorite': favorite},
      );

  Future<List<Folder>> folders({String? libraryId}) => get(
        '/folders',
        (json) => ((json as Map<String, dynamic>)['folders'] as List<dynamic>)
            .map((e) => Folder.fromJson(e as Map<String, dynamic>))
            .toList(),
        query: {'libraryId': ?libraryId},
      );

  Future<SeriesPage> series({String? libraryId, int limit = 100}) => get(
        '/series',
        (json) => SeriesPage.fromJson(json as Map<String, dynamic>),
        query: {
          'libraryId': ?libraryId,
          'limit': '$limit',
        },
      );

  /// Recherche côté serveur : plein texte, trigrammes, insensible aux accents.
  ///
  /// Une requête trop courte est refusée ici plutôt qu'au retour : le serveur
  /// répondrait des listes vides, et faire l'aller-retour pour l'apprendre
  /// coûte une latence à chaque frappe.
  Future<SearchResults> search(String query, {String? libraryId, int limit = 30}) {
    if (query.trim().length < 2) {
      return Future.value(const SearchResults(comics: [], series: []));
    }

    return get(
      '/search',
      (json) => SearchResults.fromJson(json as Map<String, dynamic>),
      query: {
        'q': query.trim(),
        'libraryId': ?libraryId,
        'limit': '$limit',
      },
    );
  }

  /// Octets d'une page, à la largeur demandée.
  ///
  /// Largeur nulle : l'image d'origine, telle qu'elle est dans l'archive.
  Future<List<int>> pageBytes(String comicId, int index, {int width = 0}) => bytes(
        '/comics/$comicId/pages/$index',
        query: width > 0 ? {'width': '$width'} : null,
      );

  Future<Manifest> manifest(String comicId) => get(
        '/comics/$comicId/manifest',
        (json) => Manifest.fromJson(json as Map<String, dynamic>),
      );

  Future<Progress> progress(String comicId) => get(
        '/comics/$comicId/progress',
        (json) => Progress.fromJson(json as Map<String, dynamic>),
      );

  Future<Progress> saveProgress(
    String comicId, {
    required int page,
    required int pageCount,
    String? status,
  }) =>
      put(
        '/comics/$comicId/progress',
        (json) => Progress.fromJson(json as Map<String, dynamic>),
        body: {
          'page': page,
          'pageCount': pageCount,
          'status': ?status,
        },
      );

  /// Pousse un lot de progressions accumulées hors ligne.
  Future<void> pushSync(List<Map<String, dynamic>> updates) =>
      post('/sync', (_) {}, body: {'updates': updates});

  /// Récupère les progressions modifiées ailleurs depuis `since`.
  ///
  /// L'autre moitié de la synchronisation. Sans elle, une lecture reprise sur
  /// le web n'atteint jamais le téléphone : il ne sait que raconter, pas
  /// écouter.
  Future<SyncChanges> pullSync({String? since, int? limit}) => get(
        '/sync',
        (json) => SyncChanges.fromJson(json as Map<String, dynamic>),
        query: {
          if (since != null && since.isNotEmpty) 'since': since,
          if (limit != null) 'limit': '$limit',
        },
      );
}
