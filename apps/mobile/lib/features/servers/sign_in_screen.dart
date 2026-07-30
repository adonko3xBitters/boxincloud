import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/auth/servers.dart';
import '../../core/auth/session.dart';
import '../../shared/theme.dart';
import '../../shared/tokens.dart';

/// Connexion à un serveur.
///
/// boxincloud est auto-hébergé : l'adresse du serveur est la première chose
/// qu'on demande, avant même l'identifiant. C'est ce qui distingue ce premier
/// écran de celui d'un service centralisé, où l'adresse est implicite.
class SignInScreen extends ConsumerStatefulWidget {
  final List<ServerAccount> knownServers;

  const SignInScreen({super.key, this.knownServers = const []});

  @override
  ConsumerState<SignInScreen> createState() => _SignInScreenState();
}

class _SignInScreenState extends ConsumerState<SignInScreen> {
  final _url = TextEditingController();
  final _username = TextEditingController();
  final _password = TextEditingController();

  bool _busy = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    // Un serveur déjà connu pré-remplit le formulaire : se reconnecter ne
    // demande alors que le mot de passe.
    final known = widget.knownServers;
    if (known.isNotEmpty) {
      _url.text = known.first.baseUrl;
      _username.text = known.first.username;
    }
  }

  @override
  void dispose() {
    _url.dispose();
    _username.dispose();
    _password.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    setState(() {
      _busy = true;
      _error = null;
    });

    try {
      await ref.read(sessionProvider.notifier).signIn(
            baseUrl: _url.text,
            username: _username.text,
            password: _password.text,
          );
    } on ApiException catch (e) {
      setState(() => _error = e.status == 401
          ? 'Identifiant ou mot de passe incorrect.'
          : e.message);
    } on NetworkException {
      setState(() => _error =
          'Serveur injoignable. Vérifiez l\'adresse et votre connexion.');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    return Scaffold(
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(BoxSpace.s6),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 420),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    'boxincloud',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                          fontWeight: FontWeight.w700,
                          color: colors.text,
                        ),
                  ),
                  const SizedBox(height: BoxSpace.s1),
                  Text(
                    'Votre bibliothèque de BD, comics et mangas.',
                    style: TextStyle(color: colors.textMuted),
                  ),
                  const SizedBox(height: BoxSpace.s8),

                  TextField(
                    controller: _url,
                    keyboardType: TextInputType.url,
                    autocorrect: false,
                    decoration: const InputDecoration(
                      labelText: 'Adresse du serveur',
                      hintText: 'bd.exemple.fr',
                      helperText: 'https par défaut. Précisez http:// en réseau local.',
                    ),
                  ),
                  const SizedBox(height: BoxSpace.s3),

                  TextField(
                    controller: _username,
                    autocorrect: false,
                    textInputAction: TextInputAction.next,
                    decoration: const InputDecoration(labelText: 'Identifiant'),
                  ),
                  const SizedBox(height: BoxSpace.s3),

                  TextField(
                    controller: _password,
                    obscureText: true,
                    onSubmitted: (_) => _busy ? null : _submit(),
                    decoration: const InputDecoration(labelText: 'Mot de passe'),
                  ),

                  if (_error != null) ...[
                    const SizedBox(height: BoxSpace.s4),
                    Container(
                      padding: const EdgeInsets.all(BoxSpace.s3),
                      decoration: BoxDecoration(
                        color: colors.danger.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(BoxRadius.md),
                        border: Border.all(color: colors.danger.withValues(alpha: 0.4)),
                      ),
                      child: Text(_error!, style: TextStyle(color: colors.danger)),
                    ),
                  ],

                  const SizedBox(height: BoxSpace.s6),
                  FilledButton(
                    onPressed: _busy ? null : _submit,
                    child: _busy
                        ? const SizedBox(
                            height: 18,
                            width: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : const Text('Se connecter'),
                  ),

                  if (widget.knownServers.length > 1) ...[
                    const SizedBox(height: BoxSpace.s6),
                    Text(
                      'Serveurs enregistrés',
                      style: TextStyle(
                        color: colors.textSubtle,
                        fontSize: 12,
                        letterSpacing: 0.4,
                      ),
                    ),
                    const SizedBox(height: BoxSpace.s2),
                    for (final server in widget.knownServers)
                      ListTile(
                        contentPadding: EdgeInsets.zero,
                        title: Text(server.label),
                        subtitle: Text(server.username),
                        onTap: () {
                          _url.text = server.baseUrl;
                          _username.text = server.username;
                        },
                      ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
