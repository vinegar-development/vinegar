package dxvk

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/vinegarhq/vinegar/util"
	"github.com/vinegarhq/vinegar/wine"
)

func Setenv() {
	log.Printf("Enabling DXVK DLL overrides")

	os.Setenv("WINEDLLOVERRIDES", os.Getenv("WINEDLLOVERRIDES")+";d3d10core=n;d3d11=n;d3d9=n;dxgi=n")
}

func Fetch(dir string, url string) (string, error) {
	tokens := strings.Split(url, "/")
	file := tokens[len(tokens)-1]
	path := filepath.Join(dir, file)

	if file == "" {
		return "", fmt.Errorf("failed to get filename from url: %s", url)
	}

	if _, err := os.Stat(path); err == nil {
		log.Printf("DXVK is already downloaded (%s)", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if err := util.Download(url, path); err != nil {
		return "", fmt.Errorf("failed to download DXVK: %w", err)
	}

	return path, nil
}

func Remove(pfx *wine.Prefix) error {
	log.Println("Removing all overridden DXVK DLLs")

	for _, dir := range []string{"syswow64", "system32"} {
		for _, dll := range []string{"d3d9", "d3d10core", "d3d11", "dxgi"} {
			dllPath := filepath.Join("drive_c", "windows", dir, dll+".dll")

			log.Println("Removing DLL:", dllPath)

			if err := os.Remove(filepath.Join(pfx.Dir, dllPath)); err != nil {
				return err
			}
		}
	}

	log.Println("Restoring wineprefix DLLs")

	return pfx.Exec("wineboot", "-u")
}

func Extract(name string, pfx *wine.Prefix) error {
	log.Printf("Extracting DXVK")

	tarFile, err := os.Open(name)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	stream, err := gzip.NewReader(tarFile)
	if err != nil {
		return err
	}
	defer stream.Close()

	reader := tar.NewReader(stream)

	for {
		header, err := reader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		destDir, ok := map[string]string{
			"x64": filepath.Join(pfx.Dir, "drive_c", "windows", "system32"),
			"x32": filepath.Join(pfx.Dir, "drive_c", "windows", "syswow64"),
		}[filepath.Base(filepath.Dir(header.Name))]

		if !ok {
			log.Printf("Skipping DXVK unhandled file: %s", header.Name)
			continue
		}

		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return err
		}

		file, err := os.Create(filepath.Join(destDir, path.Base(header.Name)))
		if err != nil {
			return err
		}

		log.Printf("Extracting DLL %s", header.Name)

		if _, err = io.Copy(file, reader); err != nil {
			file.Close()
			return err
		}

		file.Close()
	}

	return nil
}
