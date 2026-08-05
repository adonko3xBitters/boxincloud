"use client";

/**
 * Les onglets de premier niveau.
 *
 * # Pourquoi des onglets plutôt qu'une entrée de menu
 *
 * Le module eD2k vivait dans le menu du compte, derrière un avatar. C'était
 * défendable tant qu'on le voyait comme un réglage d'administration ; ça ne
 * l'est plus, parce que ce n'en est pas un. On y passe du temps : on cherche,
 * on surveille une file, on revient voir où en est un téléchargement.
 *
 * Un endroit qu'on visite ne se range pas derrière deux clics et une icône qui
 * ne l'annonce pas. Deux onglets le disent en un coup d'œil : voici les deux
 * moitiés de cette instance, vous êtes dans celle-ci.
 *
 * # Une navigation, pas un état
 *
 * Les deux onglets sont des routes, et c'est délibéré. Fondre eD2k dans la page
 * unique aurait obligé à charger son arbre de composants et son flux
 * d'événements pour tout le monde, y compris pour les comptes qui n'y ont pas
 * droit — et aurait fait perdre au module ses adresses profondes, qui portent
 * la section ouverte.
 *
 * # L'onglet eD2k n'apparaît qu'aux administrateurs
 *
 * amuled n'a aucune notion d'utilisateur : une seule file, un seul jeu de
 * préférences. Montrer l'onglet à un compte ordinaire ne ferait que proposer
 * une porte que l'API tient fermée.
 */

import Link from "next/link";

import { cx } from "@/components/ui";
import { useT } from "@/i18n";
import { useCurrentUser } from "@/lib/auth";

export type NavTab = "library" | "ed2k";

export function NavTabs({ active }: { active: NavTab }) {
  const t = useT();
  const { data: user } = useCurrentUser();

  return (
    <nav aria-label={t("nav.label")} className="flex items-center gap-0.5">
      <Tab href="/" label={t("nav.library")} current={active === "library"} />

      {user?.role === "admin" && (
        <Tab
          href="/ed2k"
          label={t("ed2k.title")}
          title={t("ed2k.menuHint")}
          current={active === "ed2k"}
        />
      )}
    </nav>
  );
}

function Tab({
  href,
  label,
  title,
  current,
}: {
  href: string;
  label: string;
  title?: string;
  current: boolean;
}) {
  return (
    <Link
      href={href}
      title={title}
      // `aria-current="page"` et non `aria-selected` : ce sont des liens vers
      // deux adresses, pas les volets d'un widget à onglets. Un lecteur d'écran
      // doit annoncer une navigation, ce qu'il fera à l'atterrissage.
      aria-current={current ? "page" : undefined}
      className={cx(
        "pressable rounded-md px-2.5 py-1 text-ui",
        current
          ? "bg-accent-subtle font-medium text-accent-text"
          : "text-muted hover:bg-surface-hover hover:text-fg",
      )}
    >
      {label}
    </Link>
  );
}
