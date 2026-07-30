"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { cx } from "@/components/ui";
import type { ManifestPage } from "@/lib/reader/pages";
import { Filmstrip } from "./filmstrip";
import {
  FIT_LABELS,
  MODE_LABELS,
  setSettings,
  useReaderSettings,
  type FitMode,
  type ReadingMode,
} from "@/lib/reader/store";

/**
 * Interface du lecteur : barres, réglages, barre de progression.
 *
 * Elle s'efface. Le principe qui gouverne tout ce fichier : on lit une planche,
 * pas une application. Les barres apparaissent au mouvement de la souris ou au
 * tap central, puis disparaissent après trois secondes d'inactivité.
 */

const HIDE_AFTER_MS = 3000;

export function ReaderChrome({
  title,
  page,
  pageCount,
  comicId,
  pages,
  onClose,
  onSeek,
  children,
}: {
  title: string;
  page: number;
  pageCount: number;
  comicId: string;
  pages: ManifestPage[];
  onClose: () => void;
  onSeek: (page: number) => void;
  children: React.ReactNode;
}) {
  const [visible, setVisible] = useState(true);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [stripOpen, setStripOpen] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Un panneau ouvert retient l'interface : rien de plus agaçant qu'une barre
  // qui s'efface pendant qu'on la manipule, ou une bande de vignettes qui
  // s'escamote au moment de choisir.
  const pinned = settingsOpen || stripOpen;

  const show = useCallback(() => {
    setVisible(true);
    if (timer.current) clearTimeout(timer.current);

    if (!pinned) {
      timer.current = setTimeout(() => setVisible(false), HIDE_AFTER_MS);
    }
  }, [pinned]);

  useEffect(() => {
    show();
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, [show]);

  // Le mouvement de souris révèle l'interface. Sur tactile, c'est le tap
  // central qui s'en charge — voir la zone dédiée plus bas.
  useEffect(() => {
    window.addEventListener("mousemove", show);
    return () => window.removeEventListener("mousemove", show);
  }, [show]);

  // « t » ouvre et ferme la bande, comme les autres raccourcis du lecteur.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const target = event.target as HTMLElement | null;
      if (target?.tagName === "INPUT" || target?.isContentEditable) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      if (event.key === "t" || event.key === "T") {
        event.preventDefault();
        setStripOpen((open) => !open);
        show();
      } else if (event.key === "Escape" && stripOpen) {
        // La bande capte Échap avant le lecteur : fermer un panneau ouvert est
        // toujours ce qu'on attend d'abord.
        event.stopPropagation();
        setStripOpen(false);
      }
    }

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
  }, [show, stripOpen]);

  return (
    <div className="relative h-dvh w-full overflow-hidden bg-black">
      {children}

      {/* Zone centrale : bascule l'interface au tap. Un tiers de la largeur,
          les deux autres tiers servant à tourner les pages. */}
      <button
        onClick={() => (visible ? setVisible(false) : show())}
        aria-label={visible ? "Masquer l'interface" : "Afficher l'interface"}
        className="absolute inset-y-0 left-[35%] w-[30%]"
      />

      <TopBar title={title} visible={visible} onClose={onClose} onToggleSettings={() => setSettingsOpen((v) => !v)} />

      <BottomBar
        visible={visible && !stripOpen}
        page={page}
        pageCount={pageCount}
        onSeek={onSeek}
        onToggleStrip={() => setStripOpen((open) => !open)}
      />

      <Filmstrip
        open={stripOpen}
        comicId={comicId}
        pages={pages}
        current={page}
        onSelect={onSeek}
        onClose={() => setStripOpen(false)}
      />

      {settingsOpen && <SettingsPanel onClose={() => setSettingsOpen(false)} />}
    </div>
  );
}

function TopBar({
  title,
  visible,
  onClose,
  onToggleSettings,
}: {
  title: string;
  visible: boolean;
  onClose: () => void;
  onToggleSettings: () => void;
}) {
  return (
    <div
      className={cx(
        "absolute inset-x-0 top-0 z-20 flex items-center gap-3 px-3 py-2",
        "bg-gradient-to-b from-black/85 to-transparent pb-8",
        "transition-opacity duration-(--motion-duration-normal)",
        visible ? "opacity-100" : "pointer-events-none opacity-0",
      )}
    >
      <IconButton onClick={onClose} label="Fermer le lecteur">
        <svg viewBox="0 0 20 20" fill="none" className="size-5" aria-hidden="true">
          <path d="M12 4 6 10l6 6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </IconButton>

      <p className="min-w-0 flex-1 truncate text-ui font-medium text-white/90">{title}</p>

      <IconButton onClick={onToggleSettings} label="Réglages de lecture">
        <svg viewBox="0 0 20 20" fill="none" className="size-5" aria-hidden="true">
          <path d="M4 6h12M4 10h12M4 14h12" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
        </svg>
      </IconButton>
    </div>
  );
}

function BottomBar({
  visible,
  page,
  pageCount,
  onSeek,
  onToggleStrip,
}: {
  visible: boolean;
  page: number;
  pageCount: number;
  onSeek: (page: number) => void;
  onToggleStrip: () => void;
}) {
  return (
    <div
      className={cx(
        "absolute inset-x-0 bottom-0 z-20 px-4 pb-3 pt-8",
        "bg-gradient-to-t from-black/85 to-transparent",
        "transition-opacity duration-(--motion-duration-normal)",
        visible ? "opacity-100" : "pointer-events-none opacity-0",
      )}
    >
      <div className="mx-auto flex max-w-3xl items-center gap-3">
        <span className="w-20 shrink-0 text-right font-mono text-meta tabular-nums text-white/70">
          {page + 1} / {pageCount}
        </span>

        {/*
          Un input range plutôt qu'une barre personnalisée : il apporte
          gratuitement le clavier, le tactile et l'accessibilité, que
          réimplémenter correctement demanderait bien plus de code que ne le
          justifie le gain visuel.
        */}
        <input
          type="range"
          min={0}
          max={Math.max(0, pageCount - 1)}
          value={page}
          onChange={(e) => onSeek(Number(e.target.value))}
          aria-label="Position dans l'album"
          className="h-1 flex-1 cursor-pointer appearance-none rounded-full bg-white/25 accent-[var(--accent)]"
        />

        <IconButton onClick={onToggleStrip} label="Pages de l'album (t)">
          <svg viewBox="0 0 20 20" fill="none" className="size-5" aria-hidden="true">
            <rect x="2.5" y="5" width="4" height="10" rx="1" stroke="currentColor" strokeWidth="1.5" />
            <rect x="8" y="5" width="4" height="10" rx="1" stroke="currentColor" strokeWidth="1.5" />
            <rect x="13.5" y="5" width="4" height="10" rx="1" stroke="currentColor" strokeWidth="1.5" />
          </svg>
        </IconButton>
      </div>
    </div>
  );
}

function SettingsPanel({ onClose }: { onClose: () => void }) {
  const settings = useReaderSettings();
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onPointerDown(event: PointerEvent) {
      if (!ref.current?.contains(event.target as Node)) onClose();
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
    }

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown, true);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown, true);
    };
  }, [onClose]);

  return (
    <div
      ref={ref}
      role="dialog"
      aria-label="Réglages de lecture"
      className="absolute right-3 top-12 z-30 w-64 rounded-xl border border-white/10 bg-neutral-900/95 p-4 shadow-2xl backdrop-blur"
    >
      <Setting label="Mode">
        {(Object.keys(MODE_LABELS) as ReadingMode[]).map((mode) => (
          <Option
            key={mode}
            active={settings.mode === mode}
            onClick={() => setSettings({ mode })}
          >
            {MODE_LABELS[mode]}
          </Option>
        ))}
      </Setting>

      {/* L'ajustement n'a pas de sens en défilement continu : la largeur y est
          imposée par la colonne. */}
      {settings.mode !== "scroll" && (
        <Setting label="Ajustement">
          {(Object.keys(FIT_LABELS) as FitMode[]).map((fit) => (
            <Option key={fit} active={settings.fit === fit} onClick={() => setSettings({ fit })}>
              {FIT_LABELS[fit]}
            </Option>
          ))}
        </Setting>
      )}

      <Setting label="Sens de lecture">
        <Option
          active={settings.direction === "ltr"}
          onClick={() => setSettings({ direction: "ltr" })}
        >
          Gauche → droite
        </Option>
        <Option
          active={settings.direction === "rtl"}
          onClick={() => setSettings({ direction: "rtl" })}
        >
          Droite → gauche
        </Option>
      </Setting>

      <p className="mt-3 border-t border-white/10 pt-3 text-micro leading-relaxed text-white/40">
        Flèches ou espace pour tourner · Début et Fin pour les extrémités ·
        <span className="text-white/55"> t </span> pour les vignettes ·
        <span className="text-white/55"> + − 0 </span> pour le zoom ·
        double-clic ou pincement pour agrandir · Échap pour sortir
      </p>
    </div>
  );
}

function Setting({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="mb-3 last:mb-0">
      <p className="mb-1.5 text-micro uppercase tracking-wide text-white/40">{label}</p>
      <div className="flex flex-wrap gap-1">{children}</div>
    </div>
  );
}

function Option({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      aria-pressed={active}
      className={cx(
        "pressable rounded-md px-2.5 py-1 text-meta font-medium",
        active ? "bg-accent text-white" : "bg-white/10 text-white/70 hover:bg-white/20",
      )}
    >
      {children}
    </button>
  );
}

function IconButton({
  onClick,
  label,
  children,
}: {
  onClick: () => void;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      aria-label={label}
      title={label}
      className="pressable grid size-9 shrink-0 place-items-center rounded-md text-white/80 hover:bg-white/10 hover:text-white"
    >
      {children}
    </button>
  );
}
