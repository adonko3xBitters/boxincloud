import 'package:flutter/material.dart';

import 'tokens.dart';

/// Thème de l'application, dérivé des tokens partagés avec le web.
///
/// Aucune couleur n'est écrite ici : elles viennent toutes de
/// `packages/design-tokens/tokens.json`, le même fichier que les variables CSS.
/// C'est ce qui empêche les deux clients de diverger visuellement en quelques
/// mois — alors que l'ergonomie est le différenciateur revendiqué du projet.
ThemeData boxTheme(Brightness brightness) {
  final colors = brightness == Brightness.dark ? boxColorsDark : boxColorsLight;

  final scheme = ColorScheme(
    brightness: brightness,
    primary: colors.accent,
    onPrimary: colors.textInverted,
    secondary: colors.accentSubtle,
    onSecondary: colors.accentText,
    error: colors.danger,
    onError: Colors.white,
    surface: colors.surface,
    onSurface: colors.text,
  );

  return ThemeData(
    useMaterial3: true,
    brightness: brightness,
    colorScheme: scheme,
    scaffoldBackgroundColor: colors.background,

    // La densité par défaut de Material laisse beaucoup d'air. Une
    // bibliothèque se parcourt d'un coup d'œil : on la resserre, comme sur le
    // web.
    visualDensity: VisualDensity.compact,

    appBarTheme: AppBarTheme(
      backgroundColor: colors.surface,
      foregroundColor: colors.text,
      elevation: 0,
      scrolledUnderElevation: 1,
      surfaceTintColor: Colors.transparent,
    ),
    cardTheme: CardThemeData(
      color: colors.surface,
      elevation: 0,
      margin: EdgeInsets.zero,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(BoxRadius.lg),
        side: BorderSide(color: colors.border),
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      filled: true,
      fillColor: colors.surface,
      border: OutlineInputBorder(
        borderRadius: BorderRadius.circular(BoxRadius.md),
        borderSide: BorderSide(color: colors.border),
      ),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(BoxRadius.md),
        borderSide: BorderSide(color: colors.border),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(BoxRadius.md),
        borderSide: BorderSide(color: colors.accent, width: 1.5),
      ),
      contentPadding: const EdgeInsets.symmetric(
        horizontal: BoxSpace.s3,
        vertical: BoxSpace.s3,
      ),
    ),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        backgroundColor: colors.accent,
        foregroundColor: colors.textInverted,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(BoxRadius.md),
        ),
        padding: const EdgeInsets.symmetric(
          horizontal: BoxSpace.s4,
          vertical: BoxSpace.s3,
        ),
      ),
    ),
    dividerTheme: DividerThemeData(color: colors.border, space: 1, thickness: 1),
    listTileTheme: ListTileThemeData(
      textColor: colors.text,
      iconColor: colors.textMuted,
    ),
    snackBarTheme: SnackBarThemeData(
      backgroundColor: colors.surfaceRaised,
      contentTextStyle: TextStyle(color: colors.text),
      behavior: SnackBarBehavior.floating,
    ),
  );
}

/// Accès aux rôles de couleur depuis un widget.
///
/// `Theme.of(context).colorScheme` ne porte qu'une poignée de rôles Material ;
/// les nôtres sont plus nombreux et plus précis — surfaces enfoncées, texte
/// atténué, bordure forte.
extension BoxColorsOf on BuildContext {
  BoxColors get colors =>
      Theme.of(this).brightness == Brightness.dark ? boxColorsDark : boxColorsLight;
}
