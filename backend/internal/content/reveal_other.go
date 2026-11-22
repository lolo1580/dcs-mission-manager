//go:build !windows

package content

import "errors"

// revealPath is only meaningful on the Windows desktop the manager runs on.
func revealPath(string) error {
	return errors.New("ouverture de dossier non prise en charge sur cette plateforme")
}
