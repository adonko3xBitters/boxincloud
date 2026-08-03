import 'dart:async';
import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
// Pour `ScrollCacheExtent`, que material.dart ne réexporte pas.
import 'package:flutter/rendering.dart';
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
import 'reader_settings.dart';

// Réexporté : les réglages ont été sortis d'ici, et tout ce qui importait le
// lecteur pour son sens de lecture continue de fonctionner.
export 'reader_settings.dart';

/// Ce que déclenche un toucher, selon l'endroit de l'écran.
enum ReaderTapZone { backward, chrome, forward }

/*
Répartition du toucher : un tiers, un tiers, un tiers.

Le tiers central appelle l'habillage ; les deux bords tournent la page. Lequel
avance dépend du sens de lecture — en manga on va vers la gauche, et c'est donc
le bord gauche qui fait avancer.

Extrait de la vue et rendu public pour être testable : l'inversion en manga est
exactement le genre de chose qu'on écrit à l'envers sans que rien ne le dise,
puisque les deux sens produisent une application qui tourne les pages.
*/
ReaderTapZone readerTapZone({
  required double x,
  required double width,
  required bool rightToLeft,
}) {
  if (width <= 0) return ReaderTapZone.chrome;

  final position = x / width;
  if (position > 0.33 && position < 0.67) return ReaderTapZone.chrome;

  final towardsStart = position <= 0.33;
  return towardsStart == rightToLeft
      ? ReaderTapZone.forward
      : ReaderTapZone.backward;
}

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

  /*
    La planche courante est-elle agrandie ?

    Cette information appartient à l'écran, alors que le zoom vit dans la page.
    Elle remonte parce que le PageView doit s'arrêter de défiler tant qu'on est
    agrandi : les deux gestes sont le même — un glissement horizontal — et
    l'arène de gestes de Flutter tranche en faveur du parent qui défile. On
    zoomait donc sur un détail, on tirait pour voir le reste de la planche, et
    on changeait de page.

    Le déplacement dans l'image est le geste attendu à ce moment-là ; tourner
    la page ne l'est pas. Il redevient disponible en repassant à l'échelle 1,
    d'un double tap ou d'un pincement.
  */
  bool _zoomed = false;

  bool _filmstripOpen = false;
  bool _settingsOpen = false;

  /*
    Écriture de la progression, différée.

    `record` écrit en base ET appelle le serveur, sans regroupement. Tirer le
    curseur d'un album de deux cents planches déclenchait donc deux cents
    écritures et deux cents requêtes, pour une seule intention : aller à une
    page. Seule la dernière position compte.

    Le délai est court — on veut que fermer l'application juste après avoir
    tourné une page conserve la position. Ce qui reste en attente est écrit
    dans `dispose`, sans quoi le dernier geste d'une lecture serait le seul
    jamais enregistré.
  */
  static const _syncDelay = Duration(milliseconds: 600);
  Timer? _syncTimer;
  int? _pendingPage;
  ProgressSync? _progressSync;
  ApiClient? _progressClient;

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

    // Ce qui attendait encore part maintenant. `ref` n'est plus consultable
    // après cette méthode, d'où les références capturées au chargement.
    _syncTimer?.cancel();
    _flushProgress();

    _controller?.dispose();
    _webtoonScroll?.dispose();
    super.dispose();
  }

  /// Écrit la position en attente, s'il y en a une.
  void _flushProgress() {
    final page = _pendingPage;
    final sync = _progressSync;
    if (page == null || sync == null) return;

    _pendingPage = null;
    unawaited(
      sync.record(
        _progressClient,
        comicId: widget.comicId,
        page: page,
        pageCount: _manifest?.pageCount ?? 0,
      ),
    );
  }

  DownloadManager? _manager(SessionActive session) => DownloadManager(
    db: ref.read(databaseProvider),
    client: session.client,
    serverId: session.server.id,
  );

  ProgressSync? get _sync {
    final session = ref.read(sessionProvider);
    if (session is! SessionActive) return null;
    return ProgressSync(
      db: ref.read(databaseProvider),
      serverId: session.server.id,
    );
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

    // Avant le manifeste : le mode et le sens doivent être connus à la première
    // image, et ce chargement-ci ne touche que la base locale.
    await ref.read(readerSettingsProvider.notifier).restore();

    final db = ref.read(databaseProvider);
    final serverId = session.server.id;

    // Capturés ici pour rester utilisables dans `dispose`, où le conteneur de
    // providers ne peut plus être consulté.
    _progressSync = _sync;
    _progressClient = session.client;

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
      final resume =
          await _sync?.resumePage(session.client, widget.comicId) ?? 0;

      if (!mounted) return;
      setState(() {
        _manifest = manifest;
        _page = resume.clamp(0, manifest.pageCount - 1);
        // En défilement continu, la reprise est un décalage à retrouver, pas
        // une page à afficher : elle attend la première mesure.
        _pendingWebtoonJump = _page;
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
        setState(
          () => _error =
              'Serveur injoignable, et cet album n\'est pas téléchargé.',
        );
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
      _pendingWebtoonJump = _page;
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
    setState(() {
      _page = index;
      // Chaque planche s'ouvre à l'échelle 1. La page précédente est démontée
      // avec son zoom, mais l'écran garde le sien : sans cette remise à plat,
      // un saut depuis le curseur laisserait le défilement verrouillé sur une
      // planche qui n'est plus agrandie.
      _zoomed = false;
    });
    _prefetch(index);

    _pendingPage = index;
    _syncTimer?.cancel();
    _syncTimer = Timer(_syncDelay, _flushProgress);
  }

  /*
    Tourne d'une planche.

    L'index avance toujours dans le sens de la lecture, quel que soit le sens
    d'affichage : c'est `reverse` sur le PageView qui inverse le mouvement à
    l'écran, et la progression enregistrée reste donc juste en manga.
  */
  void _turn(int delta) {
    final manifest = _manifest;
    final controller = _controller;
    if (manifest == null || controller == null) return;

    final target = _page + delta;
    if (target < 0 || target >= manifest.pageCount) return;

    controller.animateToPage(
      target,
      duration: const Duration(milliseconds: 220),
      curve: Curves.easeOutCubic,
    );
  }

  /// Va à une planche, dans le mode courant.
  ///
  /// Le curseur et les vignettes désignent une planche ; c'est au lecteur de
  /// savoir si cela veut dire tourner ou faire défiler.
  void _goToPage(int index) {
    if (ref.read(readerSettingsProvider).mode == ReaderMode.webtoon) {
      _scrollToPage(index);
      return;
    }
    _controller?.jumpToPage(index);
  }

  // ─── Défilement continu ────────────────────────────────────────────────────

  ScrollController? _webtoonScroll;

  /// Décalage du haut de chaque planche, et hauteur totale en dernière valeur.
  ///
  /// Calculé à la mise en page, à partir des proportions du manifeste : c'est
  /// ce qui permet de savoir quelle planche on lit sans attendre que les images
  /// arrivent, et d'y sauter depuis le curseur.
  List<double> _webtoonOffsets = const [];

  /// Largeur pour laquelle les décalages ont été calculés.
  double _webtoonWidth = 0;

  /// Planche à rejoindre dès que la mise en page sera connue.
  ///
  /// La reprise arrive avant la première mesure : sans cette attente, on
  /// sauterait dans une liste dont on ignore encore les hauteurs.
  int? _pendingWebtoonJump;

  ScrollController get _webtoonController =>
      _webtoonScroll ??= ScrollController();

  /// Proportions d'une planche, quand le manifeste les donne.
  ///
  /// Une lecture hors ligne n'a pas de manifeste détaillé. La valeur de repli
  /// est celle d'une planche ordinaire, et l'image est alors contenue dans la
  /// case réservée plutôt qu'étirée : mieux vaut une bande blanche qu'une
  /// hauteur qui saute quand l'image arrive.
  double _pageRatio(int index) {
    final pages = _manifest?.pages ?? const [];
    if (index >= pages.length) return 0.7;

    final page = pages[index];
    final width = page.width;
    final height = page.height;
    if (width == null || height == null || height == 0) return 0.7;

    return width / height;
  }

  /// Recalcule les décalages si la largeur a changé.
  void _measureWebtoon(double contentWidth, int pageCount) {
    if (contentWidth == _webtoonWidth &&
        _webtoonOffsets.length == pageCount + 1) {
      return;
    }

    final offsets = <double>[0];
    for (var index = 0; index < pageCount; index++) {
      offsets.add(offsets.last + contentWidth / _pageRatio(index));
    }

    _webtoonWidth = contentWidth;
    _webtoonOffsets = offsets;
  }

  void _scrollToPage(int index) {
    final controller = _webtoonScroll;
    if (controller == null || index + 1 >= _webtoonOffsets.length) {
      _pendingWebtoonJump = index;
      return;
    }
    if (!controller.hasClients) {
      _pendingWebtoonJump = index;
      return;
    }

    controller.jumpTo(
      _webtoonOffsets[index].clamp(0, controller.position.maxScrollExtent),
    );
  }

  /// Quelle planche occupe le haut de l'écran.
  int _pageAtOffset(double offset) {
    if (_webtoonOffsets.length < 2) return 0;

    // Le haut de l'écran plutôt que son centre : c'est la planche qu'on est en
    // train de finir qui compte pour la reprise, pas celle qui arrive.
    for (var index = _webtoonOffsets.length - 2; index >= 0; index--) {
      if (offset >= _webtoonOffsets[index]) return index;
    }
    return 0;
  }

  void _onWebtoonScroll(double offset) {
    final index = _pageAtOffset(offset);
    if (index == _page) return;
    _onPageChanged(index);
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

    final settings = ref.watch(readerSettingsProvider);

    // Ouvrir un panneau montre l'habillage : le refermer sur une barre disparue
    // laisserait l'écran sans repère.
    final chromeVisible = _chromeVisible || _filmstripOpen || _settingsOpen;

    return Scaffold(
      backgroundColor: Colors.black,
      body: Stack(
        children: [
          settings.mode == ReaderMode.webtoon
              ? _buildWebtoon(manifest, settings)
              : _buildPaged(manifest, settings),

          if (chromeVisible) _TopBar(title: widget.title),

          /*
            Le bas de l'écran, en une seule colonne.

            La barre est AU-DESSUS du panneau ouvert, et c'est le point. Les
            deux étaient auparavant ancrés au bas de la pile : le panneau
            recouvrait donc les boutons qui l'avaient ouvert, et passer des
            réglages aux vignettes demandait de fermer, viser la barre
            réapparue, rouvrir. Le voyant d'activité sur ces boutons n'était
            même jamais visible.
          */
          if (chromeVisible)
            Positioned(
              left: 0,
              right: 0,
              bottom: 0,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  _BottomBar(
                    page: _page,
                    pageCount: manifest.pageCount,
                    onSeek: _goToPage,
                    filmstripOpen: _filmstripOpen,
                    onToggleFilmstrip: () => setState(() {
                      _filmstripOpen = !_filmstripOpen;
                      if (_filmstripOpen) _settingsOpen = false;
                    }),
                    settingsOpen: _settingsOpen,
                    onToggleSettings: () => setState(() {
                      _settingsOpen = !_settingsOpen;
                      if (_settingsOpen) _filmstripOpen = false;
                    }),
                    panelBelow: _filmstripOpen || _settingsOpen,
                  ),

                  if (_settingsOpen)
                    _SettingsSheet(
                      settings: settings,
                      onClose: () => setState(() => _settingsOpen = false),
                    ),

                  if (_filmstripOpen)
                    _Filmstrip(
                      comicId: widget.comicId,
                      manifest: manifest,
                      current: _page,
                      offlineDirectory: _offlineDirectory,
                      localPages: _localPages,
                      onSelect: (index) {
                        _goToPage(index);
                        setState(() => _filmstripOpen = false);
                      },
                      onClose: () => setState(() => _filmstripOpen = false),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }

  /// Bascule l'habillage, ou referme le panneau ouvert.
  void _onTapCenter() => setState(() {
    if (_filmstripOpen || _settingsOpen) {
      _filmstripOpen = false;
      _settingsOpen = false;
    } else {
      _chromeVisible = !_chromeVisible;
    }
  });

  String? _localPathOf(int index) =>
      index < _localPages && _offlineDirectory != null
      ? '$_offlineDirectory/$index'
      : null;

  // ─── Planche par planche ───────────────────────────────────────────────────

  Widget _buildPaged(Manifest manifest, ReaderSettings settings) {
    final rightToLeft = settings.direction == ReadingDirection.rightToLeft;

    return PageView.builder(
      controller: _controller,
      onPageChanged: _onPageChanged,
      // Agrandi, le glissement horizontal appartient à l'image. Voir `_zoomed` :
      // c'est ici que la correction prend effet.
      physics: _zoomed
          ? const NeverScrollableScrollPhysics()
          : const PageScrollPhysics(),
      // En lecture manga, les pages défilent de droite à gauche. Le renversement
      // se fait ici plutôt qu'en inversant les index, ce qui fausserait la
      // progression enregistrée.
      reverse: rightToLeft,
      itemCount: manifest.pageCount,
      itemBuilder: (context, index) => _Page(
        comicId: widget.comicId,
        index: index,
        // Le chemin local n'est passé que s'il existe : la page décide alors
        // sans rien avoir à vérifier elle-même.
        localPath: _localPathOf(index),
        margin: settings.margin,
        onTapCenter: _onTapCenter,
        // Les bords tournent la page. Ils ne faisaient rien jusqu'ici : le seul
        // moyen d'avancer était de balayer, ce qui demande le pouce à mi-écran
        // et se tient mal à une main.
        onTapForward: () => _turn(1),
        onTapBackward: () => _turn(-1),
        rightToLeft: rightToLeft,
        onZoomChanged: (zoomed) {
          if (zoomed != _zoomed) setState(() => _zoomed = zoomed);
        },
      ),
    );
  }

  // ─── Défilement continu ────────────────────────────────────────────────────

  /*
    Les planches s'enchaînent sans coupure, à la largeur de l'écran.

    C'est le mode des webtoons, dont les planches sont des bandes verticales
    conçues pour se suivre : les tourner une par une coupe le récit au milieu
    d'une case. Il sert aussi à qui préfère faire défiler une bande dessinée
    ordinaire plutôt que de tourner.

    La hauteur de chaque case est réservée d'avance, d'après les proportions du
    manifeste. Sans cela, la liste sauterait à chaque image qui arrive, et la
    position de reprise ne voudrait rien dire.
  */
  Widget _buildWebtoon(Manifest manifest, ReaderSettings settings) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final inset = constraints.maxWidth * settings.margin.fraction;
        final contentWidth = constraints.maxWidth - inset * 2;

        _measureWebtoon(contentWidth, manifest.pageCount);

        // La reprise et les sauts demandés avant la première mesure sont
        // appliqués maintenant que les hauteurs sont connues.
        final pending = _pendingWebtoonJump;
        if (pending != null) {
          _pendingWebtoonJump = null;
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted) _scrollToPage(pending);
          });
        }

        return GestureDetector(
          // Aucune zone de bord ici : le geste du mode est le défilement, et
          // découper l'écran en trois ferait tourner la page par accident au
          // moindre appui pendant qu'on fait glisser.
          onTap: _onTapCenter,
          child: NotificationListener<ScrollUpdateNotification>(
            onNotification: (notification) {
              _onWebtoonScroll(notification.metrics.pixels);
              return false;
            },
            child: ListView.builder(
              controller: _webtoonController,
              padding: EdgeInsets.symmetric(horizontal: inset),
              itemCount: manifest.pageCount,
              // Les planches voisines restent construites : dans ce mode on
              // remonte souvent de quelques écrans, et les reconstruire ferait
              // clignoter des images déjà chargées.
              scrollCacheExtent: const ScrollCacheExtent.viewport(2),
              itemBuilder: (context, index) => AspectRatio(
                aspectRatio: _pageRatio(index),
                child: _PageImage(
                  comicId: widget.comicId,
                  index: index,
                  localPath: _localPathOf(index),
                ),
              ),
            ),
          ),
        );
      },
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

  /// Avance d'une planche dans le sens de la lecture.
  final VoidCallback onTapForward;
  final VoidCallback onTapBackward;

  /// Sens d'affichage, qui décide quel bord avance.
  final bool rightToLeft;

  /// Marge appliquée à l'image, pas aux gestes.
  final ReaderMargin margin;

  /// Signale l'entrée et la sortie du zoom à l'écran, qui en dépend pour
  /// verrouiller le défilement des pages.
  final ValueChanged<bool> onZoomChanged;

  const _Page({
    required this.comicId,
    required this.index,
    required this.localPath,
    required this.onTapCenter,
    required this.onTapForward,
    required this.onTapBackward,
    required this.rightToLeft,
    required this.margin,
    required this.onZoomChanged,
  });

  @override
  ConsumerState<_Page> createState() => _PageState();
}

class _PageState extends ConsumerState<_Page> {
  final _transform = TransformationController();

  /// Dernier état transmis, pour ne pas reconstruire l'écran à chaque image
  /// d'un pincement — le contrôleur notifie en continu.
  bool _zoomed = false;

  @override
  void initState() {
    super.initState();
    _transform.addListener(_onTransform);
  }

  @override
  void dispose() {
    _transform.removeListener(_onTransform);
    _transform.dispose();
    super.dispose();
  }

  void _onTransform() {
    // Un seuil plutôt qu'une égalité : un pincement laisse des échelles comme
    // 1.0000001, et comparer des flottants au repos ferait clignoter le
    // verrouillage.
    final zoomed = _transform.value.getMaxScaleOnAxis() > 1.01;
    if (zoomed == _zoomed) return;

    _zoomed = zoomed;
    widget.onZoomChanged(zoomed);
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

  /// Route le toucher vers l'habillage ou vers un changement de page.
  ///
  /// Agrandi, les bords ne tournent plus : on examine un détail, et un toucher
  /// malheureux qui change de planche ferait perdre l'endroit.
  void _onTapUp(TapUpDetails details) {
    final zone = readerTapZone(
      x: details.localPosition.dx,
      width: MediaQuery.of(context).size.width,
      rightToLeft: widget.rightToLeft,
    );

    switch (zone) {
      case ReaderTapZone.chrome:
        widget.onTapCenter();
      case ReaderTapZone.forward:
        if (!_zoomed) widget.onTapForward();
      case ReaderTapZone.backward:
        if (!_zoomed) widget.onTapBackward();
    }
  }

  @override
  Widget build(BuildContext context) {
    final inset = MediaQuery.of(context).size.width * widget.margin.fraction;

    return GestureDetector(
      onTapUp: _onTapUp,
      onDoubleTapDown: _onDoubleTap,
      onDoubleTap: () {},
      child: InteractiveViewer(
        transformationController: _transform,
        minScale: 1,
        maxScale: 5,
        // Le déplacement doit rester possible dès qu'on est agrandi, y compris
        // vers les bords de la planche, qui est précisément ce qu'on cherchait
        // à atteindre quand le PageView interceptait le geste.
        panEnabled: true,
        child: Padding(
          // La marge est posée DANS la vue interactive, donc sur l'image seule.
          // Au-dessus, elle rétrécirait aussi la surface des gestes, et le bord
          // qui tourne la page s'éloignerait du pouce — l'inverse du but.
          padding: EdgeInsets.symmetric(horizontal: inset),
          child: _PageImage(
            comicId: widget.comicId,
            index: widget.index,
            localPath: widget.localPath,
          ),
        ),
      ),
    );
  }
}

/// Une planche, sans geste ni cadre : les deux modes de lecture la partagent.
class _PageImage extends ConsumerWidget {
  final String comicId;
  final int index;
  final String? localPath;

  const _PageImage({
    required this.comicId,
    required this.index,
    required this.localPath,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final local = localPath;
    if (local != null) {
      // Sans fondu : le fichier est déjà là, et une transition simulerait un
      // chargement qui n'a pas lieu.
      return Image.file(
        File(local),
        fit: BoxFit.contain,
        errorBuilder: (_, _, _) => const _BrokenPage(),
      );
    }

    final session = ref.watch(sessionProvider);
    if (session is! SessionActive) return const SizedBox.shrink();

    return CachedNetworkImage(
      imageUrl: session.client.imageUrl(
        '/api/v1/comics/$comicId/pages/$index',
        width: 1600,
      ),
      fit: BoxFit.contain,
      fadeInDuration: const Duration(milliseconds: 120),
      placeholder: (_, _) => const Center(
        child: SizedBox(
          height: 28,
          width: 28,
          child: CircularProgressIndicator(
            strokeWidth: 2,
            color: Colors.white24,
          ),
        ),
      ),
      errorWidget: (_, _, _) => const _BrokenPage(),
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

// ─── Bande de vignettes ──────────────────────────────────────────────────────

/*
Sélecteur de pages en vignettes, comme sur le web.

Le curseur dit où l'on est ; il ne dit pas ce qu'il y a. Pour retrouver une
planche — celle avec le vaisseau, celle où le récit bascule — il faut la voir.
C'est la seule façon de naviguer dans un album qu'on relit, et le mobile en
était privé alors que le web l'avait.

La bande n'est montée que lorsqu'elle est ouverte : deux cents planches
déclencheraient sinon deux cents requêtes pour une bande que personne n'a
demandée.
*/
class _Filmstrip extends ConsumerStatefulWidget {
  final String comicId;
  final Manifest manifest;
  final int current;
  final String? offlineDirectory;
  final int localPages;
  final ValueChanged<int> onSelect;
  final VoidCallback onClose;

  const _Filmstrip({
    required this.comicId,
    required this.manifest,
    required this.current,
    required this.offlineDirectory,
    required this.localPages,
    required this.onSelect,
    required this.onClose,
  });

  @override
  ConsumerState<_Filmstrip> createState() => _FilmstripState();
}

class _FilmstripState extends ConsumerState<_Filmstrip> {
  /// Largeur demandée au serveur.
  ///
  /// C'est le plus petit palier qu'il sert : demander moins serait arrondi à
  /// la même image, et le demander explicitement garde une seule entrée de
  /// cache pour une seule vignette.
  static const _thumbWidth = 320;

  static const _itemWidth = 64.0;
  static const _gap = 8.0;
  static const _stripHeight = 132.0;

  late final ScrollController _scroll;

  @override
  void initState() {
    super.initState();
    _scroll = ScrollController();

    // La vignette courante doit être sous les yeux à l'ouverture. La largeur
    // de l'écran n'est pas connue ici : le centrage attend la première image.
    WidgetsBinding.instance.addPostFrameCallback((_) => _centerOnCurrent());
  }

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  void _centerOnCurrent() {
    if (!_scroll.hasClients || !mounted) return;

    final viewport = _scroll.position.viewportDimension;
    final target =
        widget.current * (_itemWidth + _gap) -
        (viewport / 2) +
        (_itemWidth / 2);

    _scroll.jumpTo(target.clamp(0, _scroll.position.maxScrollExtent));
  }

  /// Proportions de la planche, quand le manifeste les donne.
  ///
  /// Une lecture hors ligne n'a pas de manifeste détaillé : la valeur de repli
  /// est celle d'une planche ordinaire, et une vignette légèrement rognée vaut
  /// mieux qu'une bande dont la hauteur saute d'un élément à l'autre.
  double _ratio(int index) {
    final pages = widget.manifest.pages;
    if (index >= pages.length) return 0.7;

    final page = pages[index];
    final width = page.width;
    final height = page.height;
    if (width == null || height == null || height == 0) return 0.7;

    return width / height;
  }

  Widget _thumbnail(int index) {
    final directory = widget.offlineDirectory;
    if (index < widget.localPages && directory != null) {
      return Image.file(
        File('$directory/$index'),
        fit: BoxFit.cover,
        // Décoder deux cents planches en pleine définition pour en montrer
        // soixante pixels de large épuiserait la mémoire.
        cacheWidth: _thumbWidth,
        errorBuilder: (_, _, _) => const ColoredBox(color: Colors.white10),
      );
    }

    final session = ref.read(sessionProvider);
    if (session is! SessionActive) {
      return const ColoredBox(color: Colors.white10);
    }

    return CachedNetworkImage(
      imageUrl: session.client.imageUrl(
        '/api/v1/comics/${widget.comicId}/pages/$index',
        width: _thumbWidth,
      ),
      fit: BoxFit.cover,
      fadeInDuration: const Duration(milliseconds: 120),
      placeholder: (_, _) => const ColoredBox(color: Colors.white10),
      errorWidget: (_, _, _) => const ColoredBox(color: Colors.white10),
    );
  }

  @override
  Widget build(BuildContext context) {
    final count = widget.manifest.pageCount;

    return Container(
      color: Colors.black.withValues(alpha: 0.92),
      padding: EdgeInsets.only(
        bottom: MediaQuery.of(context).padding.bottom + BoxSpace.s2,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(
              BoxSpace.s4,
              BoxSpace.s2,
              BoxSpace.s2,
              BoxSpace.s1,
            ),
            child: Row(
              children: [
                Text(
                  '$count pages',
                  style: const TextStyle(color: Colors.white38, fontSize: 11),
                ),
                const Spacer(),
                IconButton(
                  tooltip: 'Fermer les vignettes',
                  visualDensity: VisualDensity.compact,
                  icon: const Icon(
                    Icons.close,
                    color: Colors.white54,
                    size: 20,
                  ),
                  onPressed: widget.onClose,
                ),
              ],
            ),
          ),
          SizedBox(
            height: _stripHeight,
            child: ListView.separated(
              controller: _scroll,
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: BoxSpace.s4),
              itemCount: count,
              separatorBuilder: (_, _) => const SizedBox(width: _gap),
              itemBuilder: (context, index) {
                final active = index == widget.current;

                return GestureDetector(
                  onTap: () => widget.onSelect(index),
                  child: SizedBox(
                    width: _itemWidth,
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Expanded(
                          child: DecoratedBox(
                            decoration: BoxDecoration(
                              border: Border.all(
                                // Le lecteur est noir quel que soit le thème
                                // du système : la teinte sombre est la seule
                                // qui s'y lise.
                                color: active
                                    ? boxColorsDark.accent
                                    : Colors.transparent,
                                width: 2,
                              ),
                              borderRadius: BorderRadius.circular(3),
                            ),
                            child: ClipRRect(
                              borderRadius: BorderRadius.circular(2),
                              child: Opacity(
                                opacity: active ? 1 : 0.55,
                                child: AspectRatio(
                                  aspectRatio: _ratio(index),
                                  child: _thumbnail(index),
                                ),
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          '${index + 1}',
                          style: TextStyle(
                            color: active ? Colors.white : Colors.white38,
                            fontSize: 10,
                            fontFeatures: const [FontFeature.tabularFigures()],
                          ),
                        ),
                      ],
                    ),
                  ),
                );
              },
            ),
          ),
        ],
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
  final bool filmstripOpen;
  final VoidCallback onToggleFilmstrip;
  final bool settingsOpen;
  final VoidCallback onToggleSettings;

  /// Un panneau occupe-t-il le bas de l'écran, sous cette barre ?
  ///
  /// Le cas échéant, c'est lui qui écarte la zone système ; les deux le
  /// faisant, la barre flotterait au-dessus d'un vide.
  final bool panelBelow;

  const _BottomBar({
    required this.page,
    required this.pageCount,
    required this.onSeek,
    required this.filmstripOpen,
    required this.onToggleFilmstrip,
    required this.settingsOpen,
    required this.onToggleSettings,
    required this.panelBelow,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.only(
        top: BoxSpace.s8,
        left: BoxSpace.s4,
        right: BoxSpace.s3,
        bottom: panelBelow
            ? BoxSpace.s2
            : MediaQuery.of(context).padding.bottom + BoxSpace.s2,
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
            tooltip: 'Pages en vignettes',
            visualDensity: VisualDensity.compact,
            icon: Icon(
              Icons.grid_view_rounded,
              color: filmstripOpen ? boxColorsDark.accent : Colors.white70,
            ),
            onPressed: onToggleFilmstrip,
          ),
          // Le sens de lecture était ici, en bascule directe. Il rejoint les
          // réglages : on le choisit une fois et il est retenu, alors que le
          // mode et la marge s'essaient à plusieurs reprises pour trouver ce
          // qui convient — les réunir donne un endroit où comparer.
          IconButton(
            tooltip: 'Réglages de lecture',
            visualDensity: VisualDensity.compact,
            icon: Icon(
              Icons.tune_rounded,
              color: settingsOpen ? boxColorsDark.accent : Colors.white70,
            ),
            onPressed: onToggleSettings,
          ),
        ],
      ),
    );
  }
}

// ─── Réglages de lecture ─────────────────────────────────────────────────────

/*
Panneau de réglages, au même endroit que sur le web.

Trois choix seulement, et c'est délibéré : le mode, le sens et la marge. Un
lecteur se règle une fois et s'oublie ; une liste d'options le transformerait
en tableau de bord.
*/
class _SettingsSheet extends ConsumerWidget {
  final ReaderSettings settings;
  final VoidCallback onClose;

  const _SettingsSheet({required this.settings, required this.onClose});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final notifier = ref.read(readerSettingsProvider.notifier);
    final webtoon = settings.mode == ReaderMode.webtoon;

    return Container(
      color: Colors.black.withValues(alpha: 0.92),
      padding: EdgeInsets.only(
        left: BoxSpace.s4,
        right: BoxSpace.s2,
        top: BoxSpace.s2,
        bottom: MediaQuery.of(context).padding.bottom + BoxSpace.s4,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Text(
                'RÉGLAGES DE LECTURE',
                style: TextStyle(
                  color: Colors.white38,
                  fontSize: 10,
                  letterSpacing: 0.8,
                ),
              ),
              const Spacer(),
              IconButton(
                tooltip: 'Fermer les réglages',
                visualDensity: VisualDensity.compact,
                icon: const Icon(Icons.close, color: Colors.white54, size: 20),
                onPressed: onClose,
              ),
            ],
          ),

          _SettingRow(
            label: 'Mode',
            options: [
              _Option(
                label: 'Planche par planche',
                active: !webtoon,
                onTap: () => notifier.setMode(ReaderMode.paged),
              ),
              _Option(
                label: 'Défilement continu',
                active: webtoon,
                onTap: () => notifier.setMode(ReaderMode.webtoon),
              ),
            ],
          ),

          // Le sens n'a pas de sens en défilement continu : la lecture y est
          // verticale. Le montrer quand même ferait croire à un réglage sans
          // effet, et l'y appliquer n'aurait rien à inverser.
          if (!webtoon)
            _SettingRow(
              label: 'Sens de lecture',
              options: [
                _Option(
                  label: 'Gauche → droite',
                  active: settings.direction == ReadingDirection.leftToRight,
                  onTap: () =>
                      notifier.setDirection(ReadingDirection.leftToRight),
                ),
                _Option(
                  label: 'Droite → gauche',
                  active: settings.direction == ReadingDirection.rightToLeft,
                  onTap: () =>
                      notifier.setDirection(ReadingDirection.rightToLeft),
                ),
              ],
            ),

          _SettingRow(
            label: 'Marge',
            hint: 'Rapproche la planche du pouce sur un grand écran.',
            options: [
              for (final margin in ReaderMargin.values)
                _Option(
                  label: _marginLabel(margin),
                  active: settings.margin == margin,
                  onTap: () => notifier.setMargin(margin),
                ),
            ],
          ),
        ],
      ),
    );
  }

  static String _marginLabel(ReaderMargin margin) => switch (margin) {
    ReaderMargin.none => 'Aucune',
    ReaderMargin.small => 'Petite',
    ReaderMargin.medium => 'Moyenne',
    ReaderMargin.large => 'Large',
  };
}

class _SettingRow extends StatelessWidget {
  final String label;
  final String? hint;
  final List<_Option> options;

  const _SettingRow({required this.label, this.hint, required this.options});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: BoxSpace.s3, right: BoxSpace.s2),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: const TextStyle(color: Colors.white54, fontSize: 12),
          ),
          if (hint != null)
            Padding(
              padding: const EdgeInsets.only(top: 1),
              child: Text(
                hint!,
                style: const TextStyle(color: Colors.white24, fontSize: 10),
              ),
            ),
          const SizedBox(height: BoxSpace.s1),
          Wrap(
            spacing: BoxSpace.s1,
            runSpacing: BoxSpace.s1,
            children: options,
          ),
        ],
      ),
    );
  }
}

class _Option extends StatelessWidget {
  final String label;
  final bool active;
  final VoidCallback onTap;

  const _Option({
    required this.label,
    required this.active,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(
          horizontal: BoxSpace.s3,
          vertical: BoxSpace.s2,
        ),
        decoration: BoxDecoration(
          color: active ? boxColorsDark.accent : Colors.white10,
          borderRadius: BorderRadius.circular(6),
        ),
        child: Text(
          label,
          style: TextStyle(
            color: active ? boxColorsDark.accentText : Colors.white70,
            fontSize: 12,
            fontWeight: active ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ),
    );
  }
}
