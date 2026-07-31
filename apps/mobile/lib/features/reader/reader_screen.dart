import 'dart:async';
import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/api/models.dart';
import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../core/offline/download_manager.dart';
import '../../core/offline/storage.dart';
import '../../core/sync/progress_sync.dart';
import '../library/library_controller.dart';
import '../../shared/tokens.dart';

/// Sens de lecture.
enum ReadingDirection { leftToRight, rightToLeft }

/*
Sens de lecture, retenu d'une session à l'autre.

Quelqu'un qui lit des mangas en lit rarement un seul. Un réglage oublié à
chaque lancement l'obligerait à rebasculer avant chaque album — une correction
manuelle, systématique, d'un défaut de mémoire de l'application.

La préférence n'est pas rattachée à un serveur : c'est une habitude de
personne, pas de bibliothèque.
*/
class ReadingDirectionNotifier extends Notifier<ReadingDirection> {
  static const _key = 'reader.direction';

  @override
  ReadingDirection build() => ReadingDirection.leftToRight;

  /// Relit la préférence enregistrée.
  ///
  /// Appelée par le lecteur pendant son chargement, donc avant que le sens ne
  /// serve à quoi que ce soit : la lire dans `build()` ferait rendre une
  /// première image dans le mauvais sens, puis basculer sous les yeux.
  Future<void> restore() async {
    final stored = await ref.read(databaseProvider).preference(_key);
    if (stored == null) return;

    state = ReadingDirection.values.firstWhere(
      (d) => d.name == stored,
      orElse: () => ReadingDirection.leftToRight,
    );
  }

  Future<void> toggle() async {
    state = state == ReadingDirection.leftToRight
        ? ReadingDirection.rightToLeft
        : ReadingDirection.leftToRight;

    await ref.read(databaseProvider).setPreference(_key, state.name);
  }
}

final directionProvider =
    NotifierProvider<ReadingDirectionNotifier, ReadingDirection>(
        ReadingDirectionNotifier.new);

/*
Lecteur.

La pièce sur laquelle l'application sera jugée. Trois principes la gouvernent,
les mêmes que sur le web.

L'interface s'efface : on lit une planche, pas une application. Les barres
apparaissent au tap central et disparaissent.

La page suivante est déjà là : le préchargement fait que tourner une page est
instantané, jamais un chargement.

La position est écrite localement à chaque page : fermer l'application dans le
métro ne doit pas coûter sa lecture.
*/
class ReaderScreen extends ConsumerStatefulWidget {
  final String comicId;
  final String title;

  const ReaderScreen({super.key, required this.comicId, required this.title});

  @override
  ConsumerState<ReaderScreen> createState() => _ReaderScreenState();
}

class _ReaderScreenState extends ConsumerState<ReaderScreen> {
  PageController? _controller;

  Manifest? _manifest;
  String? _error;

  int _page = 0;
  bool _chromeVisible = true;

  /// Répertoire des pages téléchargées, résolu une fois pour toutes.
  ///
  /// Le résoudre ici plutôt que dans chaque page évite un `FutureBuilder` par
  /// planche : la construction d'une page doit être synchrone, sinon tourner
  /// une page coûte une image blanche.
  String? _offlineDirectory;

  /// Nombre de pages présentes sur le disque, à partir de zéro.
  int _localPages = 0;

  @override
  void initState() {
    super.initState();

    // Plein écran immersif : les barres du système volent de la hauteur à une
    // planche, qui est verticale par nature.
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.immersiveSticky);
    _load();
  }

  @override
  void dispose() {
    SystemChrome.setEnabledSystemUIMode(SystemUiMode.edgeToEdge);
    _controller?.dispose();
    super.dispose();
  }

  DownloadManager? _manager(SessionActive session) => DownloadManager(
        db: ref.read(databaseProvider),
        client: session.client,
        serverId: session.server.id,
      );

  ProgressSync? get _sync {
    final session = ref.read(sessionProvider);
    if (session is! SessionActive) return null;
    return ProgressSync(db: ref.read(databaseProvider), serverId: session.server.id);
  }

  /*
    Chargement.

    Le téléchargement local est consulté AVANT le serveur, et c'est ce qui rend
    le mode avion utilisable. Il donne deux choses que le manifeste donnerait :
    le nombre de pages, et le fait que les pages sont sur le disque. Le serveur
    n'est alors plus qu'une confirmation — utile quand il répond, jamais
    bloquante quand il ne répond pas.

    Un album non téléchargé, lui, dépend du réseau. C'est la différence
    qu'annonce le bouton « Télécharger », et elle doit rester lisible : on ne
    promet pas le hors ligne à qui n'a rien demandé.
  */
  Future<void> _load() async {
    final session = ref.read(sessionProvider);
    if (session is! SessionActive) return;

    // Avant le manifeste : le sens de lecture doit être connu à la première
    // image, et ce chargement-ci ne touche que la base locale.
    await ref.read(directionProvider.notifier).restore();

    final db = ref.read(databaseProvider);
    final serverId = session.server.id;

    final download = await db.download(serverId, widget.comicId);
    final directory = await comicDirectory(serverId, widget.comicId);

    if (download != null) {
      // L'éviction sacrifie le plus anciennement ouvert : ouvrir remet le
      // compteur à zéro, sans quoi l'album qu'on lit tous les soirs finirait
      // par être celui qu'on efface.
      unawaited(_manager(session)?.markRead(widget.comicId) ?? Future.value());
    }

    if (!mounted) return;
    setState(() {
      _offlineDirectory = directory.path;
      _localPages = download?.pagesDone ?? 0;
    });

    try {
      final manifest = await session.client.manifest(widget.comicId);
      final resume = await _sync?.resumePage(session.client, widget.comicId) ?? 0;

      if (!mounted) return;
      setState(() {
        _manifest = manifest;
        _page = resume.clamp(0, manifest.pageCount - 1);
        _controller = PageController(initialPage: _page);
      });

      // La suivante est demandée avant même que l'utilisateur ne tourne : sur
      // un album repris en cours, c'est le premier geste qu'il fera.
      _prefetch(_page);
    } on NetworkException {
      await _loadOffline(download);
    } on ApiException catch (e) {
      // Un album téléchargé reste lisible même si le serveur le refuse
      // aujourd'hui — droits révoqués, album retiré du catalogue.
      if (download != null && download.pagesDone > 0) {
        await _loadOffline(download);
        return;
      }
      if (mounted) setState(() => _error = e.message);
    }
  }

  /// Ouvre depuis le disque seul, sans jamais interroger le serveur.
  Future<void> _loadOffline(Download? download) async {
    if (download == null || download.pagesDone == 0) {
      if (mounted) {
        setState(() => _error =
            'Serveur injoignable, et cet album n\'est pas téléchargé.');
      }
      return;
    }

    final resume = await _sync?.resumePage(null, widget.comicId) ?? 0;

    if (!mounted) return;
    setState(() {
      // Un téléchargement inachevé se lit jusqu'où il va : mieux vaut quinze
      // pages lisibles qu'un message d'erreur devant un album à moitié présent.
      _manifest = Manifest(
        comicId: widget.comicId,
        pageCount: download.pagesDone,
        pages: const [],
      );
      _page = resume.clamp(0, download.pagesDone - 1);
      _controller = PageController(initialPage: _page);
    });
  }

  /// URL d'une page, à la définition d'affichage.
  String _pageUrl(int index) {
    final session = ref.read(sessionProvider);
    if (session is! SessionActive) return '';
    return session.client.imageUrl(
      '/api/v1/comics/${widget.comicId}/pages/$index',
      width: 1600,
    );
  }

  /*
    Précharge les pages voisines.

    Trois en avant, une en arrière. En avant parce que c'est le sens de lecture ;
    une seule en arrière parce que revenir est rare, mais assez fréquent pour
    qu'un retour instantané se remarque.

    Sans cela, tourner une page affiche un indicateur de chargement — et une
    lecture entrecoupée de chargements est une lecture gâchée. Le cache d'images
    conserve ce qui a été demandé, il suffit donc de demander en avance.
  */
  void _prefetch(int around) {
    final manifest = _manifest;
    if (manifest == null) return;

    for (var offset = -1; offset <= 3; offset++) {
      final index = around + offset;
      if (index < 0 || index >= manifest.pageCount || index == around) continue;

      // Une page locale se décode depuis le disque : rien à demander d'avance,
      // et précharger consommerait de la mémoire pour un gain nul.
      if (index < _localPages) continue;

      final url = _pageUrl(index);
      if (url.isEmpty) continue;
      unawaited(precacheImage(CachedNetworkImageProvider(url), context));
    }
  }

  void _onPageChanged(int index) {
    setState(() => _page = index);
    _prefetch(index);

    final session = ref.read(sessionProvider);
    _sync?.record(
      session is SessionActive ? session.client : null,
      comicId: widget.comicId,
      page: index,
      pageCount: _manifest?.pageCount ?? 0,
    );
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Scaffold(
        appBar: AppBar(),
        body: Center(
          child: Padding(
            padding: const EdgeInsets.all(BoxSpace.s8),
            child: Text(_error!, textAlign: TextAlign.center),
          ),
        ),
      );
    }

    final manifest = _manifest;
    if (manifest == null || _controller == null) {
      return const Scaffold(
        backgroundColor: Colors.black,
        body: Center(child: CircularProgressIndicator()),
      );
    }

    final direction = ref.watch(directionProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        children: [
          PageView.builder(
            controller: _controller,
            onPageChanged: _onPageChanged,
            // En lecture manga, les pages défilent de droite à gauche. Le
            // renversement se fait ici plutôt qu'en inversant les index, ce qui
            // fausserait la progression enregistrée.
            reverse: direction == ReadingDirection.rightToLeft,
            itemCount: manifest.pageCount,
            itemBuilder: (context, index) => _Page(
              comicId: widget.comicId,
              index: index,
              // Le chemin local n'est passé que s'il existe : la page décide
              // alors sans rien avoir à vérifier elle-même.
              localPath: index < _localPages && _offlineDirectory != null
                  ? '$_offlineDirectory/$index'
                  : null,
              onTapCenter: () => setState(() => _chromeVisible = !_chromeVisible),
            ),
          ),

          if (_chromeVisible) ...[
            _TopBar(title: widget.title),
            _BottomBar(
              page: _page,
              pageCount: manifest.pageCount,
              onSeek: (value) {
                _controller!.jumpToPage(value);
              },
              direction: direction,
              onToggleDirection: () => ref.read(directionProvider.notifier).toggle(),
            ),
          ],
        ],
      ),
    );
  }
}

// ─── Page ────────────────────────────────────────────────────────────────────

/// Une planche, zoomable.
///
/// `InteractiveViewer` porte le pincement et le déplacement. Le zoom est remis à
/// plat en changeant de page : rester agrandi sur un coin arbitraire de la
/// planche suivante n'a jamais de sens.
class _Page extends ConsumerStatefulWidget {
  final String comicId;
  final int index;

  /// Fichier local, quand la page a été téléchargée.
  final String? localPath;

  final VoidCallback onTapCenter;

  const _Page({
    required this.comicId,
    required this.index,
    required this.localPath,
    required this.onTapCenter,
  });

  @override
  ConsumerState<_Page> createState() => _PageState();
}

class _PageState extends ConsumerState<_Page> {
  final _transform = TransformationController();

  @override
  void dispose() {
    _transform.dispose();
    super.dispose();
  }

  /// Double-tap : bascule entre taille normale et agrandissement au point visé.
  void _onDoubleTap(TapDownDetails details) {
    if (_transform.value != Matrix4.identity()) {
      _transform.value = Matrix4.identity();
      return;
    }

    final position = details.localPosition;
    _transform.value = Matrix4.identity()
      ..translateByDouble(-position.dx * 1.5, -position.dy * 1.5, 0, 1)
      ..scaleByDouble(2.5, 2.5, 2.5, 1);
  }

  @override
  Widget build(BuildContext context) {
    final session = ref.watch(sessionProvider);
    if (session is! SessionActive) return const SizedBox.shrink();

    final local = widget.localPath;

    return GestureDetector(
      onTapUp: (details) {
        // Le tiers central révèle l'interface ; les bords appartiennent au
        // défilement des pages, qui est le geste dominant.
        final width = MediaQuery.of(context).size.width;
        final x = details.localPosition.dx;
        if (x > width * 0.35 && x < width * 0.65) widget.onTapCenter();
      },
      onDoubleTapDown: _onDoubleTap,
      onDoubleTap: () {},
      child: InteractiveViewer(
        transformationController: _transform,
        minScale: 1,
        maxScale: 5,
        child: Center(
          child: local != null
              // Sans fondu : le fichier est déjà là, et une transition
              // simulerait un chargement qui n'a pas lieu.
              ? Image.file(
                  File(local),
                  fit: BoxFit.contain,
                  errorBuilder: (_, _, _) => const _BrokenPage(),
                )
              : CachedNetworkImage(
                  imageUrl: session.client.imageUrl(
                    '/api/v1/comics/${widget.comicId}/pages/${widget.index}',
                    width: 1600,
                  ),
                  fit: BoxFit.contain,
                  fadeInDuration: const Duration(milliseconds: 120),
                  placeholder: (_, _) => const Center(
                    child: SizedBox(
                      height: 28,
                      width: 28,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white24),
                    ),
                  ),
                  errorWidget: (_, _, _) => const _BrokenPage(),
                ),
        ),
      ),
    );
  }
}

class _BrokenPage extends StatelessWidget {
  const _BrokenPage();

  @override
  Widget build(BuildContext context) => const Center(
        child: Icon(Icons.broken_image_outlined, color: Colors.white24, size: 40),
      );
}

// ─── Habillage ───────────────────────────────────────────────────────────────

class _TopBar extends StatelessWidget {
  final String title;

  const _TopBar({required this.title});

  @override
  Widget build(BuildContext context) {
    return Positioned(
      top: 0,
      left: 0,
      right: 0,
      child: Container(
        padding: EdgeInsets.only(
          top: MediaQuery.of(context).padding.top + BoxSpace.s2,
          left: BoxSpace.s2,
          right: BoxSpace.s3,
          bottom: BoxSpace.s6,
        ),
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topCenter,
            end: Alignment.bottomCenter,
            colors: [Colors.black87, Colors.transparent],
          ),
        ),
        child: Row(
          children: [
            IconButton(
              icon: const Icon(Icons.arrow_back, color: Colors.white),
              onPressed: () => Navigator.of(context).pop(),
            ),
            Expanded(
              child: Text(
                title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(color: Colors.white, fontSize: 15),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _BottomBar extends StatelessWidget {
  final int page;
  final int pageCount;
  final ValueChanged<int> onSeek;
  final ReadingDirection direction;
  final VoidCallback onToggleDirection;

  const _BottomBar({
    required this.page,
    required this.pageCount,
    required this.onSeek,
    required this.direction,
    required this.onToggleDirection,
  });

  @override
  Widget build(BuildContext context) {
    return Positioned(
      bottom: 0,
      left: 0,
      right: 0,
      child: Container(
        padding: EdgeInsets.only(
          top: BoxSpace.s8,
          left: BoxSpace.s4,
          right: BoxSpace.s3,
          bottom: MediaQuery.of(context).padding.bottom + BoxSpace.s2,
        ),
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.bottomCenter,
            end: Alignment.topCenter,
            colors: [Colors.black87, Colors.transparent],
          ),
        ),
        child: Row(
          children: [
            SizedBox(
              width: 72,
              child: Text(
                '${page + 1} / $pageCount',
                style: const TextStyle(
                  color: Colors.white70,
                  fontSize: 12,
                  fontFeatures: [FontFeature.tabularFigures()],
                ),
              ),
            ),
            Expanded(
              // Un Slider plutôt qu'une barre dessinée à la main : il apporte
              // gratuitement l'accessibilité et le retour tactile.
              child: Slider(
                value: page.toDouble().clamp(0, (pageCount - 1).toDouble()),
                max: (pageCount - 1).toDouble().clamp(1, double.infinity),
                onChanged: (value) => onSeek(value.round()),
              ),
            ),
            IconButton(
              tooltip: direction == ReadingDirection.rightToLeft
                  ? 'Lecture manga (droite à gauche)'
                  : 'Lecture classique (gauche à droite)',
              icon: Icon(
                direction == ReadingDirection.rightToLeft
                    ? Icons.format_textdirection_r_to_l
                    : Icons.format_textdirection_l_to_r,
                color: Colors.white70,
              ),
              onPressed: onToggleDirection,
            ),
          ],
        ),
      ),
    );
  }
}
