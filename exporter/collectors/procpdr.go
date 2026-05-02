package collectors

import (
	"fmt"
	"os"
	"strings"
)

// PDRCounters holds the counter fields read back from /proc/gtp5g/pdr.
type PDRCounters struct {
	ULPktCnt  uint64
	DLPktCnt  uint64
	ULByteCnt uint64
	DLByteCnt uint64
	ULDropCnt uint64
	DLDropCnt uint64
}

// ReadPDRCounters queries /proc/gtp5g/pdr for a specific PDR by writing
// "<ifname> <seid> <pdr_id>" then reading back the counters.
// Must be called sequentially — the proc interface is not thread-safe.
func ReadPDRCounters(ifname string, seid uint64, pdrID uint16) (PDRCounters, error) {
	var c PDRCounters

	query := fmt.Sprintf("%s %d %d", ifname, seid, pdrID)
	if err := os.WriteFile("/proc/gtp5g/pdr", []byte(query), 0644); err != nil {
		return c, fmt.Errorf("write /proc/gtp5g/pdr: %w", err)
	}

	data, err := os.ReadFile("/proc/gtp5g/pdr")
	if err != nil {
		return c, fmt.Errorf("read /proc/gtp5g/pdr: %w", err)
	}

	content := string(data)
	if strings.Contains(content, "does not exists") {
		return c, fmt.Errorf("PDR seid=%d id=%d not found in kernel", seid, pdrID)
	}

	c.ULDropCnt = parseU64Field(content, "UL Drop Count:")
	c.DLDropCnt = parseU64Field(content, "DL Drop Count:")
	c.ULPktCnt  = parseU64Field(content, "UL Packet Count:")
	c.DLPktCnt  = parseU64Field(content, "DL Packet Count:")
	c.ULByteCnt = parseU64Field(content, "UL Byte Count:")
	c.DLByteCnt = parseU64Field(content, "DL Byte Count:")

	return c, nil
}

// parseU64Field extracts a uint64 from lines like "\t UL Packet Count: 42\n".
func parseU64Field(content, label string) uint64 {
	idx := strings.Index(content, label)
	if idx < 0 {
		return 0
	}
	rest := strings.TrimSpace(content[idx+len(label):])
	var val uint64
	fmt.Sscanf(rest, "%d", &val)
	return val
}
