import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/auth/session.dart';
import '../../core/search/local_search.dart';
import '../../shared/theme.dart';
import '../../shared/tokens.dart';
import '../search/search_controller.dart';
import 'series_screen.dart';

/// Toutes les séries.
///
/// Une bibliothèque de BD se pense par séries bien plus que par albums : on
/// cherche « Blacksad », pas « Arctic-Nation ». La grille de couvertures de la
/// bibliothèque montre pourtant des albums, un par tome — trente-huit vignettes
/// d'Astérix côte à côte. Cet écran donne l'autre entrée.
class SeriesListScreen extends ConsumerWidget {
  const SeriesListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = context.colors;
    final series = ref.watch(seriesListProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Séries')),
      body: series.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(BoxSpace.s8),
            child: Text('$error', textAlign: TextAlign.center),
          ),
        ),
        data: (list) {
          if (list.isEmpty) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(BoxSpace.s8),
                child: Text(
                  'Aucune série. Le serveur en détecte à partir des noms de '
                  'fichiers ; les albums isolés n\'en ont pas.',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: colors.textMuted),
                ),
              ),
            );
          }

          // Tri sur la forme repliée : sans quoi « Élève » passerait après
          // « Zorro », les lettres accentuées se classant après le Z en ASCII.
          final sorted = [...list]..sort((a, b) => fold(a.name).compareTo(fold(b.name)));

          return ListView.separated(
            itemCount: sorted.length,
            separatorBuilder: (_, _) => Divider(height: 1, color: colors.border),
            itemBuilder: (context, index) => _SeriesRow(series: sorted[index]),
          );
        },
      ),
    );
  }
}

class _SeriesRow extends ConsumerWidget {
  final SeriesHit series;

  const _SeriesRow({required this.series});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    final colors = context.colors;

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(
        horizontal: BoxSpace.s4,
        vertical: BoxSpace.s2,
      ),
      leading: SizedBox(
        width: 40,
        height: 56,
        child: ClipRRect(
          borderRadius: BorderRadius.circular(BoxRadius.sm),
          child: session is SessionActive && series.coverPath.isNotEmpty
              ? CachedNetworkImage(
                  imageUrl: session.client.imageUrl(series.coverPath, width: 160),
                  fit: BoxFit.cover,
                  placeholder: (_, _) => ColoredBox(color: colors.surfaceSunken),
                  errorWidget: (_, _, _) => ColoredBox(color: colors.surfaceSunken),
                )
              : ColoredBox(color: colors.surfaceSunken),
        ),
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
