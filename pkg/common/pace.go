package common

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultSpecDuration = 10 * time.Second
)

type PaceSpec struct {
	Rps      int
	Duration time.Duration
}

func ParsePaceSpec(paceDescriptor string) ([]PaceSpec, error) {
	paceSpecArray := strings.Split(paceDescriptor, ",")
	pacerSpecs := make([]PaceSpec, 0)

	for _, p := range paceSpecArray {
		ps := strings.Split(p, ":")
		rps, err := strconv.Atoi(ps[0])
		if err != nil {
			return nil, fmt.Errorf("error while parsing pace spec %v: %v", ps, err)
		}
		duration := DefaultSpecDuration

		if len(ps) == 2 {
			durationSec, err := strconv.Atoi(ps[1])
			if err != nil {
				return nil, fmt.Errorf("error while parsing pace spec %v: %v", ps, err)
			}
			duration = time.Second * time.Duration(durationSec)
		}

		pacerSpecs = append(pacerSpecs, PaceSpec{rps, duration})
	}

	return pacerSpecs, nil
}
