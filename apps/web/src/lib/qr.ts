import qrcode from "qrcode-generator";

/**
 * Encode une chaîne en code QR, rendu comme un unique tracé SVG.
 *
 * L'encodage vient d'une bibliothèque plutôt que d'ici. Un encodeur QR tient
 * en quelques centaines de lignes — correction Reed-Solomon, choix de version,
 * masquage — dont aucune ne se relit : une erreur ne se voit qu'au moment où un
 * téléphone refuse de scanner, sur un appareil qu'on n'a pas sous la main.
 *
 * Le rendu, lui, reste ici : un `<path>` unique plutôt qu'un millier de
 * `<rect>`, et un SVG plutôt qu'un canvas, pour que le code reste net à
 * l'impression comme à l'agrandissement.
 */
export type QrCode = {
  /** Côté de la grille, en modules. Sert de `viewBox`. */
  size: number;
  /** Tracé des modules sombres, un carré unitaire par module. */
  path: string;
};

export function encodeQR(data: string): QrCode {
  // Version 0 : la plus petite qui contienne la donnée.
  //
  // Niveau de correction M (~15 %). L le rendrait plus petit, mais un code
  // affiché à l'écran est photographié de biais, parfois sur une dalle
  // sale — la marge de correction sert exactement à ça.
  const qr = qrcode(0, "M");
  qr.addData(data);
  qr.make();

  const size = qr.getModuleCount();
  const parts: string[] = [];

  for (let row = 0; row < size; row += 1) {
    for (let col = 0; col < size; col += 1) {
      if (qr.isDark(row, col)) parts.push(`M${col} ${row}h1v1h-1z`);
    }
  }

  return { size, path: parts.join("") };
}
