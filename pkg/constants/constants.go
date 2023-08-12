package constants

import "time"

const (
	// DefaultConsulAddress is the default address of the consul
	DefaultConsulAddress = "http://127.0.0.1:8500"

	// ConfigRootNS is the root config root namespace
	ConfigRootNS = "istio-system"
)

const (
	// DebounceAfter is the delay added to events to wait after a registry event for debouncing.
	// This will delay the push by at least this interval, plus the time getting subsequent events.
	// If no change is detected the push will happen, otherwise we'll keep delaying until things settle.
	DebounceAfter = 500 * time.Millisecond

	// DebounceMax is the maximum time to wait for events while debouncing.
	// Defaults to 10 seconds. If events keep showing up with no break for this time, we'll trigger a push.
	DebounceMax = 10 * time.Second

	// AerakiFieldManager is the FileldManager for Aeraki CRDs
	AerakiFieldManager = "Aeraki"

	// RegistryConsul is the registry category for Aeraki
	RegistryConsul = "consul"
)
