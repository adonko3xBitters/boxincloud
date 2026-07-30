// Généré depuis packages/design-tokens/tokens.json — ne pas éditer.
// Régénérer avec : make generate-tokens

import 'package:flutter/material.dart';

/// Rôles de couleur d'un thème.
///
/// Les deux instances ci-dessous sont dérivées du même fichier que les
/// variables CSS du web : les deux clients ne peuvent pas diverger.
class BoxColors {
  final Color background;
  final Color surface;
  final Color surfaceRaised;
  final Color surfaceSunken;
  final Color surfaceHover;
  final Color border;
  final Color borderStrong;
  final Color text;
  final Color textMuted;
  final Color textSubtle;
  final Color textInverted;
  final Color accent;
  final Color accentHover;
  final Color accentSubtle;
  final Color accentText;
  final Color success;
  final Color warning;
  final Color danger;

  const BoxColors({
    required this.background,
    required this.surface,
    required this.surfaceRaised,
    required this.surfaceSunken,
    required this.surfaceHover,
    required this.border,
    required this.borderStrong,
    required this.text,
    required this.textMuted,
    required this.textSubtle,
    required this.textInverted,
    required this.accent,
    required this.accentHover,
    required this.accentSubtle,
    required this.accentText,
    required this.success,
    required this.warning,
    required this.danger,
  });
}

/// Rôles du thème clair.
const boxColorsLight = BoxColors(
  background: Color(0xFFF8FAFC),
  surface: Color(0xFFFFFFFF),
  surfaceRaised: Color(0xFFFFFFFF),
  surfaceSunken: Color(0xFFF1F5F9),
  surfaceHover: Color(0xFFF1F5F9),
  border: Color(0xFFE2E8F0),
  borderStrong: Color(0xFFCBD5E1),
  text: Color(0xFF0F172A),
  textMuted: Color(0xFF64748B),
  textSubtle: Color(0xFF94A3B8),
  textInverted: Color(0xFFFFFFFF),
  accent: Color(0xFF4F46E5),
  accentHover: Color(0xFF4338CA),
  accentSubtle: Color(0xFFEEF2FF),
  accentText: Color(0xFF4338CA),
  success: Color(0xFF059669),
  warning: Color(0xFFD97706),
  danger: Color(0xFFDC2626),
);

/// Rôles du thème sombre.
const boxColorsDark = BoxColors(
  background: Color(0xFF020617),
  surface: Color(0xFF0F172A),
  surfaceRaised: Color(0xFF1E293B),
  surfaceSunken: Color(0xFF020617),
  surfaceHover: Color(0xFF1E293B),
  border: Color(0xFF1E293B),
  borderStrong: Color(0xFF334155),
  text: Color(0xFFF8FAFC),
  textMuted: Color(0xFF94A3B8),
  textSubtle: Color(0xFF64748B),
  textInverted: Color(0xFF020617),
  accent: Color(0xFF6366F1),
  accentHover: Color(0xFF818CF8),
  accentSubtle: Color(0xFF1E1B4B),
  accentText: Color(0xFFA5B4FC),
  success: Color(0xFF10B981),
  warning: Color(0xFFF59E0B),
  danger: Color(0xFFEF4444),
);

/// Échelle d'espacement, en pixels logiques.
class BoxSpace {
  static const double s0 = 0;
  static const double s1 = 4;
  static const double s2 = 8;
  static const double s3 = 12;
  static const double s4 = 16;
  static const double s5 = 20;
  static const double s6 = 24;
  static const double s8 = 32;
  static const double s10 = 40;
  static const double s12 = 48;
  static const double s16 = 64;
  static const double s20 = 80;
  static const double s24 = 96;
}

/// Rayons de bordure.
class BoxRadius {
  static const double sm = 4;
  static const double md = 8;
  static const double lg = 12;
  static const double xl = 16;
  static const double r2xl = 24;
  static const double full = 9999;
}
