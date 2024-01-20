package sysinfo

import (
	"fmt"
)

// Card is a representation of a system GPU
type Card struct {
	Index    int    // Internal Kernel index
	Path     string // Path to the drm card
	Device   string // Path to the PCI device
	Driver   string // Base driver name
	Embedded bool   // Integrated display
}

func (c Card) String() string {
	return fmt.Sprintf("%d: %s", c.Index, c.Driver)
}
