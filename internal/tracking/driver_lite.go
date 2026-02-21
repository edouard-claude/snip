//go:build lite

package tracking

// DriverAvailable indicates whether the SQLite driver is compiled in.
// Lite builds exclude SQLite for faster startup and smaller binary.
const DriverAvailable = false
