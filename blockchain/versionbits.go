package blockchain

import (
	"BtcoinProject/chaincfg"
	"log"
	"math"
)

const (

	//vblengacyblockversion is the highest legacy block version before the
	//version bits shceme became active.
	vbLegacyBlockVersion = 4

	//vbtopbits defines the bits to set in the version to singnal that the
	//version bits shceme is being uesed.
	vbTopBits = 0x20000000

	//vbtopmask is the bitmask to use to determine whether or not the version
	//bits scheme is in use.
	vbTopMask = 0xe0000000

	//vbnumbits is the total number of bits availbale for use with the version bits cheme.
	vbNumBits = 29

	//unknownVernumTocheck is the number of pervious blocks to consider when
	//checking for a threshold of unkonwn block version for the purpose of warning
	//the user.
	unknownVerNumToCheck = 100

	//unkonwnVerWarnNum is the thershold of previous blocks that have an
	//unknown version to use for the purpose of warning the user.
	unkonwnVerWarnNum = unknownVerNumToCheck / 2
)

//bitconditionchecker provides a thresholdconditioncheckr which can be used to
//test whether or not a specific bit is set when it is not supposed to be
//according to the expected version based on the known deployments and the current
//state of the chain .this is usefun for detcting and warning about unkonwn rule
//activactions.
type bitConditionChecker struct {
	bit   uint32
	chain *BlockChain
}

// Ensure the bitConditionChecker type implements the thresholdConditionChecker
// interface.
var _ thresholdConditionChecker = bitConditionChecker{}

//bigintime returns the unix timestamp for the median block time after which
//voting on a rule change starts(at the next window)

//since this implementation checks for unknown rules ,it returns 0 so the rule
//is always treeated as active.

// This is part of the thresholdConditionChecker interface implementation.

func (c bitConditionChecker) BeginTime() uint {
	return 0
}

//endtime returns the unix timestamp for the medtian block time after which an
//attempted rule change fails if it has not already been locked in or activated.

//since this implementation chencks for unkonwn rules .it retusn the maximum possible
//timestamp so the rule is always treated as active.

func (c bitConditionChecker) EndTime() uint64 {
	return math.MaxUint64

}

//rulechangeactivationthreshold is the number of blocks for which the condition
//must be true in order ot lock in a rule change.
//this implemation retuns the value defined by the chain params the checker
//is associated with.
func (c bitConditionChecker) RuleChangeActivationThreshold() uint32 {
	return c.chain.chainParams.RuleChangeActivationThreshold
}

//minerconfiratminwindow is the number of blocks in each thershold state
//retarget window
func (c bitConditionChecker) MinerConfirmationWindow() uint32 {
	return c.chain.chainParams.MinerConfirmationWindow
}

//condition returns ture when the specific bit associated with the check is
//set and it is not supposed to be according to the expected version based on
//the konwn deplyments and the current state of the chain.
func (c bitConditionChecker) Condition(node *blockNode) (bool, error) {
	conditionMask := uint32(1) << c.bit
	version := uint32(node.version)
	if version&vbTopMask != vbTopBits {
		return false, nil
	}
	if version&conditionMask == 0 {
		return false, nil
	}

	expectedVersion, err := c.chain.calcNextBlockVersion(node.parent)
	if err != nil {
		return false, err
	}
	return uint32(expectedVersion)&conditionMask == 0, nil
}

//deploymentchecker provides a thersholdconditionchecker which can be used
//to test a specfic deployment rule.this is required for properly detecting
//and activating consensus rule change.
type deploymentChecker struct {
	deployment *chaincfg.ConsensusDeployment
	chain      *BlockChain
}

// Ensure the deploymentChecker type implements the thresholdConditionChecker
// interface.
var _ thresholdConditionChecker = deploymentChecker{}

// BeginTime returns the unix timestamp for the median block time after which
// voting on a rule change starts (at the next window).
//
// This implementation returns the value defined by the specific deployment the
// checker is associated with.
//
// This is part of the thresholdConditionChecker interface implementation.
func (c deploymentChecker) BeginTime() uint64 {
	return c.deployment.StartTime
}

// EndTime returns the unix timestamp for the median block time after which an
// attempted rule change fails if it has not already been locked in or
// activated.
//
// This implementation returns the value defined by the specific deployment the
// checker is associated with.
//
// This is part of the thresholdConditionChecker interface implementation.
func (c deploymentChecker) EndTime() uint64 {
	return c.deployment.ExpireTime
}

// RuleChangeActivationThreshold is the number of blocks for which the condition
// must be true in order to lock in a rule change.
//
// This implementation returns the value defined by the chain params the checker
// is associated with.
//
// This is part of the thresholdConditionChecker interface implementation.
func (c deploymentChecker) RuleChangeActivationThreshold() uint32 {
	return c.chain.chainParams.RuleChangeActivationThreshold
}

// MinerConfirmationWindow is the number of blocks in each threshold state
// retarget window.
//
// This implementation returns the value defined by the chain params the checker
// is associated with.
//
// This is part of the thresholdConditionChecker interface implementation.
func (c deploymentChecker) MinerConfirmationWindow() uint32 {
	return c.chain.chainParams.MinerConfirmationWindow
}

//condition returns true when the specific bit defined by the deployment
//associated with the checker is set.
func (c deploymentChecker) Condition(node *blockNode) (bool, error) {
	conditionMask := uint32(1) << c.deployment.BitNumber
	version := uint32(node.version)
	return (version&vbTopMask == vbTopBits) && (version&conditionMask != 0),
		nil
}

//calcnextblockversion calculates the expected version of the block after
//the passed previous block node based on the state of started and locked in
//rule change deployment
//this function differs from the exported ... in that the exported version
//uses the current best chain as the previous block node while this function
//acceipts any block node.
func (b *BlockChain) calcNextBlockVersion(prevNode *blockNode) (int32, error) {
	//set the appropriate bits for each acitvately defined rule deployment
	//that is either in the process of being voted on.or locked in for the
	//activaction at the next thershold window change.
	expectedVersion := uint32(vbTopBits)
	for id := 0; id < len(b.chainParams.Deployments); id++ {
		deployment := &b.chainParams.Deployments[id]
		cache := &b.deploymentCaches[id]
		checker := deploymentChecker{deployment: deployment, chain: b}
		state, err := b.thresholdState(prevNode, checker, cache)
		if err != nil {
			return 0, nil
		}
		if state == ThresholdStarted || state == ThresholdLockedIn {
			expectedVersion |= uint32(1) << deployment.BitNumber
		}
	}

	return int32(expectedVersion), nil
}

// CalcNextBlockVersion calculates the expected version of the block after the
// end of the current best chain based on the state of started and locked in
// rule change deployments.
//
// This function is safe for concurrent access.
func (b *BlockChain) CalcNextBlockVersion() (int32, error) {
	b.chainLock.Lock()
	version, err := b.calcNextBlockVersion(b.bestChain.Tip())
	b.chainLock.Unlock()
	return version, err
}

//warnunkonwnruleactivation displays a warning when any konwn new rule are
//either about to activate or have been activated. this will only happen once
//when new rules have been actiated and every block for those about to be activated.
func (b *BlockChain) warnUnkonwnRuleActivations(node *blockNode) error {

	//warn if any unkown new rules are either about to activate or have already
	//been activated .
	for bit := uint32(0); bit < vbNumBits; bit++ {
		checker := bitConditionChecker{bit: bit, chain: b}
		cache := &b.warningCaches[bit]
		state, err := b.thresholdState(node.parent, checker, cache)
		if err != nil {
			return err
		}

		switch state {

		case ThresholdActive:
			if !b.unknownRulesWarned {
				log.Warnf("unknow new rules activated (bit %d)",
					bit)
				b.unknownRulesWarned = true

			}

		case ThresholdLockedIn:
			window := int32(checker.MinerConfirmationWindow())
			activationHeight := window - (node.height % window)
			log.Warnf("Unknown new rules are about to activate in "+
				"%d blocks (bit %d)", activationHeight, bit)

		}

	}
	return nil
}

//warnunkonwnversons logs a warging if a high enough percentage of the last blocks have
//unexpected versions.
func (b *BlockChain) warnUnkonwVersions(node *blockNode) error {
	//nothing to do if always warened.
	if b.unknownRulesWarned {
		return nil
	}

	//warn if enough previous blocks have unexpected version.
	numUpgrated := uint32(0)
	for i := uint32(0); i < unknownVerNumToCheck && node != nil; i++ {
		expectedVersion, err := b.calcNextBlockVersion(node.parent)
		if err != nil {
			return err
		}
		if expectedVersion > vbLegacyBlockVersion && (node.version & ^expectedVersion) != 0 {
			numUpgrated++
		}
		node = node.parent
	}

	if numUpgrated > unkonwnVerWarnNum {
		log.Warn("Unknown block versions are being mined, so new " +
			"rules might be in effect.  Are you running the " +
			"latest version of the software?")
		b.unknownVersionsWarned = true
	}
	return nil

}

//over