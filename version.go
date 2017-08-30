// Copyright 2017 Applatix, Inc.
package claudia

import "fmt"

// Version information set by link flags during build
var (
	Version        = "unknown"
	Revision       = "unknown"
	FullVersion    = fmt.Sprintf("%s-%s", Version, Revision)
	BuildDate      = "unknown"
	DisplayVersion = fmt.Sprintf("%s (Build Date: %s)", FullVersion, BuildDate)
)
