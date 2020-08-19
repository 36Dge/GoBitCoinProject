package blockchain

import (
	"BtcoinProject/chaincfg/chainhash"
	"fmt"
	"golang.org/x/text/cases"
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
func (t ThresholdState) String() string {
	if s := thresholdStateStrings[t]; s != "" {
		return s
	}
	return fmt.Sprintf("unkonwn thresholdstate(%d)", int(t))
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
func (c *thresholdStateCache) Lookup(hash *chainhash.Hash) (ThresholdState, bool) {
	state, ok := c.entries[*hash]
	return state, ok
}

//update the cache to contain the provided hash to threshold state mappning
func (c *thresholdStateCache) Update(hash *chainhash.Hash, state ThresholdState) {
	c.entries[*hash] = state
}

//newthresholdcahes returns a new array of caches to be used
//when calculating threshold states.
func newThresholdCaches(numCaches uint32) []thresholdStateCache {
	caches := make([]thresholdStateCache, numCaches)
	for i := 0; i < len(caches); i++ {
		caches[i] = thresholdStateCache{entries: make(map[chainhash.Hash]ThresholdState)}
	}
	return caches
}

//thresholdstate returns the current rule change threshold state for the
//the block after the given node and deploymentid. the cache is usedt to
//ensure the thershold sates for previous windows are only calculated once
//this function must be called with chain state lock held(for writes)
func (b *BlockChain) thresholdState(prevNode *blockNode, check thresholdConditionChecker, cache *thresholdStateCache) (ThresholdState, error) {
	//the threshold state for the window that contains the genesis block is
	//defined by definition.
	confirmationWindow := int32(checker.MinerConfirmationWindow())
	if prevNode == nil || (prevNode.height+1) < confirmationWindow {
		return ThresholdDefined, nil
	}

	//get the ancestor that is the last block of the previous confirmation
	//window in order ot get its threshold state. this can be done because
	//the state is the same for all blocks within a given window
	prevNode = prevNode.Ancestor(prevNode.height -
		(prevNode.height+1)%confirmationWindow)

	//iterate backwards through each of the previous confirmation windws
	//to find the moset recently cached threshold state.
	var neededStates []*blockNode
	for prevNode != nil {
		//nothing more to do is the state of the block is already
		//cached.
		if _, ok := cache.Lookup(&prevNode.hash); ok {
			break
		}

		//the start and expiration times are based on the median block
		//time. so cauculate it now.
		medianTime := prevNode.CalcPastMedianTime()

		//the stated is simply difined if the start time has not been
		//been reached yet.
		if uint64(medianTime.Unix()) < checker.BeginTime() {
			cache.Update(&prevNode.hash, ThresholdDefined)
			break
		}

		//add this node to the list of nodes that nedd the state calculted
		//and cached.
		neededStates = append(neededStates, prevNode)

		//get the ancestor that is the last block of the previous confirmationn windoew.
		prevNode = prevNode.RelativeAncestor(confirmationWindow)
	}

	//start with the threshold state for the most recent confirmation
	//window that has a cahed state.
	state := ThresholdDefined
	if prevNode != nil {
		var ok bool
		state, ok = cache.Lookup(&prevNode.hash)
		if !ok {
			return ThresholdFailed, AssertError(fmt.Sprintf(
				"thresholdstate : cache lookup falied for %v", prevNode.hash))
		}
	}

	//since each threshold state depends on the state of the previous window
	//iterate starting from the oldest unkonwn window.
	for neededNum := len(neededStates) - 1; neededNum >= 0; neededNum-- {
		prevNode := neededStates[neededNum]

		switch state {
		case ThresholdDefined:
			//the deplyment of the rule change fails if it expires before it is
			//accepted and locked in.
			medianTime := prevNode.CalcPastMedianTime()
			medianTimeUnix := uint64(medianTime.Unix())
			if medianTimeUnix >= checker.EndTime() {
				state = ThresholdFailed
				break
			}

			//the state for the rule moves to the stated state
			//once its start time has been reached (and it has not
			//already expired per the above )
			if medianTimeUnix >= checker.BeginTime() {
				state = ThresholdStarted
			}

		case ThresholdStarted:
			//the deployment of the rule change fails if it expires
			//before it is accepted and locked in.
			medianTime := prevNode.CalcPastMedianTime()
			if uint64(medianTime.Unix()) >= check.EndTime() {
				state = ThresholdFailed
				break
			}

			//at this point ,the rule change it still being voted
			//on by the miners.so iterate backwares through the
			//confirmation window to count all of the votes in it.
			var count uint32
			countNode := prevNode
			for i := int32(0); i < confirmationWindow; i++ {
				condition, err := checker.Condition(countNode)
				if err != nil {
					return ThresholdFailed, err
				}

				if condition {
					count++
				}
				//get the previous block node.
				countNode = countNode.parent
			}

			//the state is locked in if the number of blocks in the
			//period that voted for the rule change meets the activation
			//threshold
			if count >= checker.RuleChangeActivationThreshold() {
				state = ThresholdLockedIn
			}

		case ThresholdLockedIn:
			//the new rule becomes active when its previous state
			//was locked in.
			state = ThresholdActive
			//nothing to do it the previous state is active or faild since
			//they are both termianl states.
		case ThresholdActive:
		case ThresholdFailed:
		}

		//updated the cache to avoid recalculating the state in the furture
		cache.Update(&prevNode.hash, state)

	}
	return state, nil

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
