package collectors

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// InterfaceStats holds the subset of /proc/net/dev counters we expose.
type InterfaceStats struct {
	RxBytes   uint64
	RxPackets uint64
	TxBytes   uint64
	TxPackets uint64
}

// ReadInterfaceStats reads RX/TX counters for ifname from /proc/net/dev.
func ReadInterfaceStats(ifname string) (InterfaceStats, error) {
	return ParseInterfaceStatsFromPath("/proc/net/dev", ifname)
}

// ParseInterfaceStatsFromPath reads from an arbitrary path — used by tests
// to inject fixture files instead of /proc/net/dev.
//
// Column layout (0-indexed after splitting on whitespace, interface name stripped):
//   0:RxBytes 1:RxPkts 2:RxErrs 3:RxDrop ...
//   8:TxBytes 9:TxPkts ...
func ParseInterfaceStatsFromPath(path, ifname string) (InterfaceStats, error) {
	var s InterfaceStats

	f, err := os.Open(path)
	if err != nil {
		return s, fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Lines look like: "  upfgtp:  756   9  0  0 ..."
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		if name != ifname {
			continue
		}

		rest := strings.TrimSpace(line[colon+1:])
		fields := strings.Fields(rest)
		// Need at least 10 fields (indices 0-9)
		if len(fields) < 10 {
			return s, fmt.Errorf("unexpected /proc/net/dev format for %q: %d fields", ifname, len(fields))
		}

		if _, err := fmt.Sscanf(fields[0], "%d", &s.RxBytes); err != nil {
			return s, err
		}
		if _, err := fmt.Sscanf(fields[1], "%d", &s.RxPackets); err != nil {
			return s, err
		}
		if _, err := fmt.Sscanf(fields[8], "%d", &s.TxBytes); err != nil {
			return s, err
		}
		if _, err := fmt.Sscanf(fields[9], "%d", &s.TxPackets); err != nil {
			return s, err
		}
		return s, nil
	}

	return s, fmt.Errorf("interface %q not found in /proc/net/dev", ifname)
}
