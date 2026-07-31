"use client";

import { useRouter } from "next/navigation";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { cx } from "./ui";
import { DeleteDialog, MoveDialog } from "./manage-dialogs";
import * as api from "@/lib/api/endpoints";
import { useT, type MessageKey } from "@/i18n";
import { useWorkspace, type ReadStatus, type SortOrder, type ViewMode } from "@/lib/workspace";

/**
 * Barre d'outils.
 *
 * Deux registres sur une seule ligne : à gauche les actions qui portent sur la
 * sélection, à droite les filtres et le mode de vue. Les actions restent
 * visibles en permanence, désactivées tant que rien n'est sélectionné — les
 * masquer ferait douter de leur existence.
 */
export function Toolbar({
  visibleIds,
  titleOf,
}: {
  visibleIds: string[];
  titleOf: (id: string) => string;
}) {
  const t = useT();
  const router = useRouter();
  const queryClient = useQueryClient();
  const {
    selection, clearSelection, selectAll,
    view, setView,
    readStatus, setReadStatus,
    sort, setSort,
    favorites, refreshMarks,
  } = useWorkspace();

  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<"delete" | "move" | null>(null);

  const count = selection.length;
  const has = count > 0;
  const titles = selection.map(titleOf);

  // Si toute la sélection est déjà en favori, le bouton retire au lieu d'ajouter.
  const allFavorite = has && selection.every((id) => favorites.has(id));

  async function run(action: api.BulkAction) {
    if (!has || busy) return;
    setBusy(true);
    try {
      await api.bulk(action, selection);
      refreshMarks();
      // La progression change : les listes et compteurs doivent suivre.
      await queryClient.invalidateQueries({ queryKey: ["comics"] });
      await queryClient.invalidateQueries({ queryKey: ["progress"] });
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b border-border bg-surface px-3 py-2">
      {/* Actions sur la sélection */}
      <div className="flex items-center gap-0.5">
        <Tool
          label={t("toolbar.read")}
          disabled={count !== 1}
          onClick={() => selection[0] && router.push(`/read?id=${selection[0]}`)}
          icon={<PlayIcon />}
        />
        <Divider />
        <Tool
          label={t("toolbar.markRead")}
          disabled={!has || busy}
          onClick={() => void run("read")}
          icon={<CheckIcon />}
        />
        <Tool
          label={t("toolbar.markUnread")}
          disabled={!has || busy}
          onClick={() => void run("unread")}
          icon={<UndoIcon />}
        />
        <Divider />
        <Tool
          label={allFavorite ? t("toolbar.unfavorite") : t("toolbar.favorite")}
          disabled={!has || busy}
          onClick={() => void run(allFavorite ? "unfavorite" : "favorite")}
          icon={<HeartIcon filled={allFavorite} />}
          active={allFavorite}
        />
        <Divider />
        <Tool
          label={t("toolbar.moveToFolder")}
          disabled={!has || busy}
          onClick={() => setDialog("move")}
          icon={<FolderMoveIcon />}
        />
        <Tool
          label={t("toolbar.removeFromLibrary")}
          disabled={!has || busy}
          onClick={() => setDialog("delete")}
          icon={<TrashIcon />}
          destructive
        />
      </div>

      <div className="flex items-center gap-2 text-meta text-muted">
        {has ? (
          <>
            <span className="tabular-nums">
              {t(count > 1 ? "toolbar.selectedOther" : "toolbar.selectedOne", { count })}
            </span>
            <button onClick={clearSelection} className="text-accent-text transition-colors hover:underline">
              {t("toolbar.clearSelection")}
            </button>
          </>
        ) : (
          <button
            onClick={() => selectAll(visibleIds)}
            disabled={visibleIds.length === 0}
            className="text-subtle hover:text-fg disabled:opacity-40"
          >
            {t("toolbar.selectAll")}
          </button>
        )}
      </div>

      {/* Filtres et vue, poussés à droite */}
      <div className="ml-auto flex flex-wrap items-center gap-x-3 gap-y-2">
        <SegmentedControl
          label={t("toolbar.readStatus")}
          value={readStatus}
          onChange={(v) => setReadStatus(v as ReadStatus)}
          options={[
            { value: "", label: t("toolbar.all") },
            { value: "unread", label: t("toolbar.unread") },
            { value: "in_progress", label: t("toolbar.inProgress") },
            { value: "read", label: t("toolbar.done") },
          ]}
        />

        <SegmentedControl
          label={t("toolbar.sort")}
          value={sort}
          onChange={(v) => setSort(v as SortOrder)}
          options={[
            { value: "recent", label: t("toolbar.sortAdded") },
            { value: "title", label: t("toolbar.sortTitle") },
            { value: "released", label: t("toolbar.sortReleased") },
          ]}
        />

        <div className="flex items-center gap-0.5 rounded-md border border-border p-0.5">
          {(["grid", "list", "coverflow"] as ViewMode[]).map((mode) => (
            <button
              key={mode}
              onClick={() => setView(mode)}
              aria-pressed={view === mode}
              aria-label={t(VIEW_KEYS[mode])}
              title={t(VIEW_KEYS[mode])}
              className={cx(
                "pressable grid size-7 place-items-center rounded",
                view === mode
                  ? "bg-accent text-inverted shadow-sm"
                  : "text-subtle hover:bg-surface-hover hover:text-fg",
              )}
            >
              {mode === "grid" ? <GridIcon /> : mode === "list" ? <ListIcon /> : <CoverflowIcon />}
            </button>
          ))}
        </div>
      </div>

      {dialog === "delete" && (
        <DeleteDialog ids={selection} titles={titles} onClose={() => setDialog(null)} />
      )}
      {dialog === "move" && (
        <MoveDialog ids={selection} titles={titles} onClose={() => setDialog(null)} />
      )}
    </div>
  );
}

const VIEW_KEYS: Record<ViewMode, MessageKey> = {
  grid: "toolbar.viewGrid",
  list: "toolbar.viewList",
  coverflow: "toolbar.viewCoverflow",
};

// ─── Éléments ────────────────────────────────────────────────────────────────

function Tool({
  label,
  icon,
  onClick,
  disabled,
  active,
  destructive,
}: {
  label: string;
  icon: React.ReactNode;
  onClick: () => void;
  disabled?: boolean;
  active?: boolean;
  destructive?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={label}
      aria-label={label}
      className={cx(
        "pressable grid size-8 place-items-center rounded-md",
        "disabled:opacity-30 disabled:cursor-not-allowed",
        active ? "text-danger" : "text-muted",
        // Le rouge n'apparaît qu'au survol : une barre d'outils constellée de
        // rouge banalise la couleur, qui ne signale alors plus rien.
        !disabled && (destructive ? "hover:bg-danger/10 hover:text-danger" : "hover:bg-surface-hover hover:text-fg"),
      )}
    >
      {icon}
    </button>
  );
}

function Divider() {
  return <span className="mx-1 h-4 w-px bg-border" />;
}

function SegmentedControl({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
}) {
  return (
    <div className="flex items-center gap-1.5">
      <span className="text-micro uppercase tracking-wider text-subtle">{label}</span>
      <div className="flex items-center gap-0.5 rounded-md border border-border p-0.5" role="group" aria-label={label}>
        {options.map((option) => (
          <button
            key={option.value}
            onClick={() => onChange(option.value)}
            aria-pressed={value === option.value}
            className={cx(
              "pressable rounded px-2.5 py-1 text-meta font-medium",
              value === option.value
                ? "bg-accent text-inverted shadow-sm"
                : "text-muted hover:bg-surface-hover hover:text-fg",
            )}
          >
            {option.label}
          </button>
        ))}
      </div>
    </div>
  );
}

// ─── Icônes ──────────────────────────────────────────────────────────────────

function PlayIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className="size-4" aria-hidden="true">
      <path d="M5 3.5v9l7-4.5-7-4.5Z" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
      <path d="m3 8.5 3.5 3.5L13 5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function UndoIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
      <path d="M3 8h7a3 3 0 0 1 0 6H6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M5.5 5.5 3 8l2.5 2.5" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function HeartIcon({ filled }: { filled?: boolean }) {
  return (
    <svg viewBox="0 0 16 16" fill={filled ? "currentColor" : "none"} className="size-4" aria-hidden="true">
      <path
        d="M8 13.5S2.5 10.2 2.5 6.6A3.1 3.1 0 0 1 8 4.3a3.1 3.1 0 0 1 5.5 2.3c0 3.6-5.5 6.9-5.5 6.9Z"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function FolderMoveIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
      <path d="M2 4.5A1.5 1.5 0 0 1 3.5 3h2.2l1.3 1.5h5.5A1.5 1.5 0 0 1 14 6v5.5a1.5 1.5 0 0 1-1.5 1.5h-9A1.5 1.5 0 0 1 2 11.5v-7Z"
            stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
      <path d="M6.5 8.5h4M9 7l1.5 1.5L9 10" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
      <path d="M3 4.5h10M6.5 4.5V3h3v1.5M4.5 4.5l.5 8a1 1 0 0 0 1 1h4a1 1 0 0 0 1-1l.5-8"
            stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function GridIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className="size-4" aria-hidden="true">
      <rect x="2" y="2" width="5" height="5" rx="1" />
      <rect x="9" y="2" width="5" height="5" rx="1" />
      <rect x="2" y="9" width="5" height="5" rx="1" />
      <rect x="9" y="9" width="5" height="5" rx="1" />
    </svg>
  );
}

function ListIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className="size-4" aria-hidden="true">
      <rect x="2" y="3" width="12" height="1.6" rx="0.8" />
      <rect x="2" y="7.2" width="12" height="1.6" rx="0.8" />
      <rect x="2" y="11.4" width="12" height="1.6" rx="0.8" />
    </svg>
  );
}

/** Trois couvertures en perspective : celle du centre de face, les voisines de biais. */
function CoverflowIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="currentColor" className="size-4" aria-hidden="true">
      <path d="M1.5 5.5 3.5 4v8l-2-1.5v-5Z" opacity="0.45" />
      <path d="M14.5 5.5 12.5 4v8l2-1.5v-5Z" opacity="0.45" />
      <rect x="5" y="2.5" width="6" height="11" rx="1" />
    </svg>
  );
}
