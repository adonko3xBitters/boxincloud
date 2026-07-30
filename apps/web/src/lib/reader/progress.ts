"use client";

/**
 * Sauvegarde de la progression pendant la lecture.
 *
 * Deux contraintes qui se contredisent : ne pas envoyer une requête à chaque
 * tourner de page — on en tourne vite — et ne rien perdre si l'onglet se ferme
 * brutalement.
 *
 * D'où l'anti-rebond pour le cas nominal, et un envoi forcé à la fermeture via
 * `sendBeacon`, seul mécanisme qu'un navigateur garantit d'exécuter alors que
 * la page disparaît.
 */

import { useCallback, useEffect, useRef } from "react";

import { API_BASE } from "@/lib/api/client";
import * as api from "@/lib/api/endpoints";
import { getTokens } from "@/lib/api/tokens";

/**
 * Délai avant enregistrement.
 *
 * 1,5 s : assez pour absorber une rafale de pages tournées rapidement, assez
 * court pour qu'une fermeture d'onglet ordinaire n'ait presque jamais rien à
 * rattraper.
 */
const DEBOUNCE_MS = 1500;

export function useProgressSaver(comicId: string, pageCount: number) {
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pending = useRef<number | null>(null);
  const lastSent = useRef<number | null>(null);

  const flush = useCallback(() => {
    const page = pending.current;
    if (page === null || page === lastSent.current) return;

    lastSent.current = page;
    pending.current = null;

    void api
      .updateProgress(comicId, { page, pageCount })
      .catch(() => {
        // Échec réseau : on oublie l'envoi pour que la prochaine position
        // reparte. La règle serveur « la page la plus avancée gagne » garantit
        // qu'aucune progression ne régresse à cause de cet oubli.
        lastSent.current = null;
      });
  }, [comicId, pageCount]);

  const record = useCallback(
    (page: number) => {
      pending.current = page;

      if (timer.current) clearTimeout(timer.current);
      timer.current = setTimeout(flush, DEBOUNCE_MS);
    },
    [flush],
  );

  /**
   * Envoi de dernière chance.
   *
   * `visibilitychange` plutôt que `beforeunload` : c'est le seul événement que
   * les navigateurs mobiles déclenchent de façon fiable quand l'application
   * passe en arrière-plan — et sur mobile, c'est ainsi qu'on quitte une page,
   * pas en la fermant.
   *
   * `sendBeacon` parce qu'un `fetch` ordinaire est annulé quand le document
   * disparaît.
   */
  useEffect(() => {
    function onHidden() {
      if (document.visibilityState !== "hidden") return;

      const page = pending.current;
      if (page === null || page === lastSent.current) return;

      const token = getTokens()?.accessToken;
      if (!token) return;

      // sendBeacon ne permet pas d'en-tête Authorization : le jeton passe en
      // paramètre, comme pour les images.
      const url = `${API_BASE}/sync?token=${encodeURIComponent(token)}`;
      const body = JSON.stringify({
        updates: [{ comicId, page, pageCount }],
      });

      const sent = navigator.sendBeacon(url, new Blob([body], { type: "application/json" }));
      if (sent) lastSent.current = page;
    }

    document.addEventListener("visibilitychange", onHidden);
    return () => document.removeEventListener("visibilitychange", onHidden);
  }, [comicId, pageCount]);

  // Quitter le lecteur par navigation interne : on enregistre immédiatement
  // plutôt que d'attendre l'expiration de l'anti-rebond.
  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
      flush();
    };
  }, [flush]);

  return record;
}
