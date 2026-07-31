import 'dart:async';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api/client.dart';
import '../../core/api/models.dart';
import '../../core/auth/session.dart';
import '../../core/sync/progress_sync.dart';
import '../library/library_controller.dart';
import '../../shared/tokens.dart';

/// Sens de lecture.
enum ReadingDirection { leftToRight, rightToLeft }

final directionProvider =
    StateProvider<ReadingDirection>((ref) => ReadingDirection.leftToRight);

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

  ProgressSync? get _sync {
    final session = ref.read(sessionProvider);
    if (session is! SessionActive) return null;
    return ProgressSync(db: ref.read(databaseProvider), serverId: session.server.id);
  }

  Future<void> _load() async {
    final session = ref.read(sessionProvider);
    if (session is! SessionActive) return;

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
      if (mounted) {
        setState(() => _error =
            'Serveur injoignable. Les albums téléchargés restent lisibles hors ligne.');
      }
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e.message);
    }
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
              onToggleDirection: () {
                ref.read(directionProvider.notifier).state =
                    direction == ReadingDirection.leftToRight
                        ? ReadingDirection.rightToLeft
                        : ReadingDirection.leftToRight;
              },
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
  final VoidCallback onTapCenter;

  const _Page({
    required this.comicId,
    required this.index,
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

    final url = session.client.imageUrl(
      '/api/v1/comics/${widget.comicId}/pages/${widget.index}',
      width: 1600,
    );

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
          child: CachedNetworkImage(
            imageUrl: url,
            fit: BoxFit.contain,
            fadeInDuration: const Duration(milliseconds: 120),
            placeholder: (_, _) => const Center(
              child: SizedBox(
                height: 28,
                width: 28,
                child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white24),
              ),
            ),
            errorWidget: (_, _, _) => const Center(
              child: Icon(Icons.broken_image_outlined, color: Colors.white24, size: 40),
            ),
          ),
        ),
      ),
    );
  }
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
