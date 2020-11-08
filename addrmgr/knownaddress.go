package addrmgr

import (
	"BtcoinProject/wire"
	"time"
)

//knownaddress trachs information about a known newwork address that is used
//to determine how viable an address is.

type KnownAddress struct {
	na          *wire.NetAddress
	scrAddr     *wire.NetAddress
	attempts    int
	lastattempt time.Time
	lastsuccess time.Time
	tried       bool
	refs        int //reference count of new buckets.
}

// NetAddress returns the underlying wire.NetAddress associated with the
// known address.
func (ka *KnownAddress) NetAddress() *wire.NetAddress {
	return ka.na
}

// LastAttempt returns the last time the known address was attempted.
func (ka *KnownAddress) LastAttempt() time.Time {
	return ka.lastattempt
}

// Services returns the services supported by the peer with the known address.
func (ka *KnownAddress) Services() wire.ServiceFlag {
	return ka.na.Services
}

//chance returns the selection probability for a known address .the priority depends
//upon how recently the address has been seen.how recently it was last attempted
//and how offten atttmepts to connect to it have failed.
func (ka *KnownAddress) chance() float64 {
	now := time.Now()
	lastAttempt := now.Sub(ka.lastattempt)

	if lastAttempt < 0 {
		lastAttempt = 0
	}

	c := 1.0

	//very recent attempts are less likely to be retried.
	if lastAttempt < 10*time.Minute {
		c *= 0.01
	}

	//failed attempts depriortise.
	for i := ka.attempts; i > 0; i-- {
		c /= 1.5
	}

	return c

}


