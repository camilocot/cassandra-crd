package operator

import "time"

// Config is the configuration for the cassandra operator.
type Config struct {
	// ResyncPeriod is the resync period of the operator.
	ResyncPeriod time.Duration
}
