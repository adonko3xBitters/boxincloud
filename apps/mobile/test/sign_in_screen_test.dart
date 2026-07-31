import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:boxincloud/core/auth/servers.dart';
import 'package:boxincloud/features/servers/sign_in_screen.dart';
import 'package:boxincloud/shared/theme.dart';

/*
Le premier écran.

boxincloud est auto-hébergé : l'adresse du serveur y est demandée avant même
l'identifiant, ce qui le distingue d'un service centralisé. Ces tests fixent ce
que cet écran doit montrer — faute de pouvoir le regarder tourner sur un
appareil.
*/
void main() {
  Widget host(Widget child) => ProviderScope(
        child: MaterialApp(theme: boxTheme(Brightness.dark), home: child),
      );

  testWidgets('demande l\'adresse du serveur avant l\'identifiant',
      (tester) async {
    await tester.pumpWidget(host(const SignInScreen()));

    expect(find.text('Adresse du serveur'), findsOneWidget);
    expect(find.text('Identifiant'), findsOneWidget);
    expect(find.text('Mot de passe'), findsOneWidget);

    // L'ordre compte : sur un produit auto-hébergé, l'adresse est la première
    // question, pas un réglage avancé caché en bas.
    final url = tester.getTopLeft(find.text('Adresse du serveur'));
    final user = tester.getTopLeft(find.text('Identifiant'));
    expect(url.dy, lessThan(user.dy));
  });

  testWidgets('explique le protocole par défaut', (tester) async {
    await tester.pumpWidget(host(const SignInScreen()));

    // Sans cette mention, une adresse de réseau local en http échoue sans que
    // rien n'indique pourquoi.
    expect(
      find.textContaining('https par défaut'),
      findsOneWidget,
    );
  });

  testWidgets('pré-remplit un serveur déjà connu', (tester) async {
    await tester.pumpWidget(host(const SignInScreen(
      knownServers: [
        ServerAccount(
          id: 'x',
          baseUrl: 'https://bd.exemple.fr',
          label: 'bd.exemple.fr',
          username: 'niando',
        ),
      ],
    )));

    // Se reconnecter ne doit demander que le mot de passe.
    expect(find.text('https://bd.exemple.fr'), findsOneWidget);
    expect(find.text('niando'), findsOneWidget);
  });

  testWidgets('propose de choisir parmi plusieurs serveurs', (tester) async {
    await tester.pumpWidget(host(const SignInScreen(
      knownServers: [
        ServerAccount(id: 'a', baseUrl: 'https://a.fr', label: 'Maison', username: 'moi'),
        ServerAccount(id: 'b', baseUrl: 'https://b.fr', label: 'Chez Paul', username: 'invite'),
      ],
    )));

    expect(find.text('Serveurs enregistrés'), findsOneWidget);
    expect(find.text('Chez Paul'), findsOneWidget);
  });

  testWidgets('un seul serveur connu n\'affiche pas de liste', (tester) async {
    await tester.pumpWidget(host(const SignInScreen(
      knownServers: [
        ServerAccount(id: 'a', baseUrl: 'https://a.fr', label: 'Maison', username: 'moi'),
      ],
    )));

    // Une liste d'un élément n'aide personne à choisir.
    expect(find.text('Serveurs enregistrés'), findsNothing);
  });
}
