package handlers

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/adonko3xBitters/boxincloud/server/internal/httpapi/problem"
	"github.com/adonko3xBitters/boxincloud/server/mobile"
)

/*
Distribution de l'application Android.

L'instance sert l'APK qu'elle embarque. Le téléphone qui scanne le code QR ne
parle donc jamais à GitHub, et une installation coupée d'Internet fonctionne
comme les autres — ce qui est la moindre des choses pour un projet
auto-hébergé qu'aucun magasin d'applications ne distribue.

Le bénéfice moins visible est plus durable : l'application et le serveur sont
construits ensemble, donc verrouillés sur la même version. Il n'existe aucun
couple app/serveur non testé.
*/
type Mobile struct {
	build BuildInfo
}

func NewMobile(build BuildInfo) *Mobile { return &Mobile{build: build} }

/*
APK sert l'application Android embarquée.

Les octets sortent du binaire, pas d'un stockage : ils ne peuvent ni manquer ni
diverger de la version du serveur. C'est ce qui garantit qu'un téléphone
installe l'application faite pour l'instance qu'il vient de scanner.
*/
func (h *Mobile) APK(w http.ResponseWriter, r *http.Request) {
	data, err := mobile.APK()
	if err != nil {
		problem.Write(w, r, problem.NotFound(
			"this build does not bundle the Android application"))
		return
	}

	w.Header().Set("Content-Type", "application/vnd.android.package-archive")

	// Un nom explicite : un fichier enregistré sous « android.apk » se perd
	// dans un dossier de téléchargements, et rien ne dit d'où il vient au
	// moment de l'installer.
	w.Header().Set("Content-Disposition", `attachment; filename="`+mobile.APKName+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	// Pas d'`immutable` : l'URL ne change pas d'une version à l'autre, et un
	// téléphone garderait l'ancien APK après une mise à jour de l'instance.
	w.Header().Set("Cache-Control", "no-cache")

	http.ServeContent(w, r, mobile.APKName, time.Time{}, bytes.NewReader(data))
}

// Info dit si cette instance embarque une application, et laquelle.
//
// La page de téléchargement s'en sert pour proposer l'installation ou expliquer
// son absence, plutôt que d'afficher un bouton qui répondrait 404.
func (h *Mobile) Info(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"android":   mobile.Available(),
		"version":   h.build.Version,
		"sizeBytes": mobile.Size(),
	})
}
