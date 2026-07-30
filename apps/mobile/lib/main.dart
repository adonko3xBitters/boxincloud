import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/auth/session.dart';
import 'features/library/library_screen.dart';
import 'features/servers/sign_in_screen.dart';
import 'shared/theme.dart';

void main() {
  runApp(const ProviderScope(child: BoxincloudApp()));
}

class BoxincloudApp extends StatelessWidget {
  const BoxincloudApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'boxincloud',
      debugShowCheckedModeBanner: false,
      theme: boxTheme(Brightness.light),
      darkTheme: boxTheme(Brightness.dark),

      // Le thème sombre est le défaut assumé du produit — on lit des BD le
      // soir, et une planche ressort mieux sur fond sombre. Le système garde le
      // dernier mot.
      themeMode: ThemeMode.system,

      home: const _Root(),
    );
  }
}

/// Aiguillage entre connexion et bibliothèque.
///
/// Un seul point de décision, dérivé de l'état de session : disséminer ce choix
/// dans plusieurs écrans finirait par produire deux vérités sur « suis-je
/// connecté ».
class _Root extends ConsumerWidget {
  const _Root();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);

    return switch (session) {
      SessionLoading() => const Scaffold(
          body: Center(child: CircularProgressIndicator()),
        ),
      SessionSignedOut(:final servers) => SignInScreen(knownServers: servers),
      SessionActive() => const LibraryScreen(),
    };
  }
}
