"use client";

import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { buttonClass, cx } from "./ui";
import { imageURL } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useT } from "@/i18n";
import { useWorkspace } from "@/lib/workspace";

/**
 * Panneau de détail.
 *
 * Colonne de droite, permanente : l'album mis en avant s'y affiche sans quitter
 * la liste. C'est ce qui remplace une page de détail — sur une seule page, on
 * consulte et on édite au même endroit.
 */
export function DetailPanel() {
  const t = useT();
  const { focused, favorites, ratings, refreshMarks } = useWorkspace();
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);

  const comic = useQuery({
    queryKey: ["comic", focused],
    queryFn: () => api.getComic(focused!),
    enabled: Boolean(focused),
  });

  const progress = useQuery({
    queryKey: ["progress", focused],
    queryFn: () => api.getProgress(focused!),
    enabled: Boolean(focused),
  });

  // Changer d'album ferme l'édition en cours : conserver un formulaire ouvert
  // sur un autre album ferait enregistrer la correction au mauvais endroit.
  useEffect(() => setEditing(false), [focused]);

  if (!focused) {
    return (
      <aside className="hidden w-[340px] shrink-0 border-l border-border bg-surface-sunken p-4 xl:block">
        <p className="text-center text-meta text-subtle">
          Sélectionnez un album pour voir son détail
        </p>
      </aside>
    );
  }

  if (!comic.data) {
    return (
      <aside className="hidden w-[340px] shrink-0 border-l border-border bg-surface-sunken p-4 xl:block">
        <div className="skeleton rounded-cover" style={{ aspectRatio: 0.7 }} />
      </aside>
    );
  }

  const item = comic.data;
  const isFavorite = favorites.has(item.id);
  const rating = ratings.get(item.id) ?? 0;
  const started = progress.data && progress.data.page > 0;

  async function toggleFavorite() {
    await api.setFavorite(item.id, !isFavorite);
    refreshMarks();
  }

  return (
    <aside
      key={item.id}
      className="slide-in-right hidden w-[340px] shrink-0 flex-col overflow-y-auto border-l border-border bg-surface-sunken xl:flex"
    >
      <img
        src={imageURL(item.coverPath, { width: 640 })}
        alt=""
        className="w-full object-cover"
        style={{ aspectRatio: 0.7 }}
      />

      <div className="flex flex-col gap-3 p-3">
        <div>
          {item.seriesName && (
            <p className="truncate text-micro uppercase tracking-wide text-accent-text">
              {item.seriesName}
            </p>
          )}
          <h2 className="text-title font-semibold leading-snug text-fg">{item.title}</h2>
        </div>

        <div className="flex items-center gap-1.5">
          <Link
            href={started ? `/read?id=${item.id}&page=${progress.data!.page}` : `/read?id=${item.id}`}
            className={cx(buttonClass("primary", "sm"), "flex-1")}
          >
            {started ? t("detail.resume", { page: progress.data!.page + 1 }) : t("toolbar.read")}
          </Link>

          <button
            onClick={() => void toggleFavorite()}
            title={isFavorite ? t("toolbar.unfavorite") : t("toolbar.favorite")}
            aria-pressed={isFavorite}
            className={cx(
              "pressable grid size-9 shrink-0 place-items-center rounded-md border border-border",
              isFavorite ? "text-danger" : "text-subtle hover:text-fg hover:bg-surface-hover",
            )}
          >
            <svg viewBox="0 0 16 16" fill={isFavorite ? "currentColor" : "none"} className="size-4" aria-hidden="true">
              <path d="M8 13.5S2.5 10.2 2.5 6.6A3.1 3.1 0 0 1 8 4.3a3.1 3.1 0 0 1 5.5 2.3c0 3.6-5.5 6.9-5.5 6.9Z"
                    stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
            </svg>
          </button>

          <button
            onClick={() => setEditing((v) => !v)}
            title={t("detail.editMetadata")}
            aria-pressed={editing}
            className={cx(
              "pressable grid size-9 shrink-0 place-items-center rounded-md border border-border",
              editing ? "bg-accent text-inverted" : "text-subtle hover:text-fg hover:bg-surface-hover",
            )}
          >
            <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
              <path d="m10.5 2.5 3 3-8 8H2.5v-3l8-8Z" stroke="currentColor" strokeWidth="1.4" strokeLinejoin="round" />
            </svg>
          </button>
        </div>

        {started && (
          <div>
            <div className="mb-1 flex justify-between text-meta text-muted">
              <span>
                {t("detail.pageOf", {
                  page: progress.data!.page + 1,
                  total: progress.data!.pageCount || item.pageCount,
                })}
              </span>
              <span>{Math.round(progress.data!.percent)} %</span>
            </div>
            <div className="h-1 overflow-hidden rounded-full bg-border">
              <div
                className="h-full bg-accent transition-[width] duration-(--motion-duration-slow) ease-standard"
                style={{ width: `${Math.min(100, progress.data!.percent)}%` }}
              />
            </div>
          </div>
        )}

        <RatingRow comicId={item.id} value={rating} onChange={refreshMarks} />

        {editing ? (
          <EditForm
            comic={item}
            onDone={async () => {
              setEditing(false);
              await queryClient.invalidateQueries({ queryKey: ["comic", item.id] });
              await queryClient.invalidateQueries({ queryKey: ["comics"] });
            }}
          />
        ) : (
          <>
            {item.summary && (
              <p className="text-ui leading-relaxed text-muted">{item.summary}</p>
            )}

            <dl className="grid grid-cols-2 gap-x-3 gap-y-2.5 text-meta">
              <Field label={t("detail.number")} value={item.number || "—"} />
              <Field label={t("detail.pages")} value={String(item.pageCount)} />
              <Field label={t("detail.format")} value={item.format.toUpperCase()} />
              <Field label={t("detail.size")} value={formatBytes(item.fileSize)} />
              {item.releasedAt && <Field label={t("detail.released")} value={item.releasedAt} />}
              {item.language && <Field label={t("detail.language")} value={item.language.toUpperCase()} />}
            </dl>

            <div className="border-t border-border pt-2">
              <p className="mb-1 text-micro uppercase tracking-wide text-subtle">{t("detail.file")}</p>
              <p className="break-all text-meta text-muted">{item.fileName}</p>
              {item.folderPath && (
                <p className="mt-1 break-all text-meta text-subtle">{item.folderPath}/</p>
              )}
            </div>
          </>
        )}
      </div>
    </aside>
  );
}

function RatingRow({
  comicId,
  value,
  onChange,
}: {
  comicId: string;
  value: number;
  onChange: () => void;
}) {
  async function set(next: number) {
    await api.setRating(comicId, next === value ? 0 : next);
    onChange();
  }

  const t = useT();
  return (
    <div className="flex items-center gap-2">
      <span className="text-micro uppercase tracking-wide text-subtle">{t("detail.rating")}</span>
      <div className="flex gap-1">
        {[1, 2, 3, 4, 5].map((step) => (
          <button
            key={step}
            onClick={() => void set(step)}
            aria-label={t("detail.rate", { step })}
            className={cx(
              "pressable size-3.5 rounded-full",
              step <= value ? "bg-warning" : "bg-border hover:bg-border-strong",
            )}
          />
        ))}
      </div>
      {value > 0 && (
        <button onClick={() => void set(0)} className="text-meta text-subtle transition-colors hover:text-fg">
          effacer
        </button>
      )}
    </div>
  );
}

/**
 * Édition des métadonnées.
 *
 * Les champs modifiés sont verrouillés côté serveur : une réindexation ne les
 * écrasera plus. C'est indiqué explicitement — sans quoi l'utilisateur pourrait
 * craindre de perdre sa correction au prochain scan.
 */
function EditForm({
  comic,
  onDone,
}: {
  comic: { id: string; title: string; number?: string; summary?: string; language?: string };
  onDone: () => void;
}) {
  const t = useT();
  const [title, setTitle] = useState(comic.title);
  const [number, setNumber] = useState(comic.number ?? "");
  const [summary, setSummary] = useState(comic.summary ?? "");
  const [saving, setSaving] = useState(false);

  async function save(event: React.FormEvent) {
    event.preventDefault();
    setSaving(true);
    try {
      await api.editComic(comic.id, {
        title: title !== comic.title ? title : undefined,
        number: number !== (comic.number ?? "") ? number : undefined,
        summary: summary !== (comic.summary ?? "") ? summary : undefined,
      });
      onDone();
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={save} className="flex flex-col gap-2">
      <SmallField label={t("detail.title")} value={title} onChange={setTitle} />
      <SmallField label={t("detail.number")} value={number} onChange={setNumber} />

      <label className="flex flex-col gap-1">
        <span className="text-micro uppercase tracking-wide text-subtle">{t("detail.summary")}</span>
        <textarea
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          rows={4}
          className="rounded-md border border-border bg-surface px-2.5 py-1.5 text-ui text-fg transition-colors focus:border-accent"
        />
      </label>

      <p className="text-micro leading-relaxed text-subtle">
        Les champs modifiés sont verrouillés : un nouveau scan ne les écrasera pas.
      </p>

      <div className="flex gap-1.5">
        <button type="submit" disabled={saving} className={cx(buttonClass("primary", "sm"), "flex-1")}>
          {saving ? t("storage.saving") : t("action.save")}
        </button>
        <button type="button" onClick={onDone} className={buttonClass("secondary", "sm")}>
          Annuler
        </button>
      </div>
    </form>
  );
}

function SmallField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-micro uppercase tracking-wide text-subtle">{label}</span>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg transition-colors focus:border-accent"
      />
    </label>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-micro uppercase tracking-wide text-subtle">{label}</dt>
      <dd className="truncate text-fg">{value}</dd>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} o`;
  const units = ["ko", "Mo", "Go"];
  let value = bytes / 1024;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(1)} ${units[unit]}`;
}
