//go:build linux

package sysinfo

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

const drmPath = "/sys/class/drm"

// Determines if a Card is 'embedded' or not, by checking
// if one of these displays belong to the card.
var embeddedDisplays = []string{"eDP", "LVDS"}

func getCards() (cs []Card) {
	drmCards, _ := filepath.Glob(path.Join(drmPath, "card[0-9]"))

	for i, c := range drmCards {
		dev, _ := filepath.EvalSymlinks(path.Join(c, "device"))
		driver, _ := filepath.EvalSymlinks(path.Join(dev, "driver"))
		driver = path.Base(driver)

		cs = append(cs, Card{
			Index:    i,
			Path:     c,
			Device:   dev,
			Driver:   driver,
			Embedded: embedded(c),
		})
	}

	return
}

// Walks over the drm path, and checks if there are any displays
// that are matched with the card path and contain any of embeddedDisplays
func embedded(cardPath string) (embed bool) {
	filepath.Walk(drmPath, func(p string, f os.FileInfo, err error) error {
		if !strings.HasPrefix(p, cardPath) {
			return nil
		}

		for _, hwd := range embeddedDisplays {
			if strings.Contains(p, hwd) {
				embed = true
			}
		}

		return nil
	})

	return
}
