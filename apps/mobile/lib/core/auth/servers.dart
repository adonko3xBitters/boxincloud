import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Un serveur boxincloud enregistré sur cet appareil.
///
/// L'application en gère plusieurs : un serveur familial et celui d'un ami, ou
/// un serveur de test à côté du sien. Chacun porte ses propres jetons — les
/// mélanger enverrait le jeton de l'un à l'autre.
class ServerAccount {
  final String id;
  final String baseUrl;
  final String label;
  final String username;
  final String displayName;

  const ServerAccount({
    required this.id,
    required this.baseUrl,
    required this.label,
    required this.username,
    this.displayName = '',
  });

  String get title => displayName.isNotEmpty ? displayName : username;

  Map<String, dynamic> toJson() => {
        'id': id,
        'baseUrl': baseUrl,
        'label': label,
        'username': username,
        'displayName': displayName,
      };

  factory ServerAccount.fromJson(Map<String, dynamic> json) => ServerAccount(
        id: json['id'] as String,
        baseUrl: json['baseUrl'] as String,
        label: (json['label'] as String?) ?? '',
        username: (json['username'] as String?) ?? '',
        displayName: (json['displayName'] as String?) ?? '',
      );
}

/*
Stockage des serveurs et de leurs jetons.

Tout passe par le trousseau du système — Keychain sur iOS, Keystore sur Android.
Pas de SharedPreferences : son contenu est un simple fichier, embarqué dans les
sauvegardes de l'appareil et lisible sur un téléphone rooté. Un jeton de
rafraîchissement y vaut un accès permanent à la bibliothèque.

La LISTE des serveurs y va aussi, alors qu'elle n'est pas secrète. La séparer
imposerait de tenir deux stockages cohérents entre eux, pour économiser quelques
octets dans un endroit sûr.
*/
class ServerStore {
  static const _serversKey = 'boxincloud.servers';
  static const _activeKey = 'boxincloud.active_server';

  final FlutterSecureStorage _storage;

  ServerStore({FlutterSecureStorage? storage})
      : _storage = storage ??
            const FlutterSecureStorage(
              aOptions: AndroidOptions(encryptedSharedPreferences: true),
            );

  Future<List<ServerAccount>> servers() async {
    final raw = await _storage.read(key: _serversKey);
    if (raw == null || raw.isEmpty) return const [];

    try {
      final list = jsonDecode(raw) as List<dynamic>;
      return list
          .map((e) => ServerAccount.fromJson(e as Map<String, dynamic>))
          .toList();
    } catch (_) {
      // Un enregistrement illisible ne doit pas bloquer l'application au
      // démarrage : on repart d'une liste vide plutôt que de refuser d'ouvrir.
      return const [];
    }
  }

  Future<void> save(ServerAccount server) async {
    final list = (await servers()).where((s) => s.id != server.id).toList()
      ..add(server);
    await _storage.write(key: _serversKey, value: jsonEncode(list));
  }

  /// Retire un serveur et ses jetons.
  ///
  /// Les deux ensemble, toujours : un jeton orphelin resterait valable jusqu'à
  /// son expiration sans qu'aucun écran ne permette de s'en défaire.
  Future<void> forget(String id) async {
    final list = (await servers()).where((s) => s.id != id).toList();
    await _storage.write(key: _serversKey, value: jsonEncode(list));
    await _storage.delete(key: _tokensKey(id));

    if (await activeId() == id) {
      await _storage.delete(key: _activeKey);
    }
  }

  Future<String?> activeId() => _storage.read(key: _activeKey);

  Future<void> setActive(String id) => _storage.write(key: _activeKey, value: id);

  // ─── Jetons ────────────────────────────────────────────────────────────────

  String _tokensKey(String serverId) => 'boxincloud.tokens.$serverId';

  Future<({String access, String refresh})?> tokens(String serverId) async {
    final raw = await _storage.read(key: _tokensKey(serverId));
    if (raw == null || raw.isEmpty) return null;

    try {
      final json = jsonDecode(raw) as Map<String, dynamic>;
      return (
        access: json['access'] as String,
        refresh: json['refresh'] as String,
      );
    } catch (_) {
      return null;
    }
  }

  Future<void> saveTokens(String serverId, String access, String refresh) =>
      _storage.write(
        key: _tokensKey(serverId),
        value: jsonEncode({'access': access, 'refresh': refresh}),
      );

  Future<void> clearTokens(String serverId) =>
      _storage.delete(key: _tokensKey(serverId));
}

/// Normalise une adresse saisie à la main.
///
/// Quelqu'un tape « bd.exemple.fr », « http://192.168.1.10:8070/ » ou colle une
/// URL avec un chemin. Les trois doivent marcher : refuser sur la forme
/// n'apprend rien à personne et bloque au premier écran.
String normalizeServerUrl(String input) {
  var url = input.trim();
  if (url.isEmpty) return '';

  if (!url.startsWith('http://') && !url.startsWith('https://')) {
    // https par défaut : sur un réseau local en http, l'utilisateur le précise.
    url = 'https://$url';
  }

  final parsed = Uri.tryParse(url);
  if (parsed == null || parsed.host.isEmpty) return '';

  // Le chemin est retiré : l'API vit toujours sous /api/v1, et coller l'adresse
  // d'un écran de l'application ne doit pas produire un serveur inatteignable.
  return Uri(
    scheme: parsed.scheme,
    host: parsed.host,
    port: parsed.hasPort ? parsed.port : null,
  ).toString();
}
