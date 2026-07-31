// Package mobile embarque l'application Android dans le binaire du serveur.
//
// L'instance sert donc elle-même l'APK : le téléphone qui scanne le code QR ne
// parle jamais à GitHub, et une installation sans accès à Internet fonctionne
// exactement comme les autres. C'est la contrepartie assumée d'un projet
// auto-hébergé qui n'est distribué par aucun magasin d'applications.
//
// L'autre bénéfice est moins visible et plus durable : l'application et le
// serveur sont construits ensemble, donc verrouillés sur la même version. Il
// n'existe aucun couple app/serveur non testé, et donc aucune dérive de
// compatibilité à gérer — ni côté protocole, ni côté message d'erreur.
//
// `make build-apk` remplit dist/. Seul le répertoire est versionné : le dépôt
// ne transporte pas soixante méga-octets de binaire, et un contributeur backend
// n'a pas besoin d'installer Flutter pour compiler le serveur.
package mobile

import (
	"embed"
	"errors"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// APKName est le nom sous lequel l'APK est embarqué et servi.
//
// Le même des deux côtés : un nom de fichier qui diffère entre l'embarquement
// et la route est une panne qui ne se voit qu'à l'exécution.
const APKName = "boxincloud.apk"

// ErrNotBundled signale un binaire compilé sans application mobile.
//
// Le cas est normal en développement backend et doit le rester. Il mérite
// pourtant d'être nommé : la page de téléchargement s'adapte plutôt que de
// proposer un lien qui répondrait 404.
var ErrNotBundled = errors.New("mobile : application non embarquée (lancer `make build-apk`)")

// APK retourne les octets de l'application Android.
func APK() ([]byte, error) {
	data, err := fs.ReadFile(embedded, "dist/"+APKName)
	if err != nil {
		return nil, ErrNotBundled
	}
	return data, nil
}

// Available indique si le binaire embarque une application.
func Available() bool {
	_, err := fs.Stat(embedded, "dist/"+APKName)
	return err == nil
}

// Size retourne la taille de l'APK, ou zéro s'il n'est pas embarqué.
//
// Annoncée avant le téléchargement : personne n'aime lancer sur un téléphone
// un transfert dont il ignore le poids, surtout en données mobiles.
func Size() int64 {
	info, err := fs.Stat(embedded, "dist/"+APKName)
	if err != nil {
		return 0
	}
	return info.Size()
}
