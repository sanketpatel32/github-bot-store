package main

import (
	"fmt"
	"os"
	"time"
)

func sweepInterval() time.Duration {
	h := 24
	if v := os.Getenv("SWEEP_INTERVAL_HOURS"); v != "" {
		fmt.Sscanf(v, "%d", &h)
	}
	return time.Duration(h) * time.Hour
}
