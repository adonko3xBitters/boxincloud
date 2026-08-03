package scraper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/adonko3xBitters/boxincloud/server/internal/discovery"
	"github.com/adonko3xBitters/boxincloud/server/internal/platform/netguard"
)

/*
Couche réseau : miroirs, débit, délais.

# L'agent, et pourquoi il n'est pas configurable

boxincloud s'annonce, avec l'adresse de son dépôt. Ce n'est pas de la
courtoisie : c'est ce qui permet à l'administrateur d'un site de savoir QUI le
sollicite, et de nous écrire plutôt que de nous bloquer.

Un gabarit ne peut pas le changer. Autoriser un agent arbitraire, ce serait
livrer le moyen de se faire passer pour un navigateur, c'est-à-dire l'outil de
contournement d'un blocage — l'exact inverse de ce que ce paquet cherche à
faire. Un site qui refuse boxincloud a le droit de le refuser, et la réponse
correcte est de retirer le gabarit, pas de se déguiser.

# L'ordre des précautions

Le même à chaque requête, et il n'est pas indifférent :

 1. **cache** — une réponse déjà connue ne coûte ni requête ni attente ;
 2. **netguard** — l'adresse est-elle légitime, même après redirection ;
 3. **robots.txt** — le site l'autorise-t-il ;
 4. **débit** — attendre son tour ;
 5. **requête**, bornée en temps et en taille.

Le cache passe AVANT le limiteur pour la raison expliquée dans `metadata.go` :
faire attendre son tour à une réponse déjà en mémoire transformerait une
politesse en lenteur pour rien. Et robots.txt passe avant le limiteur parce
qu'une adresse interdite ne doit consommer aucun jeton — elle ne partira pas.
*/

// userAgent identifie l'instance auprès des sites interrogés.
const userAgent = "boxincloud (+https://github.com/adonko3xBitters/boxincloud)"

// maxRedirects borne les redirections d'une même requête.
const maxRedirects = 5

// page est une réponse lue.
type page struct {
	// url est l'adresse FINALE, après redirections. C'est contre elle que les
	// adresses relatives de la page doivent être résolues, et pas contre celle
	// qu'on a demandée.
	url  string
	body []byte
}

// fetcher exécute les requêtes d'un gabarit.
type fetcher struct {
	http     *http.Client
	throttle *discovery.Throttle
	memo     *discovery.Memo
	robots   *robots
	log      *slog.Logger
}

func newFetcher(deps Deps) *fetcher {
	client := deps.HTTP
	if client == nil {
		client = &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= maxRedirects {
					return errors.New("trop de redirections")
				}
				// L'adresse d'arrivée est contrôlée comme celle de départ :
				// sinon une redirection suffirait à contourner le garde-fou.
				return netguard.Check(req.URL.String())
			},
		}
	}

	f := &fetcher{
		http:     client,
		throttle: deps.Throttle,
		memo:     deps.Memo,
		log:      deps.Log,
	}
	f.robots = newRobots(f)
	return f
}

/*
request décrit une requête à composer, indépendamment de la base.

Séparée de la base précisément pour que le repli sur un miroir n'ait qu'à la
rejouer ailleurs. Composer l'URL entière d'avance obligerait à la décomposer
pour changer d'hôte, ce qui est le genre de manipulation de chaînes dont
personne ne sort indemne.
*/
type request struct {
	method  string
	path    string
	query   url.Values
	form    url.Values
	headers map[string]string
	/*
		auth porte le secret de la source, sous le nom d'en-tête que le gabarit
		a déclaré.

		Séparé de `headers` pour une raison de fond : le gabarit dit OÙ mettre
		la clé, la source dit LAQUELLE. Un gabarit est un fichier souvent
		versionné et parfois partagé ; y écrire une clé d'API reviendrait à la
		publier. Le secret vit chiffré en base, comme le mot de passe d'un
		catalogue OPDS.
	*/
	authHeader string
	authValue  string
}

/*
attempt essaie une requête sur chaque base, dans l'ordre, jusqu'à ce qu'une
réponde.

Le repli ne se déclenche que sur ce qui ressemble à une PANNE. Un 404 ou un 403
sont des réponses : le site a compris la demande et l'a refusée, et la reposer à
un miroir donnera la même chose au prix d'une requête. Un 500, un 429 ou une
connexion refusée sont des pannes, et c'est exactement le cas pour lequel un
miroir existe.

L'interdiction par robots.txt arrête tout : les miroirs d'un site publient la
même politique, et insister serait précisément ce que la politique interdit.
*/
func (f *fetcher) attempt(
	ctx context.Context, template *Compiled, bases []string, req request,
) (page, error) {
	var failures []string

	for _, base := range bases {
		target, err := compose(base, req)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s : %v", base, err))
			continue
		}

		got, err := f.fetch(ctx, template, target, req)
		if err == nil {
			return got, nil
		}

		if errors.Is(err, ErrDisallowed) || errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {
			return page{}, err
		}

		var refused permanent
		if errors.As(err, &refused) {
			// Réponse ferme du site : inutile de la redemander ailleurs.
			return page{}, err
		}

		failures = append(failures, fmt.Sprintf("%s : %v", hostOf(base), err))
		f.log.Info("miroir écarté",
			slog.String("template", template.ID),
			slog.String("mirror", base),
			slog.Any("err", err))
	}

	return page{}, fmt.Errorf("%w (%s) : %s",
		ErrAllMirrorsFailed, template.ID, strings.Join(failures, " ; "))
}

/*
permanent marque une réponse qu'il ne sert à rien de rejouer ailleurs.

Un type d'erreur plutôt qu'un code de retour supplémentaire : l'information doit
traverser `fetch` sans que chaque appelant intermédiaire ait à la transporter,
et `errors.As` la retrouve au bout.
*/
type permanent struct{ status string }

func (p permanent) Error() string { return "le site a répondu " + p.status }

// fetch exécute une requête sur une adresse précise.
func (f *fetcher) fetch(
	ctx context.Context, template *Compiled, target string, req request,
) (page, error) {
	if cached, ok := f.cached(template, target, req); ok {
		return cached, nil
	}

	if err := netguard.Check(target); err != nil {
		return page{}, fmt.Errorf("%w : %w", discovery.ErrInvalidSource, err)
	}

	allowed, err := f.robots.allows(ctx, template, target)
	if template.IgnoreRobots {
		// Journalisé à chaque requête, pas une fois au chargement : c'est en
		// lisant les journaux d'une instance qu'on doit pouvoir constater qu'une
		// source passe outre, sans avoir à retrouver sa configuration.
		f.log.Info("robots.txt ignoré pour cette source",
			slog.String("template", template.ID),
			slog.String("url", target),
			slog.Bool("aurait_été_refusé", err == nil && !allowed))
		allowed, err = true, nil
	}
	if err != nil {
		// Un robots.txt illisible n'interdit rien : la spécification traite
		// l'absence comme une autorisation, et faire l'inverse rendrait toute
		// panne réseau équivalente à un refus définitif.
		f.log.Debug("robots.txt indisponible",
			slog.String("template", template.ID), slog.Any("err", err))
	} else if !allowed {
		return page{}, fmt.Errorf("%w : %s", ErrDisallowed, target)
	}

	if f.throttle != nil {
		if err := f.throttle.Wait(ctx, bucketFor(hostOf(target))); err != nil {
			return page{}, err
		}
	}

	body, finalURL, err := f.do(ctx, template, target, req)
	if err != nil {
		return page{}, err
	}

	got := page{url: finalURL, body: body}
	f.store(template, target, req, got)
	return got, nil
}

func (f *fetcher) do(
	ctx context.Context, template *Compiled, target string, req request,
) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, template.Timeout())
	defer cancel()

	var payload io.Reader
	if req.method == http.MethodPost && len(req.form) > 0 {
		payload = strings.NewReader(req.form.Encode())
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.method, target, payload)
	if err != nil {
		return nil, "", fmt.Errorf("%w : %w", discovery.ErrInvalidSource, err)
	}

	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.5")
	httpReq.Header.Set("Accept-Language", "fr,en;q=0.8")
	if payload != nil {
		httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for name, value := range req.headers {
		httpReq.Header.Set(name, value)
	}
	if req.authHeader != "" && req.authValue != "" {
		httpReq.Header.Set(req.authHeader, req.authValue)
	}

	resp, err := f.http.Do(httpReq)
	if err != nil {
		return nil, "", fmt.Errorf("site injoignable : %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		if retryable(resp.StatusCode) {
			return nil, "", fmt.Errorf("le site a répondu %s", resp.Status)
		}
		return nil, "", permanent{status: resp.Status}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, template.Limits.MaxBytes))
	if err != nil {
		return nil, "", fmt.Errorf("lecture de la page : %w", err)
	}

	// L'adresse finale vient de la requête telle qu'elle a été exécutée : après
	// redirections, `resp.Request.URL` est la seule qui décrive d'où vient
	// réellement le document.
	finalURL := target
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return body, finalURL, nil
}

/*
retryable dit si un code de réponse justifie d'essayer un miroir.

429 en fait partie, ce qui peut surprendre puisque c'est un refus délibéré. Mais
il dit « pas depuis cette adresse, maintenant » : un autre miroir n'est pas
concerné, et le limiteur sortant se chargera d'espacer les suivantes. Attendre
le `Retry-After` sur place serait le bon geste pour une tâche de fond ; pas pour
une recherche fédérée, où l'utilisateur attend devant sa page.
*/
func retryable(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	}
	return status >= 500
}

// compose assemble l'adresse d'une requête sur une base donnée.
func compose(base string, req request) (string, error) {
	root, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", fmt.Errorf("base illisible : %w", err)
	}

	path, err := url.Parse(req.path)
	if err != nil {
		return "", fmt.Errorf("chemin illisible : %w", err)
	}

	target := root.ResolveReference(path)
	if len(req.query) > 0 {
		target.RawQuery = req.query.Encode()
	}
	return target.String(), nil
}

// ─── Cache ───────────────────────────────────────────────────────────────────

/*
Les réponses sont mémorisées, mais pas toutes.

Un POST ne l'est pas : la méthode dit qu'on soumet quelque chose, et même quand
un site s'en sert pour une simple recherche, la mémoriser ferait dépendre le
résultat d'un corps qui n'entre pas dans la clé.

La clé est l'adresse EXACTE, sans passer par `discovery.MemoKey` : celle-ci
normalise sa dernière composante comme un titre — ponctuation retirée, casse
repliée — ce qui est juste pour des mots cherchés et faux pour une URL, où
`/a/b` et `/a-b` se confondraient.
*/
func (f *fetcher) cached(template *Compiled, target string, req request) (page, bool) {
	// Une réponse obtenue avec un secret n'est pas mémorisée. Deux comptes
	// peuvent voir deux catalogues différents à la même adresse, et servir à
	// l'un ce que l'autre a reçu serait une fuite silencieuse.
	if f.memo == nil || req.method != http.MethodGet || req.authValue != "" {
		return page{}, false
	}
	raw, ok := f.memo.Get(cacheKey(template.ID, target))
	if !ok {
		return page{}, false
	}
	got, ok := raw.(page)
	return got, ok
}

func (f *fetcher) store(template *Compiled, target string, req request, got page) {
	if f.memo == nil || req.method != http.MethodGet || req.authValue != "" {
		return
	}
	f.memo.Put(cacheKey(template.ID, target), got)
}

// cacheKey préfixe par le gabarit, pour que retirer une source n'invalide que
// ses propres pages — voir Memo.Invalidate.
func cacheKey(templateID, target string) string {
	return "scraper:" + templateID + "\x00" + target
}

// Deps rassemble ce que le client partage avec le reste de la découverte.
//
// Le limiteur et le cache sont FOURNIS, jamais construits ici, et pour la même
// raison que dans `MetadataDeps` : un limiteur par instance laisserait passer
// autant de requêtes qu'on a construit d'objets.
type Deps struct {
	HTTP     *http.Client
	Throttle *discovery.Throttle
	Memo     *discovery.Memo
	Log      *slog.Logger
}

func (d Deps) withDefaults() Deps {
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	return d
}
