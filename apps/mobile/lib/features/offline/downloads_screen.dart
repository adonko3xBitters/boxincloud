import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../core/offline/download_manager.dart';
import '../../shared/theme.dart';
import '../../shared/tokens.dart';
import 'downloads_controller.dart';

/// Gestion des téléchargements.
///
/// Deux questions, dans cet ordre : combien de place cela prend-il, et
/// qu'est-ce que je peux lire dans l'avion ? Le reste — vitesse, file d'attente
/// — n'intéresse que le temps du téléchargement.
class DownloadsScreen extends ConsumerWidget {
  const DownloadsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = context.colors;
    final downloads = ref.watch(downloadsProvider);

    return Scaffold(
      appBar: AppBar(title: const Text('Téléchargements')),
      body: Column(
        children: [
          const _DiskUsage(),
          Divider(height: 1, color: colors.border),
          Expanded(
            child: downloads.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (error, _) => Center(child: Text('$error')),
              data: (list) {
                if (list.isEmpty) return const _Empty();

                return ListView.separated(
                  itemCount: list.length,
                  separatorBuilder: (_, _) =>
                      Divider(height: 1, color: colors.border),
                  itemBuilder: (context, index) => _Row(download: list[index]),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

/// Occupation et budget.
class _DiskUsage extends ConsumerWidget {
  const _DiskUsage();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = context.colors;
    final usage = ref.watch(diskUsageProvider).valueOrNull;

    final used = usage?.used ?? 0;
    final budget = usage?.budget ?? defaultBudgetBytes;
    final ratio = budget > 0 ? (used / budget).clamp(0.0, 1.0) : 0.0;

    return Padding(
      padding: const EdgeInsets.all(BoxSpace.s4),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  '${formatBytes(used)} sur ${formatBytes(budget)}',
                  style: TextStyle(color: colors.text, fontWeight: FontWeight.w600),
                ),
              ),
              TextButton(
                onPressed: () => _editBudget(context, ref, budget),
                child: const Text('Modifier'),
              ),
            ],
          ),
          const SizedBox(height: BoxSpace.s2),
          ClipRRect(
            borderRadius: BorderRadius.circular(BoxRadius.full),
            child: LinearProgressIndicator(
              value: ratio,
              minHeight: 6,
              backgroundColor: colors.border,
              // Rouge quand on approche : le budget atteint interrompt un
              // téléchargement, ce qui se comprend mieux annoncé qu'après coup.
              color: ratio > 0.9 ? colors.danger : null,
            ),
          ),
          const SizedBox(height: BoxSpace.s2),
          Text(
            'Au-delà du budget, les albums lus sont effacés en premier, puis les '
            'plus anciennement ouverts.',
            style: TextStyle(color: colors.textSubtle, fontSize: 12, height: 1.35),
          ),
        ],
      ),
    );
  }

  Future<void> _editBudget(BuildContext context, WidgetRef ref, int current) async {
    final manager = ref.read(downloadManagerProvider);
    if (manager == null) return;

    // Des paliers plutôt qu'un champ libre : personne ne veut saisir un nombre
    // d'octets, et les valeurs utiles tiennent sur une main.
    const choices = [1, 2, 4, 8, 16, 32];

    final chosen = await showModalBottomSheet<int>(
      context: context,
      showDragHandle: true,
      builder: (context) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            for (final gigabytes in choices)
              ListTile(
                leading: Icon(
                  gigabytes * 1024 * 1024 * 1024 == current
                      ? Icons.radio_button_checked
                      : Icons.radio_button_unchecked,
                ),
                title: Text('$gigabytes Go'),
                onTap: () =>
                    Navigator.pop(context, gigabytes * 1024 * 1024 * 1024),
              ),
          ],
        ),
      ),
    );

    if (chosen == null) return;
    await manager.setBudgetBytes(chosen);
    ref.invalidate(diskUsageProvider);
  }
}

class _Row extends ConsumerWidget {
  final Download download;

  const _Row({required this.download});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final colors = context.colors;
    final session = ref.watch(sessionProvider);
    final manager = ref.watch(downloadManagerProvider);

    final ratio = download.pageCount > 0
        ? download.pagesDone / download.pageCount
        : 0.0;

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
          child: session is SessionActive && download.coverPath.isNotEmpty
              ? CachedNetworkImage(
                  imageUrl: session.client.imageUrl(download.coverPath, width: 160),
                  fit: BoxFit.cover,
                  placeholder: (_, _) => ColoredBox(color: colors.surfaceSunken),
                  errorWidget: (_, _, _) => ColoredBox(color: colors.surfaceSunken),
                )
              : ColoredBox(color: colors.surfaceSunken),
        ),
      ),
      title: Text(download.title, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(height: 2),
          Text(
            _describe(download),
            maxLines: 2,
            style: TextStyle(
              color: download.state == 'failed' ? colors.danger : colors.textSubtle,
              fontSize: 12,
            ),
          ),
          if (download.state != 'complete' && download.state != 'failed') ...[
            const SizedBox(height: 4),
            ClipRRect(
              borderRadius: BorderRadius.circular(BoxRadius.full),
              child: LinearProgressIndicator(
                value: ratio,
                minHeight: 3,
                backgroundColor: colors.border,
              ),
            ),
          ],
        ],
      ),
      trailing: IconButton(
        tooltip: 'Supprimer',
        icon: const Icon(Icons.delete_outline),
        onPressed: manager == null ? null : () => manager.remove(download.comicId),
      ),
    );
  }

  String _describe(Download download) {
    switch (download.state) {
      case 'complete':
        final series =
            download.seriesName.isNotEmpty ? '${download.seriesName} · ' : '';
        return '$series${download.pageCount} pages · ${formatBytes(download.bytes)}';
      case 'failed':
        return download.error ?? 'Échec du téléchargement.';
      case 'paused':
        return 'Interrompu à ${download.pagesDone}/${download.pageCount}';
      case 'queued':
        return 'En attente · ${download.pageCount} pages';
      default:
        return '${download.pagesDone}/${download.pageCount} pages · ${formatBytes(download.bytes)}';
    }
  }
}

class _Empty extends StatelessWidget {
  const _Empty();

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(BoxSpace.s8),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.download_outlined, size: 40, color: colors.textSubtle),
            const SizedBox(height: BoxSpace.s3),
            Text(
              'Aucun album téléchargé',
              style: TextStyle(
                color: colors.text,
                fontSize: 16,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: BoxSpace.s1),
            Text(
              'Depuis la fiche d\'un album, « Télécharger » le rend lisible sans '
              'réseau. Le téléchargement s\'interrompt si vous quittez '
              'l\'application, et reprend là où il s\'était arrêté.',
              textAlign: TextAlign.center,
              style: TextStyle(color: colors.textMuted, height: 1.4),
            ),
          ],
        ),
      ),
    );
  }
}
