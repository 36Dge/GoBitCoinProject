package wire

import (
	"net"
	"time"
)

//maxnetaddresspayload returns the max payload size for a bitcoin netaddress
//based on the protocol version
func maxNetAddressPayload(pver uint32) uint32 {
	//service 8 bytes + ip 16bytes + port 2 bytes.
	plen := uint32(26)

	//netaddresstimeversion added a timestamp field
	if pver >= NetAddressTimeVersion {
		//timestamp 4 bytes.
		plen += 4
	}
	return plen
}

//netaddrss defines information about a peer on the newwork including the
//time .it was last seen,the services it supports,its ip address,and port
type NetAddress struct {
	//last time the address was seen.this is unfortunately ,encoded as a
	//uint32 on the wire and therefore is limited to 2106,this field is
	//not present in the bitcoin version message nor was it added until
	// protocol version >= netaddresstimeversion
	Timestamp time.Time

	//bitfield which identifies the services supported by the address
	Service ServiceFlag

	//ip address of the peer
	IP net.IP

	//port the peer is using .this is encoded in big endian on wire
	//which differs from most enerything else
	Port uint16
}

//hasservice returns whether the specified service is supported by the address.
func (na *NetAddress) HasService(service ServiceFlag) bool {
	return na.Service&service == service
}

//addservice adds serivec as a supported service by the peer generating the
//message
func (na *NetAddress) AddService(service ServiceFlag) {
	na.Service |= service
}

//newnwtaddresstimestamp returns a new netaddress using the provided timestamp
//ip,port,and supported service ,the timestamp is rountded to single second
//precision .
func NewNetAddressTimestamp(
	timestamp time.Time,service ServiceFlag,ip net.IP,port uint16)  *NetAddress {

	//limit the timestamp to one second precision since the protocol does support better
	na := NetAddress{
		Timestamp: time.Unix(timestamp.Unix(),0),
		Service: service,
		IP: ip,
		Port: port,
	}
	return  &na
}


//newnetaddressipport returns a new netaddress using the provided ip,port ,and
//supported service with defaults for the remaining fields.
func NewNetAddressIPPort(ip net.IP, port uint16, services ServiceFlag) *NetAddress {
	return NewNetAddressTimestamp(time.Now(),services,ip,port)
}

//newnetaddress returns a new newtaddress using the provided tcp address and
//supported services with defaults for the remaining fields.
func NewNetAddress(addr *net.TCPAddr,services ServiceFlag) *NetAddress {
	return NewNetAddressIPPort(addr.IP,uint16(addr.Port),services)
}













