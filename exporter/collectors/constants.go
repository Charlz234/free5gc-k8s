package collectors

// Derived from ~/gtp5g/include/genl.h and genl_pdr.h
// Enum values are 0-indexed from the kernel source.

const (
	GTP5GFamilyName = "gtp5g"

	// gtp5g_cmd enum values
	GTP5GCmdGetPDR uint8 = 7  // ADD_PDR=1,ADD_FAR=2,ADD_QER=3,DEL_PDR=4,DEL_FAR=5,DEL_QER=6,GET_PDR=7
	GTP5GCmdGetFAR uint8 = 8
	GTP5GCmdGetQER uint8 = 9

	// gtp5g_device_attrs
	GTP5GAttrLink    uint16 = 1 // ifindex (u32)
	GTP5GAttrNetNsFd uint16 = 2

	// gtp5g_pdr_attrs (start at 3, device attrs occupy 1-2)
	GTP5GPDRAttrID         uint16 = 3
	GTP5GPDRAttrPrecedence uint16 = 4
	GTP5GPDRAttrPDI        uint16 = 5  // nested
	GTP5GPDRAttrOHR        uint16 = 6
	GTP5GPDRAttrFARID      uint16 = 7
	GTP5GPDRAttrRoleAddr   uint16 = 8
	GTP5GPDRAttrUnixSocket uint16 = 9
	GTP5GPDRAttrQERID      uint16 = 10
	GTP5GPDRAttrSEID       uint16 = 11
	GTP5GPDRAttrURRID      uint16 = 12

	// gtp5g_pdi_attrs (nested inside GTP5GPDRAttrPDI)
	GTP5GPDIAttrUEAddrIPv4 uint16 = 1
	GTP5GPDIAttrFTEID      uint16 = 2  // nested
	GTP5GPDIAttrSDFFilter  uint16 = 3
	GTP5GPDIAttrSrcIntf    uint16 = 4

	// gtp5g_f_teid_attrs (nested inside GTP5GPDIAttrFTEID)
	GTP5GFTEIDAttrITEID      uint16 = 1
	GTP5GFTEIDAttrGTPUAddrV4 uint16 = 2

	// PDR counter attrs — these are appended by the kernel's dump handler
	// after the standard PDR attrs. Derived from proc_gtp5g_pdr struct field
	// order and confirmed against free5gc/go-gtp5gnl source.
	// ul_drop_cnt, dl_drop_cnt, ul_pkt_cnt, dl_pkt_cnt, ul_byte_cnt, dl_byte_cnt
	// are exposed as additional nested attributes in the dump response.
	// The kernel sets them via pdr->ul_pkt_cnt etc. in gtp5g_genl_fill_pdr().
	// Attr IDs confirmed from go-gtp5gnl/lib/pdr.go in free5gc org.
	GTP5GPDRAttrULDropCnt  uint16 = 13
	GTP5GPDRAttrDLDropCnt  uint16 = 14
	GTP5GPDRAttrULPktCnt   uint16 = 15
	GTP5GPDRAttrDLPktCnt   uint16 = 16
	GTP5GPDRAttrULByteCnt  uint16 = 17
	GTP5GPDRAttrDLByteCnt  uint16 = 18
)
