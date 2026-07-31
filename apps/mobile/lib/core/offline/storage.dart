import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:path_provider/path_provider.dart';

/*
Où vivent les pages téléchargées.

Un fichier par page, pas l'archive d'origine. Ce n'est pas un détail de mise en
œuvre mais le choix qui rend le hors ligne possible : l'application ne sait
décompresser ni un RAR ni un PDF, et embarquer les deux décodeurs pour lire un
album dans un train serait payer très cher une conversion que le serveur fait
déjà mieux.

Le corollaire est heureux. Le serveur sert des pages redimensionnées à la
largeur demandée : un album de soixante mégaoctets de planches scannées en pèse
une quinzaine à la définition d'un téléphone. Sur un appareil où l'espace se
compte, le rapport est décisif.

Chaque page est écrite sous un nom temporaire puis renommée. Le renommage est
atomique sur les systèmes de fichiers visés : une coupure de réseau, une
batterie vide ou une application tuée ne peuvent donc pas laisser derrière elles
un fichier à moitié écrit qu'on prendrait ensuite pour une page valide.
*/

/// Racine des téléchargements.
///
/// Dans le répertoire de support de l'application, pas dans les documents :
/// c'est un cache reconstituable, qui n'a rien à faire dans une sauvegarde
/// d'appareil ni sous les yeux de l'utilisateur dans un explorateur de fichiers.
Future<Directory> offlineRoot() async {
  final base = await getApplicationSupportDirectory();
  return Directory(p.join(base.path, 'offline'));
}

Future<Directory> comicDirectory(String serverId, String comicId) async {
  final root = await offlineRoot();
  return Directory(p.join(root.path, serverId, comicId));
}

/// Chemin d'une page téléchargée. N'en garantit pas l'existence.
Future<String> pagePath(String serverId, String comicId, int index) async {
  final dir = await comicDirectory(serverId, comicId);
  return p.join(dir.path, '$index');
}

/// Écrit une page, de façon à ce qu'elle n'existe qu'une fois complète.
Future<int> writePage(
  String serverId,
  String comicId,
  int index,
  List<int> bytes,
) async {
  final dir = await comicDirectory(serverId, comicId);
  await dir.create(recursive: true);

  final target = p.join(dir.path, '$index');
  final temporary = File('$target.part');

  await temporary.writeAsBytes(bytes, flush: true);
  await temporary.rename(target);

  return bytes.length;
}

/// Efface les pages d'un album.
Future<void> deleteComicFiles(String serverId, String comicId) async {
  final dir = await comicDirectory(serverId, comicId);
  if (await dir.exists()) await dir.delete(recursive: true);
}

/// Efface tout ce qu'un serveur a laissé sur le disque.
Future<void> deleteServerFiles(String serverId) async {
  final root = await offlineRoot();
  final dir = Directory(p.join(root.path, serverId));
  if (await dir.exists()) await dir.delete(recursive: true);
}

/// Octets réellement occupés sur le disque.
///
/// La base tient un compte, qui suffit à l'affichage courant. Celui-ci sert à
/// le vérifier : les deux divergent si une écriture a échoué à mi-chemin ou si
/// le système a fait le ménage sous l'application.
Future<int> measureDiskUsage() async {
  final root = await offlineRoot();
  if (!await root.exists()) return 0;

  var total = 0;
  await for (final entity in root.list(recursive: true, followLinks: false)) {
    if (entity is File) total += await entity.length();
  }
  return total;
}
