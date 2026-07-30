import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/auth/session.dart';
import '../../core/db/database.dart';
import '../../shared/theme.dart';
import '../../shared/tokens.dart';
import '../reader/reader_screen.dart';
import 'library_controller.dart';

/// Bibliothèque : la grille de couvertures.
///
/// Une couverture de BD est souvent la seule chose par laquelle on reconnaît un
/// album. La grille leur laisse donc la place, et le reste — titre, série — tient
/// sur deux lignes en dessous.
class LibraryScreen extends ConsumerWidget {
  const LibraryScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    final scope = ref.watch(scopeProvider);
    final library = ref.watch(libraryProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(scope.title),
        actions: [
          IconButton(
            icon: const Icon(Icons.folder_outlined),
            tooltip: 'Dossiers',
            onPressed: () => _openFolders(context, ref),
          ),
          IconButton(
            icon: const Icon(Icons.account_circle_outlined),
            tooltip: 'Compte',
            onPressed: () => _openAccount(context, ref, session),
          ),
        ],
      ),
      body: library.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => _Message(
          icon: Icons.error_outline,
          title: 'Impossible de charger la bibliothèque',
          detail: '$error',
        ),
        data: (view) {
          if (view.comics.isEmpty) {
            return _Message(
              icon: Icons.menu_book_outlined,
              title: 'Aucun album ici',
              detail: view.offline
                  ? 'Vous êtes hors ligne, et rien n\'a encore été mis en cache.'
                  : 'Déposez des albums depuis l\'interface web pour les retrouver ici.',
            );
          }

          return Column(
            children: [
              if (view.offline) const _OfflineBanner(),
              Expanded(
                child: RefreshIndicator(
                  onRefresh: () async => ref.invalidate(libraryProvider),
                  child: _CoverGrid(comics: view.comics),
                ),
              ),
            ],
          );
        },
      ),
    );
  }

  void _openFolders(BuildContext context, WidgetRef ref) {
    final view = ref.read(libraryProvider).valueOrNull;
    if (view == null) return;

    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (context) => _FolderSheet(folders: view.folders),
    );
  }

  void _openAccount(BuildContext context, WidgetRef ref, SessionState session) {
    if (session is! SessionActive) return;

    showModalBottomSheet<void>(
      context: context,
      showDragHandle: true,
      builder: (sheetContext) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.person_outline),
              title: Text(session.user.displayName ?? session.user.username),
              subtitle: Text(session.server.label),
            ),
            const Divider(),
            for (final server in session.servers)
              if (server.id != session.server.id)
                ListTile(
                  leading: const Icon(Icons.swap_horiz),
                  title: Text(server.label),
                  subtitle: Text(server.username),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    ref.read(sessionProvider.notifier).switchTo(server.id);
                  },
                ),
            ListTile(
              leading: const Icon(Icons.logout),
              title: const Text('Se déconnecter'),
              onTap: () {
                Navigator.pop(sheetContext);
                ref.read(sessionProvider.notifier).signOut();
              },
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Grille ──────────────────────────────────────────────────────────────────

class _CoverGrid extends ConsumerWidget {
  final List<CachedComic> comics;

  const _CoverGrid({required this.comics});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final session = ref.watch(sessionProvider);
    if (session is! SessionActive) return const SizedBox.shrink();

    return GridView.builder(
      padding: const EdgeInsets.all(BoxSpace.s3),
      // Une largeur maximale plutôt qu'un nombre de colonnes fixe : la même
      // grille tient sur un téléphone et sur une tablette sans code de rupture.
      gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
        maxCrossAxisExtent: 180,
        childAspectRatio: 0.58,
        crossAxisSpacing: BoxSpace.s3,
        mainAxisSpacing: BoxSpace.s4,
      ),
      itemCount: comics.length,
      itemBuilder: (context, index) {
        final comic = comics[index];

        return _CoverTile(
          comic: comic,
          imageUrl: session.client.imageUrl(comic.coverPath, width: 320),
          onTap: () => Navigator.of(context).push(
            MaterialPageRoute<void>(
              builder: (_) => ReaderScreen(comicId: comic.id, title: comic.title),
            ),
          ),
        );
      },
    );
  }
}

class _CoverTile extends StatelessWidget {
  final CachedComic comic;
  final String imageUrl;
  final VoidCallback onTap;

  const _CoverTile({
    required this.comic,
    required this.imageUrl,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(BoxRadius.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: ClipRRect(
              borderRadius: BorderRadius.circular(BoxRadius.sm),
              child: CachedNetworkImage(
                imageUrl: imageUrl,
                fit: BoxFit.cover,
                width: double.infinity,
                // Le fond tient la place avant l'arrivée de l'image : sans lui
                // la grille sautille au fur et à mesure des chargements.
                placeholder: (_, _) => ColoredBox(color: colors.surfaceSunken),
                errorWidget: (_, _, _) => ColoredBox(
                  color: colors.surfaceSunken,
                  child: Icon(Icons.image_not_supported_outlined,
                      color: colors.textSubtle),
                ),
              ),
            ),
          ),
          const SizedBox(height: BoxSpace.s2),
          Text(
            comic.title,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              color: colors.text,
              fontWeight: FontWeight.w500,
              fontSize: 14,
            ),
          ),
          Text(
            comic.seriesName.isNotEmpty
                ? '${comic.seriesName}${comic.number.isNotEmpty ? ' · ${comic.number}' : ''}'
                : '${comic.pageCount} p.',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(color: colors.textSubtle, fontSize: 12),
          ),
        ],
      ),
    );
  }
}

// ─── Dossiers ────────────────────────────────────────────────────────────────

class _FolderSheet extends ConsumerWidget {
  final List<CachedFolder> folders;

  const _FolderSheet({required this.folders});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return SafeArea(
      child: ListView(
        shrinkWrap: true,
        children: [
          ListTile(
            leading: const Icon(Icons.apps),
            title: const Text('Tous les albums'),
            onTap: () {
              ref.read(scopeProvider.notifier).state = const LibraryScope();
              Navigator.pop(context);
            },
          ),
          const Divider(),
          for (final folder in folders)
            if (folder.path.isNotEmpty)
              ListTile(
                // L'indentation rend l'arborescence lisible sans widget d'arbre,
                // le serveur renvoyant déjà les nœuds parents avant leurs
                // enfants.
                contentPadding: EdgeInsets.only(
                  left: BoxSpace.s4 + folder.depth * BoxSpace.s4,
                  right: BoxSpace.s4,
                ),
                leading: const Icon(Icons.folder_outlined),
                title: Text(folder.name),
                trailing: Text('${folder.comicCount}'),
                onTap: () {
                  ref.read(scopeProvider.notifier).state = LibraryScope(
                    libraryId: folder.libraryId,
                    folderPath: folder.path,
                    title: folder.name,
                  );
                  Navigator.pop(context);
                },
              ),
        ],
      ),
    );
  }
}

// ─── Éléments ────────────────────────────────────────────────────────────────

/// Bandeau hors ligne.
///
/// Discret mais présent : quelqu'un qui voit une bibliothèque incomplète doit
/// savoir que c'est le réseau, pas une disparition.
class _OfflineBanner extends StatelessWidget {
  const _OfflineBanner();

  @override
  Widget build(BuildContext context) {
    final colors = context.colors;

    return Container(
      width: double.infinity,
      color: colors.surfaceSunken,
      padding: const EdgeInsets.symmetric(
        horizontal: BoxSpace.s4,
        vertical: BoxSpace.s2,
      ),
      child: Row(
        children: [
          Icon(Icons.cloud_off_outlined, size: 16, color: colors.textSubtle),
          const SizedBox(width: BoxSpace.s2),
          Expanded(
            child: Text(
              'Hors ligne — affichage du dernier état connu.',
              style: TextStyle(color: colors.textSubtle, fontSize: 12),
            ),
          ),
        ],
      ),
    );
  }
}

class _Message extends StatelessWidget {
  final IconData icon;
  final String title;
  final String detail;

  const _Message({required this.icon, required this.title, required this.detail});

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
            const SizedBox(height: BoxSpace.s4),
            Text(
              title,
              textAlign: TextAlign.center,
              style: TextStyle(
                color: colors.text,
                fontSize: 17,
                fontWeight: FontWeight.w600,
              ),
            ),
            const SizedBox(height: BoxSpace.s2),
            Text(
              detail,
              textAlign: TextAlign.center,
              style: TextStyle(color: colors.textMuted, height: 1.5),
            ),
          ],
        ),
      ),
    );
  }
}
