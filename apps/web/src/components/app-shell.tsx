"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { BrandLockup } from "./brand";
import { Button, Spinner, cx } from "./ui";
import { useCurrentUser, useLogout, useRequireAuth } from "@/lib/auth";

/**
 * Coquille des pages authentifiées : en-tête, navigation, recherche.
 *
 * La navigation est en haut plutôt que dans une barre latérale : une grille de
 * couvertures gagne à occuper toute la largeur, et une colonne fixe de 260 px
 * coûte une couverture par rangée sur un écran d'ordinateur portable.
 */
export function AppShell({ children }: { children: React.ReactNode }) {
  const authenticated = useRequireAuth();

  if (!authenticated) {
    return (
      <div className="grid min-h-dvh place-items-center">
        <Spinner className="size-6 text-muted" />
      </div>
    );
  }

  return (
    <div className="min-h-dvh">
      <Header />
      <main className="mx-auto w-full max-w-[var(--layout-content-max)] px-4 py-6 sm:px-6 lg:px-8">
        {children}
      </main>
    </div>
  );
}

function Header() {
  const pathname = usePathname();

  const links = [
    { href: "/", label: "Accueil" },
    { href: "/library", label: "Bibliothèque" },
    { href: "/series", label: "Séries" },
  ];

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/85 backdrop-blur-md">
      <div className="mx-auto flex h-[var(--layout-header-height)] w-full max-w-[var(--layout-content-max)] items-center gap-4 px-4 sm:px-6 lg:px-8">
        <Link href="/" className="shrink-0">
          <BrandLockup />
        </Link>

        <nav className="hidden items-center gap-1 sm:flex" aria-label="Navigation principale">
          {links.map((link) => {
            const active =
              link.href === "/" ? pathname === "/" : pathname.startsWith(link.href);
            return (
              <Link
                key={link.href}
                href={link.href}
                aria-current={active ? "page" : undefined}
                className={cx(
                  "rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
                  active ? "bg-surface-hover text-fg" : "text-muted hover:text-fg",
                )}
              >
                {link.label}
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <SearchBox />
          <ThemeToggle />
          <UserMenu />
        </div>
      </div>

      {/* Sur mobile, la navigation passe sous l'en-tête plutôt que dans un
          menu : trois liens ne justifient pas un tiroir. */}
      <nav
        className="flex items-center gap-1 border-t border-border px-4 py-1.5 sm:hidden"
        aria-label="Navigation principale"
      >
        {links.map((link) => {
          const active = link.href === "/" ? pathname === "/" : pathname.startsWith(link.href);
          return (
            <Link
              key={link.href}
              href={link.href}
              aria-current={active ? "page" : undefined}
              className={cx(
                "rounded-md px-3 py-1.5 text-sm font-medium",
                active ? "bg-surface-hover text-fg" : "text-muted",
              )}
            >
              {link.label}
            </Link>
          );
        })}
      </nav>
    </header>
  );
}

/**
 * Champ de recherche.
 *
 * Navigue vers la page de résultats à la validation. La recherche instantanée
 * vit sur cette page : afficher un panneau de suggestions au-dessus d'une
 * grille de couvertures masquerait précisément ce qu'on cherche.
 */
function SearchBox() {
  const router = useRouter();
  const [value, setValue] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  // « / » met le focus dans la recherche, comme partout ailleurs.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== "/" || event.metaKey || event.ctrlKey) return;

      const target = event.target as HTMLElement | null;
      const typing =
        target?.tagName === "INPUT" ||
        target?.tagName === "TEXTAREA" ||
        target?.isContentEditable;
      if (typing) return;

      event.preventDefault();
      inputRef.current?.focus();
    }

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    const q = value.trim();
    if (q.length >= 2) router.push(`/search?q=${encodeURIComponent(q)}`);
  }

  return (
    <form onSubmit={onSubmit} className="relative hidden md:block" role="search">
      <svg
        viewBox="0 0 20 20"
        fill="none"
        className="pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-subtle"
        aria-hidden="true"
      >
        <circle cx="9" cy="9" r="5.5" stroke="currentColor" strokeWidth="1.5" />
        <path d="m13.5 13.5 3 3" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
      <input
        ref={inputRef}
        type="search"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="Rechercher…"
        aria-label="Rechercher dans la bibliothèque"
        className={cx(
          "h-9 w-56 rounded-md border border-border bg-surface pl-8 pr-8 text-sm",
          "placeholder:text-subtle focus:border-accent focus:w-72",
          "transition-[width,border-color] duration-[--motion-duration-normal]",
        )}
      />
      <kbd className="pointer-events-none absolute right-2 top-1/2 hidden -translate-y-1/2 rounded border border-border px-1 font-mono text-[10px] text-subtle lg:block">
        /
      </kbd>
    </form>
  );
}

function ThemeToggle() {
  const [theme, setTheme] = useState<"light" | "dark" | null>(null);

  useEffect(() => {
    const stored = localStorage.getItem("boxincloud.theme");
    setTheme(stored === "light" || stored === "dark" ? stored : null);
  }, []);

  function toggle() {
    // Trois états : suivre le système, forcer clair, forcer sombre. Le cycle
    // repasse par « système » pour qu'un utilisateur puisse y revenir.
    const next = theme === null ? "light" : theme === "light" ? "dark" : null;
    setTheme(next);

    if (next === null) {
      localStorage.removeItem("boxincloud.theme");
      delete document.documentElement.dataset.theme;
    } else {
      localStorage.setItem("boxincloud.theme", next);
      document.documentElement.dataset.theme = next;
    }
  }

  const label =
    theme === null ? "Thème : système" : theme === "light" ? "Thème : clair" : "Thème : sombre";

  return (
    <button
      onClick={toggle}
      title={label}
      aria-label={label}
      className="grid size-9 place-items-center rounded-md text-muted transition-colors hover:bg-surface-hover hover:text-fg"
    >
      {theme === "light" ? <SunIcon /> : theme === "dark" ? <MoonIcon /> : <AutoIcon />}
    </button>
  );
}

function UserMenu() {
  const { data: user } = useCurrentUser();
  const logout = useLogout();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;

    function onPointerDown(event: PointerEvent) {
      if (!ref.current?.contains(event.target as Node)) setOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }

    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const initial = (user?.displayName || user?.username || "?").charAt(0).toUpperCase();

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Menu du compte"
        className="grid size-9 place-items-center rounded-full bg-accent-subtle text-sm font-semibold text-accent-text transition-opacity hover:opacity-85"
      >
        {initial}
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 top-full z-50 mt-1 w-56 overflow-hidden rounded-lg border border-border bg-surface-raised shadow-[var(--shadow-lg)]"
        >
          <div className="border-b border-border px-3 py-2.5">
            <p className="truncate text-sm font-medium text-fg">
              {user?.displayName || user?.username || "…"}
            </p>
            <p className="truncate text-xs text-muted">
              {user?.role === "admin" ? "Administrateur" : "Utilisateur"}
            </p>
          </div>
          <div className="p-1">
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-start"
              onClick={() => {
                setOpen(false);
                void logout();
              }}
            >
              Se déconnecter
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}

// ─── Icônes ──────────────────────────────────────────────────────────────────

function SunIcon() {
  return (
    <svg viewBox="0 0 20 20" fill="none" className="size-[18px]" aria-hidden="true">
      <circle cx="10" cy="10" r="3.5" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M10 2v1.5M10 16.5V18M18 10h-1.5M3.5 10H2m12.7-4.7-1 1m-7.4 7.4-1 1m9.4 0-1-1M6.3 6.3l-1-1"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg viewBox="0 0 20 20" fill="none" className="size-[18px]" aria-hidden="true">
      <path
        d="M16 11.5A6.5 6.5 0 0 1 8.5 4a6.5 6.5 0 1 0 7.5 7.5Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function AutoIcon() {
  return (
    <svg viewBox="0 0 20 20" fill="none" className="size-[18px]" aria-hidden="true">
      <circle cx="10" cy="10" r="6.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 3.5a6.5 6.5 0 0 1 0 13V3.5Z" fill="currentColor" />
    </svg>
  );
}
