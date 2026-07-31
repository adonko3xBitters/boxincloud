import 'dart:async';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../shared/theme.dart';
import '../../shared/tokens.dart';
import '../library/comic_detail_screen.dart';
import '../library/series_screen.dart';
import 'search_controller.dart';

/// Recherche.
///
/// Un écran plein plutôt qu'une barre repliée dans la bibliothèque : sur
/// téléphone, le clavier mange la moitié de la hauteur, et disputer le reste à
/// une grille de couvertures ne laisserait voir que deux résultats.
class SearchScreen extends ConsumerStatefulWidget {
  const SearchScreen({super.key});

  @override
  ConsumerState<SearchScreen> createState() => _SearchScreenState();
}

class _SearchScreenState extends ConsumerState<SearchScreen> {
  final _controller = TextEditingController();
  final _focus = FocusNode();

  /// La requête effectivement soumise, en retard sur la frappe.
  String _query = '';
  Timer? _debounce;

  @override
  void initState() {
    super.initState();
    // Le clavier s'ouvre tout seul : on n'arrive sur cet écran que pour taper.
    WidgetsBinding.instance.addPostFrameCallback((_) => _focus.requestFocus());
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _controller.dispose();
    _focus.dispose();
    super.dispose();
  }

  /*
  Une requête par pause de frappe, pas une par caractère.

  « astérix » lancerait sept recherches, dont six dont personne ne lira le
  résultat — et sur un serveur familial derrière une connexion domestique, ces
  six-là retardent la septième.
  */
  void _onChanged(String value) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 250), () {
      if (mounted) setState(() => _query = value);
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;
    final results = ref.watch(searchProvider(_query));

    return Scaffold(
      appBar: AppBar(
        titleSpacing: 0,
        title: TextField(
          controller: _controller,
          focusNode: _focus,
          autocorrect: false,
          textInputAction: TextInputAction.search,
          onChanged: _onChanged,
          // Valider force la recherche sans attendre la temporisation : on tape
          // « entrée » précisément quand on ne veut plus attendre.
          onSubmitted: (value) {
            _debounce?.cancel();
            setState(() => _query = value);
          },
          decoration: InputDecoration(
            hintText: 'Titre, série…',
            border: InputBorder.none,
            hintStyle: TextStyle(color: colors.textSubtle),
          ),
        ),
        actions: [
          if (_controller.text.isNotEmpty)
            IconButton(
              icon: const Icon(Icons.close),
              tooltip: 'Effacer',
              onPressed: () {
                _debounce?.cancel();
                _controller.clear();
                setState(() => _query = '');
                _focus.requestFocus();
              },
            ),
        ],
      ),
      body: results.when(
        // Pas d'indicateur pendant la frappe : la liste précédente reste
        // affichée, ce qui donne une recherche continue plutôt qu'un
        // clignotement à chaque pause.
        loading: () => _query.isEmpty ? const _Hint() : const _Searching(),
        error: (error, _) => _Centered(
          icon: Icons.error_outline,
          title: 'La recherche a échoué',
          detail: '$error',
        ),
        data: (outcome) {
          if (_query.trim().length < 2) return const _Hint();

          if (outcome.isEmpty) {
            return _Centered(
              icon: Icons.search_off,
              title: 'Aucun résultat',
              detail: outcome.offline
                  ? 'Vous êtes hors ligne : seuls les albums déjà en cache ont été cherchés.'
                  : 'Rien ne correspond à « ${_query.trim()} ».',
            );
          }

          return ListView(
            children: [
              if (outcome.offline) const _OfflineNote(),
              if (outcome.series.isNotEmpty) ...[
                const _SectionTitle('Séries'),
                for (final series in outcome.series) _SeriesRow(series: series),
              ],
              if (outcome.comics.isNotEmpty) ...[
                const _SectionTitle('Albums'),
                for (final comic in outcome.comics) _ComicRow(comic: comic),
              ],
              const SizedBox(height: BoxSpace.s6),
            ],
          );
        },
      ),
    );
  }
}

// ─── Lignes ──────────────────────────────────────────────────────────────────

class _SeriesRow extends ConsumerWidget {
  final SeriesHit series;

  const _SeriesRow({required this.series});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    final colors = context.colors;

    return ListTile(
      leading: _Thumb(
        url: session is SessionActive && series.coverPath.isNotEmpty
            ? session.client.imageUrl(series.coverPath, width: 160)
            : null,
      ),
      title: Text(series.name, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Text(
        series.comicCount == 1 ? '1 album' : '${series.comicCount} albums',
        style: TextStyle(color: colors.textSubtle, fontSize: 12),
      ),
      trailing: Icon(Icons.chevron_right, color: colors.textSubtle),
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => SeriesScreen(seriesId: series.id, name: series.name),
        ),
      ),
    );
  }
}

class _ComicRow extends ConsumerWidget {
  final CachedComic comic;

  const _ComicRow({required this.comic});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    final colors = context.colors;

    // La série sert de sous-titre quand elle existe : deux tomes d'une même
    // série portent des titres qui ne se distinguent pas hors contexte.
    final subtitle = comic.seriesName.isNotEmpty
        ? (comic.number.isNotEmpty
            ? '${comic.seriesName} · T${comic.number}'
            : comic.seriesName)
        : '${comic.pageCount} p.';

    return ListTile(
      leading: _Thumb(
        url: session is SessionActive
            ? session.client.imageUrl(comic.coverPath, width: 160)
            : null,
      ),
      title: Text(comic.title, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Text(
        subtitle,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: colors.textSubtle, fontSize: 12),
      ),
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute<void>(builder: (_) => ComicDetailScreen(comic: comic)),
      ),
    );
  }
}

class _Thumb extends StatelessWidget {
  final String? url;

  const _Thumb({this.url});

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    return SizedBox(
      width: 40,
      height: 56,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(BoxRadius.sm),
        child: url == null
            ? ColoredBox(color: colors.surfaceSunken)
            : CachedNetworkImage(
                imageUrl: url!,
                fit: BoxFit.cover,
                placeholder: (_, _) => ColoredBox(color: colors.surfaceSunken),
                errorWidget: (_, _, _) => ColoredBox(color: colors.surfaceSunken),
              ),
      ),
    );
  }
}

// ─── États ───────────────────────────────────────────────────────────────────

class _SectionTitle extends StatelessWidget {
  final String label;

  const _SectionTitle(this.label);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(BoxSpace.s4, BoxSpace.s4, BoxSpace.s4, BoxSpace.s1),
      child: Text(
        label.toUpperCase(),
        style: TextStyle(
          color: context.colors.textSubtle,
          fontSize: 11,
          letterSpacing: 0.6,
          fontWeight: FontWeight.w600,
        ),
      ),
    );
  }
}

class _OfflineNote extends StatelessWidget {
  const _OfflineNote();

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(
        horizontal: BoxSpace.s4,
        vertical: BoxSpace.s2,
      ),
      color: colors.surfaceSunken,
      child: Text(
        'Hors ligne — recherche dans les albums en cache.',
        style: TextStyle(color: colors.textMuted, fontSize: 12),
      ),
    );
  }
}

class _Searching extends StatelessWidget {
  const _Searching();

  @override
  Widget build(BuildContext context) =>
      const Center(child: CircularProgressIndicator());
}

class _Hint extends StatelessWidget {
  const _Hint();

  @override
  Widget build(BuildContext context) => const _Centered(
        icon: Icons.search,
        title: 'Chercher un album',
        detail: 'Deux lettres suffisent. Les accents ne comptent pas.',
      );
}

class _Centered extends StatelessWidget {
  final IconData icon;
  final String title;
  final String detail;

  const _Centered({required this.icon, required this.title, required this.detail});

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(BoxSpace.s8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 40, color: colors.textSubtle),
            const SizedBox(height: BoxSpace.s3),
            Text(
              title,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: colors.text,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: BoxSpace.s1),
            Text(
              detail,
              textAlign: TextAlign.center,
              style: TextStyle(color: colors.textMuted),
            ),
          ],
        ),
      ),
    );
  }
}
