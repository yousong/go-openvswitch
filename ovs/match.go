// Copyright 2017 DigitalOcean.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ovs

import (
	"bytes"
	"encoding"
	"fmt"
	"net"
	"strings"
)

// Constants for use in Match names.
const (
	source      = "src"
	destination = "dst"

	sourceHardwareAddr = "sha"
	targetHardwareAddr = "tha"
	sourceProtocolAddr = "spa"
	targetProtocolAddr = "tpa"
)

// Constants of full Match names.
const (
	arpSHA   = "arp_sha"
	arpSPA   = "arp_spa"
	arpTHA   = "arp_tha"
	arpTPA   = "arp_tpa"
	conjID   = "conj_id"
	ctMark   = "ct_mark"
	ctState  = "ct_state"
	ctZone   = "ct_zone"
	dlSRC    = "dl_src"
	dlDST    = "dl_dst"
	dlType   = "dl_type"
	dlVLAN   = "dl_vlan"
	icmpType = "icmp_type"
	ipv6DST  = "ipv6_dst"
	ipv6SRC  = "ipv6_src"
	ndSLL    = "nd_sll"
	ndTLL    = "nd_tll"
	ndTarget = "nd_target"
	nwDST    = "nw_dst"
	nwProto  = "nw_proto"
	nwSRC    = "nw_src"
	tcpFlags = "tcp_flags"
	tpDST    = "tp_dst"
	tpSRC    = "tp_src"
	tunID    = "tun_id"
	vlanTCI  = "vlan_tci"
)

// A Match is a type which can be marshaled into an OpenFlow packet matching
// statement.  Matches can be used with Flows to match specific packet types
// and fields.
//
// Matches must also implement fmt.GoStringer for code generation purposes.
type Match interface {
	encoding.TextMarshaler
	fmt.GoStringer
}

// DataLinkSource matches packets with a source hardware address and optional
// wildcard mask matching addr.
func DataLinkSource(addr string) Match {
	return &dataLinkMatch{
		srcdst: source,
		addr:   addr,
	}
}

// DataLinkDestination matches packets with a destination hardware address
// and optional wildcard mask matching addr.
func DataLinkDestination(addr string) Match {
	return &dataLinkMatch{
		srcdst: destination,
		addr:   addr,
	}
}

const (
	// ethernetAddrLen is the length in bytes of an ethernet hardware address.
	ethernetAddrLen = 6
)

var _ Match = &dataLinkMatch{}

// A dataLinkMatch is a Match returned by DataLink{Source,Destination}.
type dataLinkMatch struct {
	srcdst string
	addr   string
}

// GoString implements Match.
func (m *dataLinkMatch) GoString() string {
	if m.srcdst == source {
		return fmt.Sprintf("ovs.DataLinkSource(%q)", m.addr)
	}

	return fmt.Sprintf("ovs.DataLinkDestination(%q)", m.addr)
}

// MarshalText implements Match.
func (m *dataLinkMatch) MarshalText() ([]byte, error) {
	// Split the string before possible wildcard mask
	ss := strings.SplitN(m.addr, "/", 2)

	hwAddr, err := net.ParseMAC(ss[0])
	if err != nil {
		return nil, err
	}
	if len(hwAddr) != ethernetAddrLen {
		return nil, fmt.Errorf("hardware address must be %d octets, but got %d",
			ethernetAddrLen, len(hwAddr))
	}

	if len(ss) == 1 {
		// Address has no wildcard mask
		return bprintf("dl_%s=%s", m.srcdst, hwAddr.String()), nil
	}

	wildcard, err := net.ParseMAC(ss[1])
	if err != nil {
		return nil, err
	}
	if len(wildcard) != ethernetAddrLen {
		return nil, fmt.Errorf("wildcard mask must be %d octets, but got %d",
			ethernetAddrLen, len(wildcard))
	}

	return bprintf("dl_%s=%s/%s", m.srcdst, hwAddr.String(), wildcard.String()), nil
}

// DataLinkType matches packets with the specified EtherType.
func DataLinkType(etherType uint16) Match {
	return &dataLinkTypeMatch{
		etherType: etherType,
	}
}

var _ Match = &dataLinkTypeMatch{}

// A dataLinkTypeMatch is a Match returned by DataLinkType.
type dataLinkTypeMatch struct {
	etherType uint16
}

// MarshalText implements Match.
func (m *dataLinkTypeMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=0x%04x", dlType, m.etherType), nil
}

// GoString implements Match.
func (m *dataLinkTypeMatch) GoString() string {
	return fmt.Sprintf("ovs.DataLinkType(0x%04x)", m.etherType)
}

// VLANNone is a special value which indicates that DataLinkVLAN should only
// match packets with no VLAN tag specified.
const VLANNone = 0xffff

// DataLinkVLAN matches packets with the specified VLAN ID matching vid.
func DataLinkVLAN(vid int) Match {
	return &dataLinkVLANMatch{
		vid: vid,
	}
}

var _ Match = &dataLinkVLANMatch{}

// A dataLinkVLANMatch is a Match returned by DataLinkVLAN.
type dataLinkVLANMatch struct {
	vid int
}

// MarshalText implements Match.
func (m *dataLinkVLANMatch) MarshalText() ([]byte, error) {
	if !validVLANVID(m.vid) && m.vid != VLANNone {
		return nil, errInvalidVLANVID
	}

	if m.vid == VLANNone {
		return bprintf("%s=0xffff", dlVLAN), nil
	}

	return bprintf("%s=%d", dlVLAN, m.vid), nil
}

// GoString implements Match.
func (m *dataLinkVLANMatch) GoString() string {
	if m.vid == VLANNone {
		return "ovs.DataLinkVLAN(ovs.VLANNone)"
	}

	return fmt.Sprintf("ovs.DataLinkVLAN(%d)", m.vid)
}

// NetworkSource matches packets with a source IPv4 address or IPv4 CIDR
// block matching ip.
func NetworkSource(ip string) Match {
	return &networkMatch{
		srcdst: source,
		ip:     ip,
	}
}

// NetworkDestination matches packets with a destination IPv4 address or
// IPv4 CIDR block matching ip.
func NetworkDestination(ip string) Match {
	return &networkMatch{
		srcdst: destination,
		ip:     ip,
	}
}

var _ Match = &networkMatch{}

// A networkMatch is a Match returned by Network{Source,Destination}.
type networkMatch struct {
	srcdst string
	ip     string
}

// MarshalText implements Match.
func (m *networkMatch) MarshalText() ([]byte, error) {
	return matchIPv4AddressOrCIDR(fmt.Sprintf("nw_%s", m.srcdst), m.ip)
}

// GoString implements Match.
func (m *networkMatch) GoString() string {
	if m.srcdst == source {
		return fmt.Sprintf("ovs.NetworkSource(%q)", m.ip)
	}

	return fmt.Sprintf("ovs.NetworkDestination(%q)", m.ip)
}

type regMatch struct {
	n    int
	val  uint32
	mask uint32
}

// RegMatch matches flows' associated register fields
func RegMatch(n int, val, mask uint32) Match {
	return &regMatch{
		n:    n,
		val:  val,
		mask: mask,
	}
}

// MarshalText implements Match.
func (m *regMatch) MarshalText() ([]byte, error) {
	if m.mask == 0 {
		return []byte{}, nil
	} else if m.mask == ^uint32(0) {
		if m.val == 0 {
			return bprintf("reg%d=0", m.n), nil
		}
		return bprintf("reg%d=0x%x", m.n, m.val), nil
	} else {
		return bprintf("reg%d=0x%x/0x%x", m.n, m.val, m.mask), nil
	}
}

// GoString implements Match.
func (m *regMatch) GoString() string {
	return fmt.Sprintf("ovs.RegMatch(%q, %q, %q)", m.n, m.val, m.mask)
}

// ConjunctionID matches flows that have matched all dimension of a conjunction
// inside of the openflow table.
func ConjunctionID(id uint32) Match {
	return &conjunctionIDMatch{
		id: id,
	}
}

// A conjunctionIDMatch is a Match returned by ConjunctionID
type conjunctionIDMatch struct {
	id uint32
}

// MarshalText implements Match.
func (m *conjunctionIDMatch) MarshalText() ([]byte, error) {
	return bprintf("conj_id=%v", m.id), nil
}

// GoString implements Match.
func (m *conjunctionIDMatch) GoString() string {
	return fmt.Sprintf("ovs.ConjunctionID(%v)", m.id)
}

// NetworkProtocol matches packets with the specified IP or IPv6 protocol
// number matching num.  For example, specifying 1 when a Flow's Protocol
// is IPv4 matches ICMP packets, or 58 when Protocol is IPv6 matches ICMPv6
// packets.
func NetworkProtocol(num uint8) Match {
	return &networkProtocolMatch{
		num: num,
	}
}

var _ Match = &networkProtocolMatch{}

// A networkProtocolMatch is a Match returned by NetworkProtocol.
type networkProtocolMatch struct {
	num uint8
}

// MarshalText implements Match.
func (m *networkProtocolMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", nwProto, m.num), nil
}

// GoString implements Match.
func (m *networkProtocolMatch) GoString() string {
	return fmt.Sprintf("ovs.NetworkProtocol(%d)", m.num)
}

// IPv6Source matches packets with a source IPv6 address or IPv6 CIDR
// block matching ip.
func IPv6Source(ip string) Match {
	return &ipv6Match{
		srcdst: source,
		ip:     ip,
	}
}

// IPv6Destination matches packets with a destination IPv6 address or
// IPv6 CIDR block matching ip.
func IPv6Destination(ip string) Match {
	return &ipv6Match{
		srcdst: destination,
		ip:     ip,
	}
}

var _ Match = &ipv6Match{}

// An ipv6Match is a Match returned by IPv6{Source,Destination}.
type ipv6Match struct {
	srcdst string
	ip     string
}

// MarshalText implements Match.
func (m *ipv6Match) MarshalText() ([]byte, error) {
	return matchIPv6AddressOrCIDR(fmt.Sprintf("ipv6_%s", m.srcdst), m.ip)
}

// GoString implements Match.
func (m *ipv6Match) GoString() string {
	if m.srcdst == source {
		return fmt.Sprintf("ovs.IPv6Source(%q)", m.ip)
	}

	return fmt.Sprintf("ovs.IPv6Destination(%q)", m.ip)
}

// ICMPType matches packets with the specified ICMP type matching typ.
func ICMPType(typ uint8) Match {
	return &icmpTypeMatch{
		typ: typ,
	}
}

var _ Match = &icmpTypeMatch{}

// An icmpTypeMatch is a Match returned by ICMPType.
type icmpTypeMatch struct {
	typ uint8
}

// MarshalText implements Match.
func (m *icmpTypeMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", icmpType, m.typ), nil
}

// GoString implements Match.
func (m *icmpTypeMatch) GoString() string {
	return fmt.Sprintf("ovs.ICMPType(%d)", m.typ)
}

// NeighborDiscoveryTarget matches packets with an IPv6 neighbor discovery target
// IPv6 address or IPv6 CIDR block matching ip.
func NeighborDiscoveryTarget(ip string) Match {
	return &neighborDiscoveryTargetMatch{
		ip: ip,
	}
}

var _ Match = &neighborDiscoveryTargetMatch{}

// A neighborDiscoveryTargetMatch is a Match returned by NeighborDiscoveryTarget.
type neighborDiscoveryTargetMatch struct {
	ip string
}

// MarshalText implements Match.
func (m *neighborDiscoveryTargetMatch) MarshalText() ([]byte, error) {
	return matchIPv6AddressOrCIDR(ndTarget, m.ip)
}

// GoString implements Match.
func (m *neighborDiscoveryTargetMatch) GoString() string {
	return fmt.Sprintf("ovs.NeighborDiscoveryTarget(%q)", m.ip)
}

// NeighborDiscoverySourceLinkLayer matches packets with an IPv6 neighbor
// solicitation source link-layer address matching addr.
func NeighborDiscoverySourceLinkLayer(addr net.HardwareAddr) Match {
	return &neighborDiscoveryLinkLayerMatch{
		srctgt: source,
		addr:   addr,
	}
}

// NeighborDiscoveryTargetLinkLayer matches packets with an IPv6 neighbor
// solicitation target link-layer address matching addr.
func NeighborDiscoveryTargetLinkLayer(addr net.HardwareAddr) Match {
	return &neighborDiscoveryLinkLayerMatch{
		srctgt: destination,
		addr:   addr,
	}
}

var _ Match = &neighborDiscoveryLinkLayerMatch{}

// A neighborDiscoveryLinkLayerMatch is a Match returned by DataLinkVLAN.
type neighborDiscoveryLinkLayerMatch struct {
	srctgt string
	addr   net.HardwareAddr
}

// MarshalText implements Match.
func (m *neighborDiscoveryLinkLayerMatch) MarshalText() ([]byte, error) {
	if m.srctgt == source {
		return matchEthernetHardwareAddress(ndSLL, m.addr)
	}

	return matchEthernetHardwareAddress(ndTLL, m.addr)
}

// GoString implements Match.
func (m *neighborDiscoveryLinkLayerMatch) GoString() string {
	syntax := hwAddrGoString(m.addr)

	if m.srctgt == source {
		return fmt.Sprintf("ovs.NeighborDiscoverySourceLinkLayer(%s)", syntax)
	}

	return fmt.Sprintf("ovs.NeighborDiscoveryTargetLinkLayer(%s)", syntax)
}

// ARPSourceHardwareAddress matches packets with an ARP source hardware address
// (SHA) matching addr.
func ARPSourceHardwareAddress(addr net.HardwareAddr) Match {
	return &arpHardwareAddressMatch{
		srctgt: source,
		addr:   addr,
	}
}

// ARPTargetHardwareAddress matches packets with an ARP target hardware address
// (THA) matching addr.
func ARPTargetHardwareAddress(addr net.HardwareAddr) Match {
	return &arpHardwareAddressMatch{
		srctgt: destination,
		addr:   addr,
	}
}

var _ Match = &arpHardwareAddressMatch{}

// An arpHardwareAddressMatch is a Match returned by ARP{Source,Target}HardwareAddress.
type arpHardwareAddressMatch struct {
	srctgt string
	addr   net.HardwareAddr
}

// MarshalText implements Match.
func (m *arpHardwareAddressMatch) MarshalText() ([]byte, error) {
	if m.srctgt == source {
		return matchEthernetHardwareAddress(arpSHA, m.addr)
	}

	return matchEthernetHardwareAddress(arpTHA, m.addr)
}

// GoString implements Match.
func (m *arpHardwareAddressMatch) GoString() string {
	syntax := hwAddrGoString(m.addr)

	if m.srctgt == source {
		return fmt.Sprintf("ovs.ARPSourceHardwareAddress(%s)", syntax)
	}

	return fmt.Sprintf("ovs.ARPTargetHardwareAddress(%s)", syntax)
}

// ARPSourceProtocolAddress matches packets with an ARP source protocol address
// (SPA) IPv4 address or IPv4 CIDR block matching addr.
func ARPSourceProtocolAddress(ip string) Match {
	return &arpProtocolAddressMatch{
		srctgt: source,
		ip:     ip,
	}
}

// ARPTargetProtocolAddress matches packets with an ARP target protocol address
// (TPA) IPv4 address or IPv4 CIDR block matching addr.
func ARPTargetProtocolAddress(ip string) Match {
	return &arpProtocolAddressMatch{
		srctgt: destination,
		ip:     ip,
	}
}

var _ Match = &arpProtocolAddressMatch{}

// An arpProtocolAddressMatch is a Match returned by ARP{Source,Target}ProtocolAddress.
type arpProtocolAddressMatch struct {
	srctgt string
	ip     string
}

// MarshalText implements Match.
func (m *arpProtocolAddressMatch) MarshalText() ([]byte, error) {
	if m.srctgt == source {
		return matchIPv4AddressOrCIDR(arpSPA, m.ip)
	}

	return matchIPv4AddressOrCIDR(arpTPA, m.ip)
}

// GoString implements Match.
func (m *arpProtocolAddressMatch) GoString() string {
	if m.srctgt == source {
		return fmt.Sprintf("ovs.ARPSourceProtocolAddress(%q)", m.ip)
	}

	return fmt.Sprintf("ovs.ARPTargetProtocolAddress(%q)", m.ip)
}

// TransportSourcePort matches packets with a transport layer (TCP/UDP) source
// port matching port.
func TransportSourcePort(port uint16) Match {
	return &transportPortMatch{
		srcdst: source,
		port:   port,
		mask:   0,
	}
}

// TransportDestinationPort matches packets with a transport layer (TCP/UDP)
// destination port matching port.
func TransportDestinationPort(port uint16) Match {
	return &transportPortMatch{
		srcdst: destination,
		port:   port,
		mask:   0,
	}
}

// TransportSourceMaskedPort matches packets with a transport layer (TCP/UDP)
// source port matching a masked port range.
func TransportSourceMaskedPort(port uint16, mask uint16) Match {
	return &transportPortMatch{
		srcdst: source,
		port:   port,
		mask:   mask,
	}
}

// TransportDestinationMaskedPort matches packets with a transport layer (TCP/UDP)
// destination port matching a masked port range.
func TransportDestinationMaskedPort(port uint16, mask uint16) Match {
	return &transportPortMatch{
		srcdst: destination,
		port:   port,
		mask:   mask,
	}
}

// A transportPortMatch is a Match returned by Transport{Source,Destination}Port.
type transportPortMatch struct {
	srcdst string
	port   uint16
	mask   uint16
}

var _ Match = &transportPortMatch{}

// A TransportPortRanger represents a port range that can be expressed as an array of bitwise matches.
type TransportPortRanger interface {
	MaskedPorts() ([]Match, error)
}

// A TransportPortRange reprsents the start and end values of a transport protocol port range.
type transportPortRange struct {
	srcdst    string
	startPort uint16
	endPort   uint16
}

// TransportDestinationPortRange represent a port range intended for a transport protocol destination port.
func TransportDestinationPortRange(startPort uint16, endPort uint16) TransportPortRanger {
	return &transportPortRange{
		srcdst:    destination,
		startPort: startPort,
		endPort:   endPort,
	}
}

// TransportSourcePortRange represent a port range intended for a transport protocol source port.
func TransportSourcePortRange(startPort uint16, endPort uint16) TransportPortRanger {
	return &transportPortRange{
		srcdst:    source,
		startPort: startPort,
		endPort:   endPort,
	}
}

// MaskedPorts returns the represented port ranges as an array of bitwise matches.
func (pr *transportPortRange) MaskedPorts() ([]Match, error) {
	portRange := PortRange{
		Start: pr.startPort,
		End:   pr.endPort,
	}

	bitRanges, err := portRange.BitwiseMatch()
	if err != nil {
		return nil, err
	}

	var ports []Match

	for _, br := range bitRanges {
		maskedPortRange := &transportPortMatch{
			srcdst: pr.srcdst,
			port:   br.Value,
			mask:   br.Mask,
		}
		ports = append(ports, maskedPortRange)
	}

	return ports, nil
}

// MarshalText implements Match.
func (m *transportPortMatch) MarshalText() ([]byte, error) {
	return matchTransportPort(m.srcdst, m.port, m.mask)
}

// GoString implements Match.
func (m *transportPortMatch) GoString() string {
	if m.mask > 0 {
		if m.srcdst == source {
			return fmt.Sprintf("ovs.TransportSourceMaskedPort(%#x, %#x)", m.port, m.mask)
		}

		return fmt.Sprintf("ovs.TransportDestinationMaskedPort(%#x, %#x)", m.port, m.mask)
	}

	if m.srcdst == source {
		return fmt.Sprintf("ovs.TransportSourcePort(%d)", m.port)
	}

	return fmt.Sprintf("ovs.TransportDestinationPort(%d)", m.port)
}

// A vlanTCIMatch is a Match returned by VLANTCI.
type vlanTCIMatch struct {
	tci  uint16
	mask uint16
}

// VLANTCI matches packets based on their VLAN tag control information, using
// the specified TCI and optional mask value.
func VLANTCI(tci, mask uint16) Match {
	return &vlanTCIMatch{
		tci:  tci,
		mask: mask,
	}
}

// MarshalText implements Match.
func (m *vlanTCIMatch) MarshalText() ([]byte, error) {
	if m.mask != 0 {
		return bprintf("%s=0x%04x/0x%04x", vlanTCI, m.tci, m.mask), nil
	}

	return bprintf("%s=0x%04x", vlanTCI, m.tci), nil
}

// GoString implements Match.
func (m *vlanTCIMatch) GoString() string {
	return fmt.Sprintf("ovs.VLANTCI(0x%04x, 0x%04x)", m.tci, m.mask)
}

// A connectionTrackingMarkMatch is a Match returned by ConnectionTrackingMark.
type connectionTrackingMarkMatch struct {
	mark uint32
	mask uint32
}

// ConnectionTrackingMark matches a metadata associated with a connection tracking entry
func ConnectionTrackingMark(mark, mask uint32) Match {
	return &connectionTrackingMarkMatch{
		mark: mark,
		mask: mask,
	}
}

// MarshalText implements Match.
func (m *connectionTrackingMarkMatch) MarshalText() ([]byte, error) {
	if m.mask != 0 {
		return bprintf("%s=0x%08x/0x%08x", ctMark, m.mark, m.mask), nil
	}

	return bprintf("%s=0x%08x", ctMark, m.mark), nil
}

// GoString implements Match.
func (m *connectionTrackingMarkMatch) GoString() string {
	return fmt.Sprintf("ovs.ConnectionTrackingMark(0x%08x, 0x%08x)", m.mark, m.mask)
}

// A connectionTrackingZoneMatch is a Match returned by ConnectionTrackingZone.
type connectionTrackingZoneMatch struct {
	zone uint16
}

// ConnectionTrackingZone is a mechanism to define separate connection tracking contexts.
func ConnectionTrackingZone(zone uint16) Match {
	return &connectionTrackingZoneMatch{
		zone: zone,
	}
}

// MarshalText implements Match.
func (m *connectionTrackingZoneMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%d", ctZone, m.zone), nil
}

// GoString implements Match.
func (m *connectionTrackingZoneMatch) GoString() string {
	return fmt.Sprintf("ovs.ConnectionTrackingZone(%d)", m.zone)
}

// ConnectionTrackingState matches packets using their connection state, when
// connection tracking is enabled on the host.  Use the SetState and UnsetState
// functions to populate the parameter list for this function.
func ConnectionTrackingState(state ...string) Match {
	return &connectionTrackingMatch{
		state: state,
	}
}

var _ Match = &connectionTrackingMatch{}

// A connectionTrackingMatch is a Match returned by ConnectionTrackingState.
type connectionTrackingMatch struct {
	state []string
}

// MarshalText implements Match.
func (m *connectionTrackingMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%s", ctState, strings.Join(m.state, "")), nil
}

// GoString implements Match.
func (m *connectionTrackingMatch) GoString() string {
	buf := bytes.NewBuffer(nil)
	for i, s := range m.state {
		_, _ = buf.WriteString(fmt.Sprintf("%q", s))

		if i != len(m.state)-1 {
			_, _ = buf.WriteString(", ")
		}
	}

	return fmt.Sprintf("ovs.ConnectionTrackingState(%s)", buf.String())
}

// CTState is a connection tracking state, which can be used with the
// ConnectionTrackingState function.
type CTState string

// List of common CTState constants available in OVS 2.5.  Reference the
// ovs-ofctl man-page for a description of each one.
const (
	CTStateNew         CTState = "new"
	CTStateEstablished CTState = "est"
	CTStateRelated     CTState = "rel"
	CTStateReply       CTState = "rpl"
	CTStateInvalid     CTState = "inv"
	CTStateTracked     CTState = "trk"
)

// SetState sets the specified CTState flag.  This helper should be used
// with ConnectionTrackingState.
func SetState(state CTState) string {
	return fmt.Sprintf("+%s", state)
}

// UnsetState unsets the specified CTState flag.  This helper should be used
// with ConnectionTrackingState.
func UnsetState(state CTState) string {
	return fmt.Sprintf("-%s", state)
}

// TCPFlags matches packets using their enabled TCP flags, when matching TCP
// flags on a TCP segment.   Use the SetTCPFlag and UnsetTCPFlag functions to
// populate the parameter list for this function.
func TCPFlags(flags ...string) Match {
	return &tcpFlagsMatch{
		flags: flags,
	}
}

var _ Match = &tcpFlagsMatch{}

// A tcpFlagsMatch is a Match returned by TCPFlags.
type tcpFlagsMatch struct {
	flags []string
}

// MarshalText implements Match.
func (m *tcpFlagsMatch) MarshalText() ([]byte, error) {
	return bprintf("%s=%s", tcpFlags, strings.Join(m.flags, "")), nil
}

// GoString implements Match.
func (m *tcpFlagsMatch) GoString() string {
	buf := bytes.NewBuffer(nil)
	for i, s := range m.flags {
		_, _ = buf.WriteString(fmt.Sprintf("%q", s))

		if i != len(m.flags)-1 {
			_, _ = buf.WriteString(", ")
		}
	}

	return fmt.Sprintf("ovs.TCPFlags(%s)", buf.String())
}

// TCPFlag represents a flag in the TCP header, which can be used with the
// TCPFlags function.
type TCPFlag string

// RFC 793 TCP Flags
const (
	TCPFlagURG TCPFlag = "urg"
	TCPFlagACK TCPFlag = "ack"
	TCPFlagPSH TCPFlag = "psh"
	TCPFlagRST TCPFlag = "rst"
	TCPFlagSYN TCPFlag = "syn"
	TCPFlagFIN TCPFlag = "fin"
)

// SetTCPFlag sets the specified TCPFlag.  This helper should be used
// with TCPFlags.
func SetTCPFlag(flag TCPFlag) string {
	return fmt.Sprintf("+%s", flag)
}

// UnsetTCPFlag unsets the specified TCPFlag.  This helper should be used
// with TCPFlags.
func UnsetTCPFlag(flag TCPFlag) string {
	return fmt.Sprintf("-%s", flag)
}

// TunnelID returns a Match that matches the given ID exactly.
func TunnelID(id uint64) Match {
	return &tunnelIDMatch{
		id:   id,
		mask: 0,
	}
}

// TunnelIDWithMask returns a Match with specified ID and mask.
func TunnelIDWithMask(id, mask uint64) Match {
	return &tunnelIDMatch{
		id:   id,
		mask: mask,
	}
}

var _ Match = &tunnelIDMatch{}

// A tunnelIDMatch is a Match against a tunnel ID.
type tunnelIDMatch struct {
	id   uint64
	mask uint64
}

// GoString implements Match.
func (m *tunnelIDMatch) GoString() string {
	if m.mask > 0 {
		return fmt.Sprintf("ovs.TunnelIDWithMask(%#x, %#x)", m.id, m.mask)
	}

	return fmt.Sprintf("ovs.TunnelID(%#x)", m.id)
}

// MarshalText implements Match.
func (m *tunnelIDMatch) MarshalText() ([]byte, error) {
	if m.mask == 0 {
		return bprintf("%s=%#x", tunID, m.id), nil
	}

	return bprintf("%s=%#x/%#x", tunID, m.id, m.mask), nil
}

// matchIPv4AddressOrCIDR attempts to create a Match using the specified key
// and input string, which could be interpreted as an IPv4 address or IPv4
// CIDR block.
func matchIPv4AddressOrCIDR(key string, ip string) ([]byte, error) {
	errInvalidIPv4 := fmt.Errorf("%q is not a valid IPv4 address or IPv4 CIDR block", ip)

	if ipAddr, _, err := net.ParseCIDR(ip); err == nil {
		if ipAddr.To4() == nil {
			return nil, errInvalidIPv4
		}

		return bprintf("%s=%s", key, ip), nil
	}

	if ipAddr := net.ParseIP(ip); ipAddr != nil {
		if ipAddr.To4() == nil {
			return nil, errInvalidIPv4
		}

		return bprintf("%s=%s", key, ipAddr.String()), nil
	}

	return nil, errInvalidIPv4
}

// matchIPv6AddressOrCIDR attempts to create a Match using the specified key
// and input string, which could be interpreted as an IPv6 address or IPv6
// CIDR block.
func matchIPv6AddressOrCIDR(key string, ip string) ([]byte, error) {
	errInvalidIPv6 := fmt.Errorf("%q is not a valid IPv6 address or IPv6 CIDR block", ip)

	if ipAddr, _, err := net.ParseCIDR(ip); err == nil {
		if ipAddr.To16() == nil || ipAddr.To4() != nil {
			return nil, errInvalidIPv6
		}

		return bprintf("%s=%s", key, ip), nil
	}

	if ipAddr := net.ParseIP(ip); ipAddr != nil {
		if ipAddr.To16() == nil || ipAddr.To4() != nil {
			return nil, errInvalidIPv6
		}

		return bprintf("%s=%s", key, ipAddr.String()), nil
	}

	return nil, errInvalidIPv6
}

// matchEthernetHardwareAddress attempts to create a Match using the specified
// key and input hardware address, which must be a 6-octet Ethernet hardware
// address.
func matchEthernetHardwareAddress(key string, addr net.HardwareAddr) ([]byte, error) {
	if len(addr) != ethernetAddrLen {
		return nil, fmt.Errorf("hardware address must be %d octets, but got %d",
			ethernetAddrLen, len(addr))
	}

	return bprintf("%s=%s", key, addr.String()), nil
}

// matchTransportPort is the common implementation for
// Transport{Source,Destination}Port.
func matchTransportPort(srcdst string, port uint16, mask uint16) ([]byte, error) {
	// No mask specified
	if mask == 0 {
		return bprintf("tp_%s=%d", srcdst, port), nil
	}

	return bprintf("tp_%s=0x%04x/0x%04x", srcdst, port, mask), nil
}
