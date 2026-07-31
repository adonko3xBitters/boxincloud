"use client";

import { useEffect, useState } from "react";

import { cx } from "./ui";
import { AccountsPanel } from "./accounts-panel";
import { DiscoverySources } from "./discovery-panel";
import { MobileAppDialog } from "./mobile-app-dialog";
import { SessionsPanel } from "./sessions-panel";
import { StoragePanel } from "./storage-panel";
import { useT, type MessageKey } from "@/i18n";

/**
 * Configuration — un hub à cartes.
 *
 * Le menu du compte listait à plat le stockage, les comptes, les appareils et
 * l'application mobile, entre l'identité de l'utilisateur et le bouton de
 * déconnexion. Cinq entrées sans hiérarchie, dont trois réservées aux
 * administrateurs, dans une surface qu'on ouvre surtout pour se déconnecter ou
 * changer de langue.
 *
 * Le menu ne garde donc que ce qui le concerne — qui je suis, ma langue, ma
 * sortie — et tout le réglage passe derrière une seule entrée.
 *
 * Les cartes portent une description, ce qui n'est pas de l'ornement : « Comptes »
 * et « Appareils connectés » sont impossibles à distinguer sur leur seul titre
 * quand on cherche où révoquer une session.
 */

type SectionKey = "storage" | "accounts" | "sources" | "devices" | "mobile";

type Section = {
  key: SectionKey;
  title: MessageKey;
  description: MessageKey;
  /** Réservée aux administrateurs : l'API refuserait de toute façon. */
  adminOnly: boolean;
  icon: React.ReactNode;
};

const SECTIONS: Section[] = [
  {
    key: "storage",
    title: "storage.title",
    description: "settings.storage.hint",
    adminOnly: true,
    icon: (
      <path
        d="M3 5.5C3 4.4 5.7 3.5 9 3.5s6 .9 6 2v7c0 1.1-2.7 2-6 2s-6-.9-6-2v-7Zm0 3.5c0 1.1 2.7 2 6 2s6-.9 6-2"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "accounts",
    title: "accounts.title",
    description: "settings.accounts.hint",
    adminOnly: true,
    icon: (
      <path
        d="M9 9a2.75 2.75 0 1 0 0-5.5A2.75 2.75 0 0 0 9 9Zm-5.5 5.5c0-2.5 2.5-4 5.5-4s5.5 1.5 5.5 4"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "sources",
    title: "discovery.sources.title",
    description: "settings.sources.hint",
    adminOnly: true,
    icon: (
      <path
        d="M9 15.5a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13Zm-6.5-6.5h13M9 2.5c1.8 2 2.7 4.2 2.7 6.5S10.8 13.5 9 15.5C7.2 13.5 6.3 11.3 6.3 9S7.2 4.5 9 2.5Z"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "devices",
    title: "account.devices",
    description: "settings.devices.hint",
    adminOnly: false,
    icon: (
      <path
        d="M6.5 2.5h5a1.5 1.5 0 0 1 1.5 1.5v10a1.5 1.5 0 0 1-1.5 1.5h-5A1.5 1.5 0 0 1 5 14V4a1.5 1.5 0 0 1 1.5-1.5Zm2 10.5h1"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    ),
  },
  {
    key: "mobile",
    title: "download.title",
    description: "settings.mobile.hint",
    adminOnly: false,
    icon: (
      <path
        d="M9 3v8m0 0 3-3m-3 3-3-3m-3 6h12"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    ),
  },
];

export function SettingsHub({
  isAdmin = false,
  onClose,
}: {
  isAdmin?: boolean;
  onClose: () => void;
}) {
  const t = useT();
  const [section, setSection] = useState<SectionKey | null>(null);

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      event.stopPropagation();
      // Échap remonte d'un niveau plutôt que de tout fermer : sortir d'un
      // réglage ne doit pas faire perdre le hub qui y menait.
      if (section) setSection(null);
      else onClose();
    }
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [onClose, section]);

  // Les sections détaillées gardent leurs panneaux existants : ils étaient déjà
  // écrits, testés et traduits. Le hub ne fait que les rassembler.
  if (section === "storage") return <StoragePanel onClose={() => setSection(null)} />;
  if (section === "accounts") return <AccountsPanel onClose={() => setSection(null)} />;
  if (section === "devices") return <SessionsPanel onClose={() => setSection(null)} />;
  if (section === "mobile") return <MobileAppDialog onClose={() => setSection(null)} />;

  const visible = SECTIONS.filter((entry) => !entry.adminOnly || isAdmin);

  return (
    <div className="fixed inset-0 z-[60] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("settings.title")}
        className="rise-in flex max-h-[85vh] w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
      >
        <header className="flex items-center gap-3 border-b border-border px-4 py-3">
          <h2 className="text-title font-semibold text-fg">
            {section === "sources" ? t("discovery.sources.title") : t("settings.title")}
          </h2>

          {section === "sources" && (
            <button
              onClick={() => setSection(null)}
              className="pressable rounded px-2 py-1 text-meta text-muted hover:bg-surface-hover hover:text-fg"
            >
              {t("settings.back")}
            </button>
          )}

          <button
            onClick={onClose}
            aria-label={t("action.close")}
            className="pressable ml-auto grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path
                d="m4 4 8 8M12 4l-8 8"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
              />
            </svg>
          </button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4">
          {section === "sources" ? (
            <DiscoverySources />
          ) : (
            <ul className="grid gap-2 sm:grid-cols-2">
              {visible.map((entry) => (
                <li key={entry.key}>
                  <button
                    onClick={() => setSection(entry.key)}
                    className={cx(
                      "pressable flex w-full items-start gap-3 rounded-lg border border-border p-3 text-left",
                      "hover:border-border-strong hover:bg-surface-hover",
                    )}
                  >
                    <span className="grid size-9 shrink-0 place-items-center rounded-md bg-accent-subtle text-accent-text">
                      <svg viewBox="0 0 18 18" fill="none" className="size-4.5" aria-hidden="true">
                        {entry.icon}
                      </svg>
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate text-ui font-medium text-fg">
                        {t(entry.title)}
                      </span>
                      <span className="block text-meta text-muted">
                        {t(entry.description)}
                      </span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </div>
  );
}
