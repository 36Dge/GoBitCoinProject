package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
)

type ThresholdState byte

// These constants are used to identify specific threshold states.
const (
	// ThresholdDefined is the first state for each deployment and is the
	// state for the genesis block has by definition for all deployments.
	ThresholdDefined ThresholdState = iota

	// ThresholdStarted is the state for a deployment once its start time
	// has been reached.
	ThresholdStarted

	// ThresholdLockedIn is the state for a deployment during the retarget
	// period which is after the ThresholdStarted state period and the
	// number of blocks that have voted for the deployment equal or exceed
	// the required number of votes for the deployment.
	ThresholdLockedIn

	// ThresholdActive is the state for a deployment for all blocks after a
	// retarget period in which the deployment was in the ThresholdLockedIn
	// state.
	ThresholdActive

	// ThresholdFailed is the state for a deployment once its expiration
	// time has been reached and it did not reach the ThresholdLockedIn
	// state.
	ThresholdFailed

	// numThresholdsStates is the maximum number of threshold states used in
	// tests.
	numThresholdsStates


)

var thresholdStateStrings = map[ThresholdState]string{
	ThresholdDefined:  "ThresholdDefined",
	ThresholdStarted:  "ThresholdStarted",
	ThresholdLockedIn: "ThresholdLockedIn",
	ThresholdActive:   "ThresholdActive",
	ThresholdFailed:   "ThresholdFailed",
}

//string returns the thersholdstate as a human-readable name.
func (t ThresholdState)String()string  {
	if s := thresholdStateStrings[t]; s != ""{
		return s
	}
	return fmt.Sprintf("unkonwn thresholdstate(%d)",int(t))
}



// thresholdConditionChecker provides a generic interface that is invoked to
// determine when a consensus rule change threshold should be changed.
type thresholdConditionChecker interface {
	// BeginTime returns the unix timestamp for the median block time after
	// which voting on a rule change starts (at the next window).
	BeginTime() uint64

	// EndTime returns the unix timestamp for the median block time after
	// which an attempted rule change fails if it has not already been
	// locked in or activated.
	EndTime() uint64

	// RuleChangeActivationThreshold is the number of blocks for which the
	// condition must be true in order to lock in a rule change.
	RuleChangeActivationThreshold() uint32

	// MinerConfirmationWindow is the number of blocks in each threshold
	// state retarget window.
	MinerConfirmationWindow() uint32

	// Condition returns whether or not the rule change activation condition
	// has been met.  This typically involves checking whether or not the
	// bit associated with the condition is set, but can be more complex as
	// needed.
	Condition(*blockNode) (bool, error)
}

//thresholdstatecache provides a type to cache the threshold states of each
//thershold window for s set of ids.
type thresholdStateCache struct {
	entries map[chainhash.Hash]ThresholdState
}

//lookup returns the threshold state associated with the given
//hash along with a boolean that indicates whether or not it is
//valid
func (c *thresholdStateCache) Lookup(hash *chainhash.Hash)(ThresholdState,bool){
	state,ok := c.entries[*hash]
	return state,ok
}

//update the cache to contain the provided hash to threshold state mappning
func (c *thresholdStateCache)Update(hash *chainhash.Hash,state ThresholdState)  {
	c.entries[*hash] = state
}

//newthresholdcahes returns a new array of caches to be used
//when calculating threshold states.
func newThresholdCaches(numCaches uint32) []thresholdStateCache {
	caches := make([]thresholdStateCache,numCaches)
	for i := 0; i< len(caches);i++{
		caches[i] = thresholdStateCache{entries: make(map[chainhash.Hash]ThresholdState)}
	}
	return  caches
}



// IsDeploymentActive returns true if the target deploymentID is active, and
// false otherwise.
//
// This function is safe for concurrent access.
func (b *BlockChain) IsDeploymentActive(deploymentID uint32) (bool, error) {
	b.chainLock.Lock()
	state, err := b.deploymentState(b.bestChain.Tip(), deploymentID)
	b.chainLock.Unlock()
	if err != nil {
		return false, err
	}

	return state == ThresholdActive, nil
}


