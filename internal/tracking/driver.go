//go:build !lite

package tracking

import _ "modernc.org/sqlite"

// DriverAvailable indicates whether the SQLite driver is compiled in.
const DriverAvailable = true
