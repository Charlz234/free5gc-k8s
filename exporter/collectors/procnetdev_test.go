package collectors_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Charlz234/free5gc-k8s/exporter/collectors"
)

// fixture matches exactly what core-5g returned:
// upfgtp:     756       9    0    0    0     0          0         0      756       9    4    0    0     0       0          0
const procNetDevFixture = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0: 1234567    8901    0    0    0     0          0         0   987654    5678    0    0    0     0       0          0
upfgtp:     756       9    0    0    0     0          0         0      756       9    4    0    0     0       0          0
`

func TestReadInterfaceStats(t *testing.T) {
	// Write fixture to a temp file, then monkey-patch the path.
	// Since ReadInterfaceStats opens /proc/net/dev directly, we test
	// the parsing logic by writing a real fixture and confirming output.
	// For full isolation, refactor ReadInterfaceStats to accept a path param.

	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "dev")
	if err := os.WriteFile(fixturePath, []byte(procNetDevFixture), 0644); err != nil {
		t.Fatal(err)
	}

	// Direct parse test using exported ParseInterfaceStatsFromPath
	stats, err := collectors.ParseInterfaceStatsFromPath(fixturePath, "upfgtp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stats.RxBytes != 756 {
		t.Errorf("RxBytes: got %d, want 756", stats.RxBytes)
	}
	if stats.RxPackets != 9 {
		t.Errorf("RxPackets: got %d, want 9", stats.RxPackets)
	}
	if stats.TxBytes != 756 {
		t.Errorf("TxBytes: got %d, want 756", stats.TxBytes)
	}
	if stats.TxPackets != 9 {
		t.Errorf("TxPackets: got %d, want 9", stats.TxPackets)
	}
}

func TestReadInterfaceStats_NotFound(t *testing.T) {
	dir := t.TempDir()
	fixturePath := filepath.Join(dir, "dev")
	os.WriteFile(fixturePath, []byte(procNetDevFixture), 0644)

	_, err := collectors.ParseInterfaceStatsFromPath(fixturePath, "doesnotexist")
	if err == nil {
		t.Fatal("expected error for missing interface, got nil")
	}
}
