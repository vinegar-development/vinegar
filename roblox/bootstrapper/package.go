package bootstrapper

import (
	"fmt"
	"log"

	"github.com/vinegarhq/vinegar/util"
)

// Package is a representation of a Binary package.
type Package struct {
	Name     string
	Checksum string
	Size     int64
	ZipSize  int64
}

type Packages []Package

// Verify checks the named package source file against it's checksum
func (p *Package) Verify(src string) error {
	log.Printf("Verifying Package %s (%s)", p.Name, p.Checksum)

	if err := util.VerifyFileMD5(src, p.Checksum); err != nil {
		return fmt.Errorf("verify package %s: %w", p.Name, err)
	}

	return nil
}

// Download will download the package to the named dest destination
// directory with the given deployURL deploy mirror; if the package
// exists and has the correct checksum, it will return immediately.
//
// If downloading the package fails, it will attempt to re-download
// only once.
func (p *Package) Download(dest, deployURL string) error {
	if err := p.Verify(dest); err == nil {
		log.Printf("Package %s is already downloaded", p.Name)
		return nil
	}

	log.Printf("Downloading Package %s (%s)", p.Name, dest)

	if err := util.Download(deployURL+"-"+p.Name, dest); err == nil {
		return p.Verify(dest)
	}

	log.Printf("Failed to fetch package %s, retrying...", p.Name)

	if err := util.Download(deployURL+"-"+p.Name, dest); err != nil {
		return fmt.Errorf("download package %s: %w", p.Name, err)
	}

	return p.Verify(dest)
}

// Extract extracts the named package source file to a given destination directory
func (p *Package) Extract(src, dest string) error {
	if err := extract(src, dest); err != nil {
		return fmt.Errorf("extract package %s (%s): %w", p.Name, src, err)
	}

	log.Printf("Extracted Package %s (%s): %s", p.Name, p.Checksum, dest)
	return nil
}
