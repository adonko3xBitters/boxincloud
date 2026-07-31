"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";

import { Button, Spinner, cx } from "./ui";
import * as api from "@/lib/api/endpoints";
import { describeError } from "@/lib/api/problem";
import { useT } from "@/i18n";

/**
 * Rapprochement de métadonnées.
 *
 * Il PROPOSE, il n'applique pas. Choisir une fiche remplit les champs du
 * formulaire d'édition ; c'est l'utilisateur qui enregistre ensuite, après
 * avoir relu.
 *
 * La distinction est tout l'écran. Un rapprochement de métadonnées est flou par
 * nature — deux albums peuvent partager un titre, une base peut proposer
 * l'édition anglaise d'une œuvre française — et écrire directement reviendrait
 * à remplacer une fiche par celle d'un homonyme sans que personne s'en aperçoive.
 * Une absence se voit et se corrige ; une fiche fausse mais plausible, non.
 *
 * D'où aussi le lien vers la fiche d'origine sur chaque proposition : c'est le
 * seul moyen pour l'utilisateur de vérifier ce qu'on lui met sous les yeux.
 */

/** Ce que le rapprochement sait remplir. Le titre reste hors de portée. */
export type MetadataFill = {
  summary?: string;
  language?: string;
};

export function MetadataMatch({
  title,
  onApply,
}: {
  title: string;
  onApply: (fill: MetadataFill) => void;
}) {
  const t = useT();
  const [applied, setApplied] = useState(false);

  const search = useMutation({
    mutationFn: () => api.discoveryDescribe({ title }),
  });

  const data = search.data;
  const failing = data?.sources.some((source) => source.error) ?? false;

  return (
    <div className="flex flex-col gap-2 rounded-md border border-border p-3">
      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="ghost"
          onClick={() => {
            setApplied(false);
            search.mutate();
          }}
          disabled={search.isPending || !title.trim()}
        >
          {search.isPending ? t("metadata.searching") : t("metadata.find")}
        </Button>
        {search.isPending && <Spinner className="size-4" />}
      </div>

      <p className="text-meta text-subtle">{t("metadata.explain")}</p>

      {search.error && (
        <p className="text-meta text-danger">{describeError(search.error, t)}</p>
      )}

      {applied && <p className="text-meta text-muted">{t("metadata.applied")}</p>}

      {/*
        Aucune base activée n'est un cas normal — une instance coupée
        d'Internet — et non une panne. Le dire évite de laisser croire que la
        recherche a échoué.
      */}
      {data && data.sources.length === 0 && (
        <p className="text-meta text-muted">{t("metadata.offline")}</p>
      )}

      {failing && <p className="text-meta text-danger">{t("metadata.partial")}</p>}

      {data && data.sources.length > 0 && data.candidates.length === 0 && (
        <div className="text-meta text-muted">
          <p>{t("metadata.none")}</p>
          {/*
            La réserve est dite plutôt que tue : ces bases sont des catalogues
            de LIVRES, et la franco-belge y est mal couverte. Laisser
            l'utilisateur conclure que son album n'existe nulle part serait
            faux.
          */}
          <p className="mt-1 text-subtle">{t("metadata.noneHint")}</p>
        </div>
      )}

      {data && data.candidates.length > 0 && (
        <ul className="flex flex-col gap-2">
          {data.candidates.map((candidate, index) => (
            <Candidate
              key={`${candidate.providerKind}-${index}`}
              candidate={candidate}
              onApply={() => {
                onApply({
                  summary: candidate.summary,
                  language: candidate.language,
                });
                setApplied(true);
              }}
            />
          ))}
        </ul>
      )}
    </div>
  );
}

function Candidate({
  candidate,
  onApply,
}: {
  candidate: api.DiscoveryDescription;
  onApply: () => void;
}) {
  const t = useT();
  const percent = Math.round(candidate.confidence * 100);

  return (
    <li className="flex gap-2.5 rounded-md border border-border p-2">
      {candidate.coverUrl ? (
        // Image servie par la base d'origine : boxincloud ne réhéberge rien, et
        // `no-referrer` évite de lui annoncer l'adresse de l'instance.
        <img
          src={candidate.coverUrl}
          alt=""
          referrerPolicy="no-referrer"
          loading="lazy"
          className="h-16 w-11 shrink-0 rounded object-cover"
        />
      ) : (
        <div className="h-16 w-11 shrink-0 rounded bg-surface-hover" />
      )}

      <div className="min-w-0 flex-1">
        <p className="truncate text-ui font-medium text-fg">{candidate.title}</p>

        {candidate.authors && candidate.authors.length > 0 && (
          <p className="truncate text-meta text-muted">{candidate.authors.join(", ")}</p>
        )}

        <p className="text-meta text-subtle">
          {t("metadata.from", { source: candidate.providerName })}
          {candidate.published ? ` · ${candidate.published}` : ""}
          {candidate.publisher ? ` · ${candidate.publisher}` : ""}
        </p>

        <div className="mt-1.5 flex flex-wrap items-center gap-2">
          {/*
            La confiance est montrée, pas cachée derrière un tri. L'utilisateur
            doit pouvoir voir qu'une proposition est faible avant de la prendre
            — c'est ce qui distingue un choix d'une acceptation.
          */}
          <span
            className={cx(
              "rounded px-1.5 py-0.5 text-meta tabular-nums",
              percent >= 80
                ? "bg-accent-subtle text-accent-text"
                : "border border-border text-subtle",
            )}
          >
            {t("metadata.confidence", { percent })}
          </span>

          <button
            type="button"
            onClick={onApply}
            className="pressable rounded border border-border px-2 py-0.5 text-meta text-fg hover:bg-surface-hover"
          >
            {t("metadata.apply")}
          </button>

          {candidate.pageUrl && (
            <a
              href={candidate.pageUrl}
              target="_blank"
              rel="noreferrer noopener"
              className="text-meta text-muted underline underline-offset-2 hover:text-fg"
            >
              {t("metadata.openSource")}
            </a>
          )}
        </div>
      </div>
    </li>
  );
}
