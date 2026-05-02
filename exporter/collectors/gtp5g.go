package collectors

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
)

// PDR holds the per-PDR state returned from a GTP5G_CMD_GET_PDR dump.
type PDR struct {
	ID       uint16
	SEID     uint64
	UEIPV4   net.IP // from PDI nested attr
	TEID     uint32
	FARID    uint32

	ULPktCnt  uint64
	DLPktCnt  uint64
	ULByteCnt uint64
	DLByteCnt uint64
	ULDropCnt uint64
	DLDropCnt uint64
}

// DumpPDRs sends GTP5G_CMD_GET_PDR with NLM_F_DUMP over the upfgtp ifindex
// and returns all PDRs with their counters.
func DumpPDRs(ifindex uint32) ([]PDR, error) {
	conn, err := genetlink.Dial(nil)
	if err != nil {
		return nil, fmt.Errorf("genetlink dial: %w", err)
	}
	defer conn.Close()

	family, err := conn.GetFamily(GTP5GFamilyName)
	if err != nil {
		return nil, fmt.Errorf("get family %q: %w", GTP5GFamilyName, err)
	}

	// Build ifindex attribute (GTP5G_LINK = 1, u32 NLA)
	ifindexBytes := make([]byte, 4)
	binary.NativeEndian.PutUint32(ifindexBytes, ifindex)
	attrs, err := netlink.MarshalAttributes([]netlink.Attribute{
		{Type: uint16(GTP5GAttrLink), Data: ifindexBytes},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal attrs: %w", err)
	}

	req := genetlink.Message{
		Header: genetlink.Header{
			Command: GTP5GCmdGetPDR,
			Version: family.Version,
		},
		Data: attrs,
	}

	msgs, err := conn.Execute(req, family.ID, netlink.Request|netlink.Dump)
	if err != nil {
		return nil, fmt.Errorf("dump PDRs: %w", err)
	}

	pdrs := make([]PDR, 0, len(msgs))
	for _, msg := range msgs {
		pdr, err := parsePDR(msg.Data)
		if err != nil {
			// Log and continue — partial data is better than nothing
			continue
		}
		pdrs = append(pdrs, pdr)
	}
	return pdrs, nil
}

func parsePDR(data []byte) (PDR, error) {
	var pdr PDR

	attrs, err := netlink.UnmarshalAttributes(data)
	if err != nil {
		return pdr, fmt.Errorf("unmarshal PDR attrs: %w", err)
	}

	for _, attr := range attrs {
		switch uint16(attr.Type) & 0x1fff {
		case GTP5GPDRAttrID:
			if len(attr.Data) >= 2 {
				pdr.ID = binary.NativeEndian.Uint16(attr.Data[:2])
			}
		case GTP5GPDRAttrSEID:
			if len(attr.Data) >= 8 {
				pdr.SEID = binary.NativeEndian.Uint64(attr.Data[:8])
			}
		case GTP5GPDRAttrFARID:
			if len(attr.Data) >= 4 {
				pdr.FARID = binary.NativeEndian.Uint32(attr.Data[:4])
			}
		case GTP5GPDRAttrPDI:
			pdi, err := parsePDI(attr.Data)
			if err == nil {
				pdr.UEIPV4 = pdi.UEIPv4
				pdr.TEID = pdi.TEID
			}
		case GTP5GPDRAttrULPktCnt:
			if len(attr.Data) >= 8 {
				pdr.ULPktCnt = binary.NativeEndian.Uint64(attr.Data[:8])
			}
		case GTP5GPDRAttrDLPktCnt:
			if len(attr.Data) >= 8 {
				pdr.DLPktCnt = binary.NativeEndian.Uint64(attr.Data[:8])
			}
		case GTP5GPDRAttrULByteCnt:
			if len(attr.Data) >= 8 {
				pdr.ULByteCnt = binary.NativeEndian.Uint64(attr.Data[:8])
			}
		case GTP5GPDRAttrDLByteCnt:
			if len(attr.Data) >= 8 {
				pdr.DLByteCnt = binary.NativeEndian.Uint64(attr.Data[:8])
			}
		case GTP5GPDRAttrULDropCnt:
			if len(attr.Data) >= 8 {
				pdr.ULDropCnt = binary.NativeEndian.Uint64(attr.Data[:8])
			}
		case GTP5GPDRAttrDLDropCnt:
			if len(attr.Data) >= 8 {
				pdr.DLDropCnt = binary.NativeEndian.Uint64(attr.Data[:8])
			}
		}
	}

	return pdr, nil
}

type pdiResult struct {
	UEIPv4 net.IP
	TEID   uint32
}

func parsePDI(data []byte) (pdiResult, error) {
	var r pdiResult
	attrs, err := netlink.UnmarshalAttributes(data)
	if err != nil {
		return r, err
	}
	for _, attr := range attrs {
		switch uint16(attr.Type) & 0x1fff {
		case GTP5GPDIAttrUEAddrIPv4:
			if len(attr.Data) >= 4 {
				r.UEIPv4 = net.IP([]byte{attr.Data[0], attr.Data[1], attr.Data[2], attr.Data[3]})
			}
		case GTP5GPDIAttrFTEID:
			teid, err := parseFTEID(attr.Data)
			if err == nil {
				r.TEID = teid
			}
		}
	}
	return r, nil
}

func parseFTEID(data []byte) (uint32, error) {
	attrs, err := netlink.UnmarshalAttributes(data)
	if err != nil {
		return 0, err
	}
	for _, attr := range attrs {
		if uint16(attr.Type) == GTP5GFTEIDAttrITEID && len(attr.Data) >= 4 {
			return binary.NativeEndian.Uint32(attr.Data[:4]), nil
		}
	}
	return 0, nil
}
