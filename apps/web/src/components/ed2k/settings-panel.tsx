"use client";

/**
 * Paramètres du module — la déclaration du démon.
 *
 * Ce panneau vient de la page elle-même, où il vivait tant qu'il était le seul
 * contenu. Le code est repris tel quel : il était écrit, traduit et éprouvé, et
 * seul son emplacement change.
 */

import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { Button, Input } from "@/components/ui";
import { useT } from "@/i18n";
import * as api from "@/lib/api/endpoints";
import { describeFields } from "@/lib/api/problem";
import type { Ed2kStatus } from "@/lib/api/client";

import { useEd2kError } from "./errors";

export function DaemonForm({ status }: { status: Ed2kStatus }) {
  const t = useT();
  const queryClient = useQueryClient();
  const describe = useEd2kError();

  const declared = status.daemon;

  const [host, setHost] = useState(declared?.host ?? "amuled");
  const [port, setPort] = useState(String(declared?.port ?? 4712));
  const [password, setPassword] = useState("");
  const [label, setLabel] = useState(declared?.label ?? "");

  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});

  async function save(event: React.FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setFields({});

    try {
      await api.setEd2kDaemon({
        host,
        port: Number(port),
        password,
        label: label || undefined,
      });
      // Le mot de passe n'est jamais réaffiché : le champ se vide, comme il se
      // videra au prochain chargement de la page.
      setPassword("");
      await queryClient.invalidateQueries({ queryKey: ["ed2k"] });
    } catch (err) {
      setError(describe(err));
      setFields(describeFields(err, t));
    } finally {
      setBusy(false);
    }
  }

  async function forget() {
    setBusy(true);
    setError(null);
    try {
      await api.forgetEd2kDaemon();
      setConfirming(false);
      setPassword("");
      await queryClient.invalidateQueries({ queryKey: ["ed2k"] });
    } catch (err) {
      setError(describe(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="rounded-lg border border-border bg-surface p-4">
      <h2 className="text-ui font-medium text-fg">{t("ed2k.daemon.title")}</h2>
      <p className="mt-1 max-w-prose text-meta text-muted">{t("ed2k.daemon.hint")}</p>

      <form onSubmit={(event) => void save(event)} className="mt-4 flex flex-col gap-3">
        <div className="flex flex-col gap-3 sm:flex-row">
          <Input
            name="host"
            label={t("ed2k.daemon.host")}
            value={host}
            onChange={(event) => setHost(event.target.value)}
            error={fields.host}
            autoComplete="off"
          />
          <div className="sm:w-40">
            <Input
              name="port"
              type="number"
              min={1}
              max={65535}
              label={t("ed2k.daemon.port")}
              value={port}
              onChange={(event) => setPort(event.target.value)}
              hint={t("ed2k.daemon.portHint")}
              error={fields.port}
            />
          </div>
        </div>

        <Input
          name="password"
          type="password"
          label={t("ed2k.daemon.password")}
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          hint={t("ed2k.daemon.passwordHint")}
          error={fields.password}
          autoComplete="new-password"
        />

        <Input
          name="label"
          label={t("ed2k.daemon.label")}
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          hint={t("ed2k.daemon.labelHint")}
        />

        {error && <p className="text-meta text-danger">{error}</p>}

        <div className="flex flex-wrap items-center gap-2">
          <Button type="submit" loading={busy} size="sm">
            {t("ed2k.daemon.save")}
          </Button>

          {declared && !confirming && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setConfirming(true)}
              disabled={busy}
            >
              {t("ed2k.daemon.forget")}
            </Button>
          )}
        </div>

        {confirming && (
          <div className="rounded-md border border-border bg-surface-sunken p-3">
            <p className="max-w-prose text-meta text-muted">{t("ed2k.daemon.forgetConfirm")}</p>
            <div className="mt-2 flex gap-2">
              <Button
                type="button"
                variant="danger"
                size="sm"
                loading={busy}
                onClick={() => void forget()}
              >
                {t("action.confirm")}
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setConfirming(false)}
              >
                {t("action.cancel")}
              </Button>
            </div>
          </div>
        )}
      </form>

      <dl className="mt-4 grid gap-2 border-t border-border pt-3 sm:grid-cols-2">
        <Fact term={t("ed2k.incoming")} hint={t("ed2k.incomingHint")}>
          <code className="text-meta text-fg">{status.incomingDir}</code>
        </Fact>

        {declared && (
          <Fact term={t("ed2k.daemon.lastSeen")}>
            <span className="text-meta text-fg">
              {declared.lastSeenAt
                ? new Date(declared.lastSeenAt).toLocaleString()
                : t("ed2k.daemon.never")}
            </span>
          </Fact>
        )}
      </dl>
    </section>
  );
}

export function DisabledNotice() {
  const t = useT();

  return (
    <section className="mt-4 rounded-lg border border-border bg-surface p-4">
      <h2 className="text-ui font-medium text-fg">{t("ed2k.disabledTitle")}</h2>
      <p className="mt-1 max-w-prose text-meta text-muted">{t("ed2k.disabledHint")}</p>
      <code className="mt-3 block w-fit rounded bg-surface-sunken px-2 py-1 text-meta text-fg">
        BOXINCLOUD_ED2K_ENABLED=true
      </code>
    </section>
  );
}

function Fact({
  term,
  hint,
  children,
}: {
  term: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="min-w-0">
      <dt className="text-micro uppercase tracking-wide text-subtle">{term}</dt>
      <dd className="mt-0.5 min-w-0 break-all">{children}</dd>
      {hint && <p className="mt-1 max-w-prose text-micro text-subtle">{hint}</p>}
    </div>
  );
}