package bootstrapper

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/vinegarhq/vinegar/util"
)

// PackageManifest is a representation of a Binary version's packages
// DeployURL is required, as it is where the package manifest is fetched from.
type PackageManifest struct {
	*Deployment
	DeployURL string
	Packages
}

var (
	ErrInvalidPkgManifest      = errors.New("invalid package manifest given")
	ErrUnhandledPkgManifestVer = errors.New("unhandled package manifest version")
)

func channelPath(channel string) string {
	// Ensure that the channel is lowercased, since internally in
	// ClientSettings it will be lowercased, but not on the deploy mirror.
	channel = strings.ToLower(channel)

	// Roblox CDN only accepts no channel if its the default channel.
	// DefaultChannel is in all caps, and since channel is lowercased above,
	// make it lowercased too in this check.
	if channel == strings.ToLower(DefaultChannel) {
		return "/"
	}

	return "/channel/" + channel + "/"
}

// FetchPackageManifest retrieves a package manifest for the given binary deployment.
func FetchPackageManifest(d *Deployment) (PackageManifest, error) {
	cdn, err := CDN()
	if err != nil {
		return PackageManifest{}, err
	}
	durl := cdn + channelPath(d.Channel) + d.GUID
	url := durl + "-rbxPkgManifest.txt"

	log.Printf("Fetching manifest for %s (%s)", d.GUID, url)

	smanif, err := util.Body(url)
	if err != nil {
		return PackageManifest{}, fmt.Errorf("fetch %s package manifest: %w", d.GUID, err)
	}

	// Because the manifest ends with also a newline, it has to be removed.
	manif := strings.Split(smanif, "\r\n")
	if len(manif) > 0 && manif[len(manif)-1] == "" {
		manif = manif[:len(manif)-1]
	}

	pkgs, err := parsePackages(manif)
	if err != nil {
		return PackageManifest{}, err
	}

	return PackageManifest{
		Deployment: d,
		DeployURL:  durl,
		Packages:   pkgs,
	}, nil
}

func parsePackages(manifest []string) (Packages, error) {
	pkgs := make(Packages, 0)

	if (len(manifest)-1)%4 != 0 {
		return pkgs, ErrInvalidPkgManifest
	}

	if manifest[0] != "v0" {
		return pkgs, fmt.Errorf("%w: %s", ErrUnhandledPkgManifestVer, manifest[0])
	}

	for i := 1; i <= len(manifest)-4; i += 4 {
		if manifest[i] == "RobloxPlayerLauncher.exe" ||
			manifest[i] == "WebView2RuntimeInstaller.zip" {
			continue
		}

		zs, err := strconv.ParseInt(manifest[i+2], 10, 64)
		if err != nil {
			return pkgs, err
		}
		s, err := strconv.ParseInt(manifest[i+3], 10, 64)
		if err != nil {
			return pkgs, err
		}

		pkgs = append(pkgs, Package{
			Name:     manifest[i],
			Checksum: manifest[i+1],
			Size:     s,
			ZipSize:  zs,
		})
	}

	return pkgs, nil
}
