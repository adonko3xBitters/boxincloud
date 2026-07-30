"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { cx } from "@/components/ui";
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
  onClose,
  onSeek,
  children,
}: {
  title: string;
  page: number;
  pageCount: number;
  onClose: () => void;
  onSeek: (page: number) => void;
  children: React.ReactNode;
}) {
  const [visible, setVisible] = useState(true);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const show = useCallback(() => {
    setVisible(true);
    if (timer.current) clearTimeout(timer.current);

    // Le panneau de réglages ouvert bloque l'effacement : rien de plus agaçant
    // qu'une interface qui disparaît pendant qu'on la manipule.
    if (!settingsOpen) {
      timer.current = setTimeout(() => setVisible(false), HIDE_AFTER_MS);
    }
  }, [settingsOpen]);

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
        visible={visible}
        page={page}
        pageCount={pageCount}
        onSeek={onSeek}
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
        "transition-opacity duration-[--motion-duration-normal]",
        visible ? "opacity-100" : "pointer-events-none opacity-0",
      )}
    >
      <IconButton onClick={onClose} label="Fermer le lecteur">
        <svg viewBox="0 0 20 20" fill="none" className="size-5" aria-hidden="true">
          <path d="M12 4 6 10l6 6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      </IconButton>

      <p className="min-w-0 flex-1 truncate text-sm font-medium text-white/90">{title}</p>

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
}: {
  visible: boolean;
  page: number;
  pageCount: number;
  onSeek: (page: number) => void;
}) {
  return (
    <div
      className={cx(
        "absolute inset-x-0 bottom-0 z-20 px-4 pb-3 pt-8",
        "bg-gradient-to-t from-black/85 to-transparent",
        "transition-opacity duration-[--motion-duration-normal]",
        visible ? "opacity-100" : "pointer-events-none opacity-0",
      )}
    >
      <div className="mx-auto flex max-w-3xl items-center gap-3">
        <span className="w-16 shrink-0 text-right font-mono text-xs tabular-nums text-white/70">
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

      <p className="mt-3 border-t border-white/10 pt-3 text-[11px] leading-relaxed text-white/40">
        Flèches ou espace pour tourner · Début et Fin pour les extrémités ·
        Échap pour sortir
      </p>
    </div>
  );
}

function Setting({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="mb-3 last:mb-0">
      <p className="mb-1.5 text-[11px] uppercase tracking-wide text-white/40">{label}</p>
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
        "rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
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
      className="grid size-9 shrink-0 place-items-center rounded-md text-white/80 transition-colors hover:bg-white/10 hover:text-white"
    >
      {children}
    </button>
  );
}
