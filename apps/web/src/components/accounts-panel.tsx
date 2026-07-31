"use client";

import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { buttonClass, cx } from "./ui";
import { ApiError } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { useT } from "@/i18n";
import { useCurrentUser } from "@/lib/auth";

/**
 * Administration des comptes.
 *
 * Un serveur familial ou partagé a besoin de plusieurs comptes : chacun sa
 * progression de lecture, ses favoris, et pour certains une bibliothèque
 * restreinte. Tout cela n'existait qu'en base, sans aucun moyen d'y toucher.
 */

export function AccountsPanel({ onClose }: { onClose: () => void }) {
  const t = useT();
  const { data: me } = useCurrentUser();
  const [selected, setSelected] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const accounts = useQuery({ queryKey: ["accounts"], queryFn: api.listAccounts });
  const list = accounts.data?.accounts ?? [];

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.stopPropagation();
        onClose();
      }
    }
    document.addEventListener("keydown", onKeyDown, true);
    return () => document.removeEventListener("keydown", onKeyDown, true);
  }, [onClose]);

  const current = useMemo(
    () => list.find((account) => account.id === selected) ?? null,
    [list, selected],
  );

  return (
    <div className="fixed inset-0 z-[60] grid place-items-center bg-[var(--overlay)] p-4">
      <div
        role="dialog"
        aria-modal="true"
        aria-label={t("accounts.title")}
        className="rise-in flex h-[80vh] w-full max-w-4xl overflow-hidden rounded-xl border border-border bg-surface shadow-2xl"
      >
        {/* Liste des comptes */}
        <div className="flex w-64 shrink-0 flex-col border-r border-border bg-surface-sunken">
          <div className="flex items-center justify-between border-b border-border px-3 py-2.5">
            <h2 className="text-ui font-semibold text-fg">Comptes</h2>
            <span className="text-meta tabular-nums text-subtle">{list.length}</span>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto p-1.5">
            {accounts.isLoading ? (
              <p className="px-2 py-3 text-meta text-subtle">{t("state.loading")}</p>
            ) : (
              list.map((account) => (
                <button
                  key={account.id}
                  onClick={() => {
                    setSelected(account.id);
                    setCreating(false);
                  }}
                  className={cx(
                    "pressable flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left",
                    selected === account.id && !creating
                      ? "bg-accent text-inverted"
                      : "text-muted hover:bg-surface-hover hover:text-fg",
                  )}
                >
                  <span className="grid size-7 shrink-0 place-items-center rounded-full bg-accent-subtle text-meta font-semibold text-accent-text">
                    {(account.displayName || account.username).charAt(0).toUpperCase()}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-ui">
                      {account.displayName || account.username}
                    </span>
                    <span
                      className={cx(
                        "block truncate text-meta",
                        selected === account.id && !creating ? "opacity-80" : "text-subtle",
                      )}
                    >
                      {account.role === "admin" ? "administrateur" : "lecteur"}
                      {account.restricted ? t("accounts.suffixRestricted") : ""}
                    </span>
                  </span>
                </button>
              ))
            )}
          </div>

          <div className="border-t border-border p-2">
            <button
              onClick={() => {
                setCreating(true);
                setSelected(null);
              }}
              className={cx(buttonClass("secondary", "sm"), "w-full")}
            >
              {t("accounts.new")}
            </button>
          </div>
        </div>

        {/* Fiche */}
        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex items-center justify-between border-b border-border px-4 py-2.5">
            <h3 className="text-ui font-semibold text-fg">
              {creating ? t("accounts.new") : (current?.username ?? t("accounts.pickOne"))}
            </h3>
            <button
              onClick={onClose}
              aria-label={t("action.close")}
              className="pressable grid size-8 place-items-center rounded text-subtle hover:bg-surface-hover hover:text-fg"
            >
              <svg viewBox="0 0 16 16" fill="none" className="size-4" aria-hidden="true">
                <path d="m4 4 8 8M12 4l-8 8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
              </svg>
            </button>
          </header>

          <div className="min-h-0 flex-1 overflow-y-auto p-4">
            {creating ? (
              <CreateForm
                onDone={(id) => {
                  setCreating(false);
                  setSelected(id);
                }}
              />
            ) : current ? (
              <AccountForm
                account={current}
                isSelf={current.id === me?.id}
                onDeleted={() => setSelected(null)}
              />
            ) : (
              <p className="py-12 text-center text-ui text-subtle">
                Choisissez un compte à gauche, ou créez-en un.
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}

// ─── Création ────────────────────────────────────────────────────────────────

function CreateForm({ onDone }: { onDone: (id: string) => void }) {
  const t = useT();
  const queryClient = useQueryClient();
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<"admin" | "user">("user");
  const [error, setError] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: () =>
      api.createAccount({
        username,
        password,
        role,
        email: email || undefined,
        displayName: displayName || undefined,
      }),
    onSuccess: async (account) => {
      await queryClient.invalidateQueries({ queryKey: ["accounts"] });
      onDone(account.id);
    },
    onError: (err) => setError(describe(err)),
  });

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        setError(null);
        create.mutate();
      }}
      className="flex max-w-md flex-col gap-3"
    >
      <TextField
        label={t("auth.username")}
        value={username}
        onChange={setUsername}
        required
        autoFocus
      />
      <TextField label={t("auth.displayName")} value={displayName} onChange={setDisplayName} />
      <TextField label={t("auth.email")} value={email} onChange={setEmail} type="email" />
      <TextField
        label={t("auth.password")}
        value={password}
        onChange={setPassword}
        type="password"
        required
        hint={t("accounts.passwordHint")}
      />

      <RoleField value={role} onChange={setRole} />

      {error && <ErrorNote>{error}</ErrorNote>}

      <button type="submit" disabled={create.isPending} className={buttonClass("primary", "md")}>
        {create.isPending ? t("storage.creating") : t("accounts.create")}
      </button>
    </form>
  );
}

// ─── Fiche d'un compte ───────────────────────────────────────────────────────

function AccountForm({
  account,
  isSelf,
  onDeleted,
}: {
  account: api.Account;
  isSelf: boolean;
  onDeleted: () => void;
}) {
  const t = useT();
  const queryClient = useQueryClient();
  const [displayName, setDisplayName] = useState(account.displayName ?? "");
  const [email, setEmail] = useState(account.email ?? "");
  const [role, setRole] = useState(account.role);
  const [restricted, setRestricted] = useState(account.restricted);
  const [maxAge, setMaxAge] = useState(account.maxAgeRating?.toString() ?? "");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  // Changer de compte doit repartir de ses valeurs, pas de celles du précédent.
  useEffect(() => {
    setDisplayName(account.displayName ?? "");
    setEmail(account.email ?? "");
    setRole(account.role);
    setRestricted(account.restricted);
    setMaxAge(account.maxAgeRating?.toString() ?? "");
    setPassword("");
    setError(null);
    setSaved(false);
  }, [account]);

  const save = useMutation({
    mutationFn: () =>
      api.updateAccount(account.id, {
        displayName,
        email,
        role,
        restricted,
        maxAgeRating: restricted && maxAge ? Number(maxAge) : undefined,
        password: password || undefined,
      }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["accounts"] });
      setPassword("");
      setSaved(true);
    },
    onError: (err) => setError(describe(err)),
  });

  const remove = useMutation({
    mutationFn: () => api.deleteAccount(account.id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["accounts"] });
      onDeleted();
    },
    onError: (err) => setError(describe(err)),
  });

  return (
    <div className="flex max-w-md flex-col gap-4">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          setError(null);
          setSaved(false);
          save.mutate();
        }}
        className="flex flex-col gap-3"
      >
        <TextField label="Nom affiché" value={displayName} onChange={setDisplayName} />
        <TextField label="Adresse e-mail" value={email} onChange={setEmail} type="email" />

        <RoleField
          value={role}
          onChange={setRole}
          disabled={isSelf}
          hint={
            isSelf
              ? t("accounts.selfRoleHint")
              : undefined
          }
        />

        <div className="rounded-md border border-border p-3">
          <label className="flex items-start gap-2.5">
            <input
              type="checkbox"
              checked={restricted}
              onChange={(e) => setRestricted(e.target.checked)}
              className="mt-0.5 size-4 accent-[var(--accent)]"
            />
            <span>
              <span className="block text-ui font-medium text-fg">{t("accounts.restricted")}</span>
              <span className="block text-meta leading-relaxed text-muted">
{t("accounts.restrictedHint")}
              </span>
            </span>
          </label>

          {restricted && (
            <label className="mt-3 flex items-center gap-2 pl-6.5">
              <span className="text-meta text-muted">{t("accounts.maxRating")}</span>
              <input
                type="number"
                min={0}
                max={21}
                value={maxAge}
                onChange={(e) => setMaxAge(e.target.value)}
                placeholder="12"
                className="h-8 w-20 rounded-md border border-border bg-surface px-2 text-ui tabular-nums text-fg"
              />
              <span className="text-meta text-subtle">{t("accounts.years")}</span>
            </label>
          )}
        </div>

        <TextField
          label={t("accounts.newPassword")}
          value={password}
          onChange={setPassword}
          type="password"
          hint={t("accounts.newPasswordHint")}
        />

        {error && <ErrorNote>{error}</ErrorNote>}
        {saved && !error && (
          <p className="rounded-md border border-success/40 bg-success/10 px-3 py-2 text-meta text-success">
            Enregistré.
          </p>
        )}

        <button type="submit" disabled={save.isPending} className={buttonClass("primary", "md")}>
          {save.isPending ? t("storage.saving") : t("action.save")}
        </button>
      </form>

      <LibraryAccess userId={account.id} />

      <div className="border-t border-border pt-4">
        <button
          onClick={() => remove.mutate()}
          disabled={isSelf || remove.isPending}
          className={cx(
            "pressable rounded-md border border-danger/40 px-3 py-2 text-ui font-medium text-danger",
            "hover:bg-danger/10 disabled:opacity-40 disabled:hover:bg-transparent",
          )}
        >
          {remove.isPending ? t("accounts.disabling") : t("accounts.disable")}
        </button>
        <p className="mt-1.5 text-meta leading-relaxed text-subtle">
          {isSelf
            ? t("accounts.cannotDisableSelf")
            : t("accounts.disableHint")}
        </p>
      </div>
    </div>
  );
}

// ─── Accès aux bibliothèques ─────────────────────────────────────────────────

/**
 * Accès accordés à un compte.
 *
 * Le modèle mérite d'être dit à l'écran : une bibliothèque sans aucune
 * autorisation explicite est visible de tous. Le premier accès accordé la
 * referme donc pour les autres — sans cet avertissement, le geste passerait
 * pour une simple ouverture, alors qu'il restreint.
 */
function LibraryAccess({ userId }: { userId: string }) {
  const t = useT();
  const queryClient = useQueryClient();

  const libraries = useQuery({ queryKey: ["libraries"], queryFn: api.listLibraries });
  const grants = useQuery({
    queryKey: ["library-access", userId],
    queryFn: () => api.listAccountAccess(userId),
  });

  const byLibrary = useMemo(() => {
    const map = new Map<string, boolean>();
    for (const grant of grants.data?.grants ?? []) {
      map.set(grant.libraryId, grant.canWrite);
    }
    return map;
  }, [grants.data]);

  async function toggle(libraryId: string, granted: boolean, canWrite: boolean) {
    if (granted) {
      await api.grantLibraryAccess(libraryId, userId, canWrite);
    } else {
      await api.revokeLibraryAccess(libraryId, userId);
    }
    await queryClient.invalidateQueries({ queryKey: ["library-access", userId] });
    await queryClient.invalidateQueries({ queryKey: ["libraries"] });
  }

  const items = libraries.data?.libraries ?? [];

  return (
    <div className="border-t border-border pt-4">
      <h4 className="text-ui font-semibold text-fg">{t("accounts.libraryAccess")}</h4>
      <p className="mt-1 text-meta leading-relaxed text-muted">
{t("accounts.libraryAccessHint")}
      </p>

      <ul className="mt-2.5 flex flex-col gap-1">
        {items.map((library) => {
          const granted = byLibrary.has(library.id);
          const canWrite = byLibrary.get(library.id) ?? false;

          return (
            <li
              key={library.id}
              className="flex items-center gap-2 rounded-md border border-border px-3 py-2"
            >
              <input
                type="checkbox"
                checked={granted}
                onChange={(e) => void toggle(library.id, e.target.checked, canWrite)}
                aria-label={t("accounts.accessTo", { name: library.name })}
                className="size-4 accent-[var(--accent)]"
              />
              <span className="min-w-0 flex-1 truncate text-ui text-fg">{library.name}</span>

              <label
                className={cx(
                  "flex items-center gap-1.5 text-meta",
                  granted ? "text-muted" : "text-subtle opacity-50",
                )}
              >
                <input
                  type="checkbox"
                  checked={canWrite}
                  disabled={!granted}
                  onChange={(e) => void toggle(library.id, true, e.target.checked)}
                  className="size-3.5 accent-[var(--accent)]"
                />
                {t("share.write")}
              </label>
            </li>
          );
        })}
        {items.length === 0 && (
          <li className="text-meta text-subtle">{t("accounts.noLibrary")}</li>
        )}
      </ul>
    </div>
  );
}

// ─── Éléments ────────────────────────────────────────────────────────────────

function TextField({
  label,
  value,
  onChange,
  type = "text",
  required,
  autoFocus,
  hint,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  required?: boolean;
  autoFocus?: boolean;
  hint?: string;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="text-micro uppercase tracking-wide text-subtle">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        required={required}
        autoFocus={autoFocus}
        autoComplete={type === "password" ? "new-password" : "off"}
        className="h-9 rounded-md border border-border bg-surface px-2.5 text-ui text-fg"
      />
      {hint && <span className="text-meta leading-relaxed text-subtle">{hint}</span>}
    </label>
  );
}

function RoleField({
  value,
  onChange,
  disabled,
  hint,
}: {
  value: "admin" | "user";
  onChange: (value: "admin" | "user") => void;
  disabled?: boolean;
  hint?: string;
}) {
  const t = useT();
  return (
    <div className="flex flex-col gap-1">
      <span className="text-micro uppercase tracking-wide text-subtle">{t("auth.role")}</span>
      <div className="flex gap-1.5">
        {(["user", "admin"] as const).map((option) => (
          <button
            key={option}
            type="button"
            disabled={disabled}
            onClick={() => onChange(option)}
            aria-pressed={value === option}
            className={cx(
              "pressable rounded-md border px-3 py-1.5 text-ui font-medium disabled:opacity-50",
              value === option
                ? "border-accent bg-accent text-inverted"
                : "border-border text-muted hover:bg-surface-hover hover:text-fg",
            )}
          >
            {option === "admin" ? t("auth.roleAdmin") : t("accounts.roleReader")}
          </button>
        ))}
      </div>
      {hint && <span className="text-meta leading-relaxed text-subtle">{hint}</span>}
    </div>
  );
}

function ErrorNote({ children }: { children: React.ReactNode }) {
  return (
    <p className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-meta leading-relaxed text-danger">
      {children}
    </p>
  );
}

function describe(error: unknown): string {
  if (error instanceof ApiError) {
    const fields = error.problem?.errors;
    if (fields) {
      const first = Object.values(fields)[0];
      if (first) return first;
    }
    return error.problem?.detail ?? error.problem?.title ?? error.message;
  }
  return error instanceof Error ? error.message : "erreur inconnue";
}
