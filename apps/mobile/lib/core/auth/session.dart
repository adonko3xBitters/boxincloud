import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../api/client.dart';
import '../api/models.dart';
import 'servers.dart';

/// État d'authentification de l'application.
sealed class SessionState {
  const SessionState();
}

/// Au démarrage : on lit le trousseau avant de savoir quoi afficher.
class SessionLoading extends SessionState {
  const SessionLoading();
}

/// Aucun serveur connecté — premier lancement, ou déconnexion.
class SessionSignedOut extends SessionState {
  final List<ServerAccount> servers;
  const SessionSignedOut({this.servers = const []});
}

/// Connecté à un serveur.
class SessionActive extends SessionState {
  final ServerAccount server;
  final User user;
  final ApiClient client;
  final List<ServerAccount> servers;

  const SessionActive({
    required this.server,
    required this.user,
    required this.client,
    required this.servers,
  });
}

final serverStoreProvider = Provider((ref) => ServerStore());

final sessionProvider =
    NotifierProvider<SessionController, SessionState>(SessionController.new);

/*
Contrôleur de session.

Il porte le rafraîchissement de jeton, et c'est le point délicat : le serveur
fait tourner le jeton de rafraîchissement à chaque usage et interprète la
réutilisation d'un jeton déjà consommé comme un vol — il révoque alors toute la
session. Deux requêtes qui échouent en même temps ne doivent donc pas
rafraîchir chacune de leur côté.
*/
class SessionController extends Notifier<SessionState> {
  Future<SessionTokens?>? _refreshInFlight;

  @override
  SessionState build() {
    Future.microtask(restore);
    return const SessionLoading();
  }

  ServerStore get _store => ref.read(serverStoreProvider);

  /// Rétablit la session enregistrée au démarrage.
  Future<void> restore() async {
    final servers = await _store.servers();
    final activeId = await _store.activeId();

    if (activeId == null || servers.every((s) => s.id != activeId)) {
      state = SessionSignedOut(servers: servers);
      return;
    }

    final server = servers.firstWhere((s) => s.id == activeId);
    final tokens = await _store.tokens(server.id);
    if (tokens == null) {
      state = SessionSignedOut(servers: servers);
      return;
    }

    final client = _clientFor(server, tokens.access, tokens.refresh);

    try {
      final user = await client.me();
      state = SessionActive(
        server: server,
        user: user,
        client: client,
        servers: servers,
      );
    } on NetworkException {
      /*
        Serveur injoignable au démarrage : on reste connecté.

        Déconnecter parce que le réseau manque effacerait la session de quelqu'un
        qui ouvre l'application dans le métro — alors que le cache local a
        justement de quoi lui montrer sa bibliothèque.
      */
      state = SessionActive(
        server: server,
        user: User(
          id: '',
          username: server.username,
          displayName: server.displayName,
          role: 'user',
          restricted: false,
        ),
        client: client,
        servers: servers,
      );
    } on ApiException {
      // Jeton refusé et non rafraîchissable : la session est bel et bien finie.
      await _store.clearTokens(server.id);
      state = SessionSignedOut(servers: servers);
    }
  }

  /// Connecte un serveur, en l'ajoutant s'il est nouveau.
  Future<void> signIn({
    required String baseUrl,
    required String username,
    required String password,
    String label = '',
  }) async {
    final url = normalizeServerUrl(baseUrl);
    if (url.isEmpty) {
      throw const ApiException(status: 0, detail: 'Adresse de serveur invalide.');
    }

    final client = ApiClient(baseUrl: url);
    final tokens = await client.login(
      username: username,
      password: password,
      deviceName: await _deviceName(),
    );

    final server = ServerAccount(
      id: '$url#$username',
      baseUrl: url,
      label: label.isNotEmpty ? label : Uri.parse(url).host,
      username: tokens.user.username,
      displayName: tokens.user.displayName ?? '',
    );

    await _store.save(server);
    await _store.saveTokens(server.id, tokens.accessToken, tokens.refreshToken);
    await _store.setActive(server.id);

    client.useTokens(SessionTokens(
      accessToken: tokens.accessToken,
      refreshToken: tokens.refreshToken,
    ));
    client.onRefreshNeeded(() => _refresh(server.id, client));

    state = SessionActive(
      server: server,
      user: tokens.user,
      client: client,
      servers: await _store.servers(),
    );
  }

  /// Bascule vers un autre serveur déjà enregistré.
  Future<void> switchTo(String serverId) async {
    await _store.setActive(serverId);
    state = const SessionLoading();
    await restore();
  }

  /// Déconnecte le serveur actif, sans l'oublier.
  ///
  /// L'adresse et l'identifiant restent : se reconnecter ne demandera que le
  /// mot de passe.
  Future<void> signOut() async {
    final current = state;
    if (current is! SessionActive) return;

    final tokens = await _store.tokens(current.server.id);
    if (tokens != null) {
      try {
        await current.client.logout(tokens.refresh);
      } catch (_) {
        // Le serveur peut être injoignable : les jetons locaux partent quand
        // même, c'est ce que l'utilisateur a demandé.
      }
    }

    await _store.clearTokens(current.server.id);
    state = SessionSignedOut(servers: await _store.servers());
  }

  /// Oublie un serveur et tout ce qui s'y rattache.
  Future<void> forget(String serverId) async {
    await _store.forget(serverId);
    state = const SessionLoading();
    await restore();
  }

  // ─── Rafraîchissement ──────────────────────────────────────────────────────

  ApiClient _clientFor(ServerAccount server, String access, String refresh) {
    final client = ApiClient(baseUrl: server.baseUrl);
    client.useTokens(SessionTokens(accessToken: access, refreshToken: refresh));
    client.onRefreshNeeded(() => _refresh(server.id, client));
    return client;
  }

  /*
    Rafraîchit le jeton, une seule fois à la fois.

    Le serveur fait tourner le jeton de rafraîchissement à chaque usage et
    considère la réutilisation d'un jeton consommé comme un vol : il révoque
    alors toute la session. Sans cette garde, cinq requêtes expirant ensemble en
    présenteraient quatre déjà consommés, et l'utilisateur serait déconnecté
    pour avoir simplement ouvert un écran.
  */
  Future<SessionTokens?> _refresh(String serverId, ApiClient client) {
    return _refreshInFlight ??= _doRefresh(serverId, client)
        .whenComplete(() => _refreshInFlight = null);
  }

  Future<SessionTokens?> _doRefresh(String serverId, ApiClient client) async {
    final stored = await _store.tokens(serverId);
    if (stored == null) return null;

    try {
      final tokens = await client.refresh(stored.refresh);
      await _store.saveTokens(serverId, tokens.accessToken, tokens.refreshToken);
      return SessionTokens(
        accessToken: tokens.accessToken,
        refreshToken: tokens.refreshToken,
      );
    } on NetworkException {
      // Hors ligne : le jeton n'est pas invalide, il est inatteignable.
      return null;
    } on ApiException {
      await _store.clearTokens(serverId);
      state = SessionSignedOut(servers: await _store.servers());
      return null;
    }
  }

  Future<String> _deviceName() async {
    if (kIsWeb) return 'Navigateur';
    try {
      return Platform.isIOS ? 'iPhone' : 'Android';
    } catch (_) {
      return 'Mobile';
    }
  }
}

/// Client de l'API du serveur actif, ou null si personne n'est connecté.
final apiProvider = Provider<ApiClient?>((ref) {
  final session = ref.watch(sessionProvider);
  return session is SessionActive ? session.client : null;
});
