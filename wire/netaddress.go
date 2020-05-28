package wire

import (
	"net"
	"time"
)

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
