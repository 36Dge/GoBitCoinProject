package blockchain

import "fmt"

// DeploymentError标识一个错误，改错误指示部署ID为 uint32

type DeploymentError uint32

// 错误将断言错误作为可读字符串返回满足错误接口
func (e DeploymentError) Error() string {
	return fmt.Sprintf("deployment ID %d does not exist", uint32(e))
}

//断言错误标识指示内部代码一致性的错误
//问题，并应被视为一个关键和不可恢复的错误。

type AssertError string

//错误将断言错误作为可读字符串返回并满足
//错误接口

func (e AssertError) Error() string {
	return "assertion failed" + string(e)
}

// 错误代码标识一种错误
type ErrorCode int

// 这些常量用于标识特定的RuelError
const (
	// errDuplicateBlock指示已经具有相同哈希的块存在

	ErrDuplicateBlock ErrorCode = iota

	// errblocktoobig 指示序列化块大小超过最大允许大小
	ErrBlockTooBig

	// errblockweighttoohigh表示块的计算重量
	// 度量值超过了允许的最大值
	ErrBlockWeightTooHigh

	// errblockversiontooold 指示块版本太旧，并且
	// 由于大部分网路已经升级，不在接受更新的版本
	ErrBlockVersionTooOld

	// errInvalidTime表示传递的块中的时间具有精度
	// 超过一秒钟，链共识规则要求时间戳的最大精度为1秒
	ErrInvalidTime

	// errTimeTooOld表示时间早于每个链的最后几个共识规则
	// 或之前最近的检查点

	ErrTimeTooOld

	// errTimeToNew表示与之相比，未来时间太远
	// 当前时间
	ErrTimeTooNew

	// errDifficultyTooLow表示块的难度较低
	// 比最近一次检查点要求的难度大
	ErrDifficultyTooLow

	// errUnexpectedDifficulty表示指定的位与
	// 不对齐预期值，因为它与计算值不匹配
	// 根据难度重新获得的规则进行估价，或超出有效
	// 范围
	ErrUnexpectedDifficulty

	//低于要求的目标哈希
	ErrHighHash

	// errbadmerkleroot表示计算的merkle根不匹配预期值
	ErrBadMerkleRoot

	//errbadcheckpoint指示预期位于检查点高度和预期高度不匹配
	ErrBadCheckpoint

	// errforktooold表示块正视图分叉区块链（在最近的检查点之前）
	ErrForkTooOld

	// errcheckpointtimetooold指示块在最近的检查点(?)
	ErrCheckpointTimeTooOld

	// errnottransactions指示块没有一个交易，有效块必须至少
	//具有coinbase交易
	ErrNoTransactions

	//errnotxinputs表示事务没有任何输入。一个有效事务必须有一
	//个输入
	ErrNoTxInputs

	// errnotxoutputs表示事务没有任何输出。一个有效的事务必须
	// 至少有一个输出
	ErrNoTxOutputs

	// errtxtoobig表示事务超出了允许的最大大小
	// 序列化时
	ErrTxTooBig

	//errbadtxoutvalue表示事物的输出值在某些方面无效
	//例如超出范围
	ErrBadTxOutValue

	//errDuplicatetxinputs表示事务引用相同的多次输入
	ErrDuplicateTxInputs

	//errbadtxinputs表示事务输入在某种程度上无效
	//例如引用一个超出范围的输入或根本不引用一个
	ErrBadTxInput

	//errMisstxout表示由输入引用的输出要么不存在
	//要么已经用完了
	ErrMissingTxOut

	//errunfinalizedtx表示交易尚未完成
	//有效块只能包含已完成的事务
	ErrUnfinalizedTx

	// errduplaicatetx指示块包含相同的事务
	// （或至少两个哈希值相同的事务）。一个块
	// 只能包含唯一的事务
	ErrDuplicateTx

	//erroverwritetx表示块包含的事务与上一个尚未
	//完全完成的事务具有相同的哈希值
	ErrOverwriteTx

	//ErrUndulizeSpend表示事务正试图花费尚未达到
	//所需期限的CoinBase
	ErrImmatureSpend

	//errspendtoohigh表示事务正视图花费更多值大于其
	//所有输入的总和
	ErrSpendTooHigh

	//errbadfees表示块的总费用因以下原因无效
	//超过最大可能值
	ErrBadFees

	//errtoomanysingops表示签名操作的总数对于事务或块
	//超出了允许的最大限制
	ErrTooManySigOps

	//errFirsttxnotcoinbase表示块中的第一个事务不是coinbase
	//事务

	ErrFirstTxNotCoinbase

	//errMultipleIntercases表示一个块包含多个
	//CoinBase交易。
	ErrMultipleCoinbases


	//errbadCoinBaseScriptlen指示签名脚本的长度
	//因为CoinBase事务不在有效范围内。
	ErrBadCoinbaseScriptLen

	//errbadCoinBaseValue指示CoinBase值的数量
	//不符合补贴的预期价值加上所有费用的总和。
	ErrBadCoinbaseValue

	//errMissingCoinBaseHeight指示
	//块不是以序列化块高度作为开始
	//版本2和更高版本块需要。
	ErrMissingCoinbaseHeight


	//errbadCoinBaseHeight指示
	//版本2和更高版本块的CoinBase事务不匹配
	//预期值。
	ErrBadCoinbaseHeight

	//errscriptMalformed表示中的事务脚本格式不正确
	//某种方式。例如，它可能比允许的最大值长
	//长度或分析失败。
	ErrScriptMalformed

	//errscriptValidation指示执行事务的结果
	//脚本失败。错误包括执行脚本时的任何失败
	//这样的签名验证失败并在
	//堆栈。
	ErrScriptValidation

	//errUnexpectedWitness指示块包含事务
	//有证人数据，但没有证人承诺
	//CoinBase交易记录。
	ErrUnexpectedWitness

	//ErrInvalidWitnessCommitment表示一个街区的证人
	//
	ErrInvalidWitnessCommitment


	//错误见证承诺不匹配表示见证承诺
	//包含在块的CoinBase事务中与
	//人工计算的见证承诺。
	ErrWitnessCommitmentMismatch

	//errPreviousBlockUnknown表示上一个块未知。
	ErrPreviousBlockUnknown

	//errInvalidancestorBlock指示此块的祖先具有
	//验证已失败。
	ErrInvalidAncestorBlock

	//errPrevBlockNotBest指示块的上一个块不是
	//当前链尖。这不是块验证规则，但是必需的
	//对于通过getblocktemplate rpc提交的块建议。
	ErrPrevBlockNotBest

)

// 将错误代码映射为常量名，以便进行漂亮的打印
var errorCodeString = map[ErrorCode]string{
	ErrDuplicateBlock:            "ErrDuplicateBlock",
	ErrBlockTooBig:               "ErrBlockTooBig",
	ErrBlockVersionTooOld:        "ErrBlockVersionTooOld",
	ErrBlockWeightTooHigh:        "ErrBlockWeightTooHigh",
	ErrInvalidTime:               "ErrInvalidTime",
	ErrTimeTooOld:                "ErrTimeTooOld",
	ErrTimeTooNew:                "ErrTimeTooNew",
	ErrDifficultyTooLow:          "ErrDifficultyTooLow",
	ErrUnexpectedDifficulty:      "ErrUnexpectedDifficulty",
	ErrHighHash:                  "ErrHighHash",
	ErrBadMerkleRoot:             "ErrBadMerkleRoot",
	ErrBadCheckpoint:             "ErrBadCheckpoint",
	ErrForkTooOld:                "ErrForkTooOld",
	ErrCheckpointTimeTooOld:      "ErrCheckpointTimeTooOld",
	ErrNoTransactions:            "ErrNoTransactions",
	ErrNoTxInputs:                "ErrNoTxInputs",
	ErrNoTxOutputs:               "ErrNoTxOutputs",
	ErrTxTooBig:                  "ErrTxTooBig",
	ErrBadTxOutValue:             "ErrBadTxOutValue",
	ErrDuplicateTxInputs:         "ErrDuplicateTxInputs",
	ErrBadTxInput:                "ErrBadTxInput",
	ErrMissingTxOut:              "ErrMissingTxOut",
	ErrUnfinalizedTx:             "ErrUnfinalizedTx",
	ErrDuplicateTx:               "ErrDuplicateTx",
	ErrOverwriteTx:               "ErrOverwriteTx",
	ErrImmatureSpend:             "ErrImmatureSpend",
	ErrSpendTooHigh:              "ErrSpendTooHigh",
	ErrBadFees:                   "ErrBadFees",
	ErrTooManySigOps:             "ErrTooManySigOps",
	ErrFirstTxNotCoinbase:        "ErrFirstTxNotCoinbase",
	ErrMultipleCoinbases:         "ErrMultipleCoinbases",
	ErrBadCoinbaseScriptLen:      "ErrBadCoinbaseScriptLen",
	ErrBadCoinbaseValue:          "ErrBadCoinbaseValue",
	ErrMissingCoinbaseHeight:     "ErrMissingCoinbaseHeight",
	ErrBadCoinbaseHeight:         "ErrBadCoinbaseHeight",
	ErrScriptMalformed:           "ErrScriptMalformed",
	ErrScriptValidation:          "ErrScriptValidation",
	ErrUnexpectedWitness:         "ErrUnexpectedWitness",
	ErrInvalidWitnessCommitment:  "ErrInvalidWitnessCommitment",
	ErrWitnessCommitmentMismatch: "ErrWitnessCommitmentMismatch",
	ErrPreviousBlockUnknown:      "ErrPreviousBlockUnknown",
	ErrInvalidAncestorBlock:      "ErrInvalidAncestorBlock",
	ErrPrevBlockNotBest:          "ErrPrevBlockNotBest",


}




//RuleError标识规则冲突。用来表示
//由于许多验证之一，块或事务处理失败
//规则。调用方可以使用类型断言来确定失败是否是
//特别是由于违反规则，访问错误代码字段
//确定违反规则的具体原因。

type RuleError struct {
	ErrorCode   ErrorCode //描述错误的类型
	Description string    // 人类可读的问题描述
}

// 错误接口并打印人类可读的错误。
func (e RuleError) Error() string {
	return e.Description
}

// RuleError在给定一组参数的情况下创建RuleError
// 这两个参数就是按照结构体中的单元字段类型定义的。返回值就是一个结构体

func ruleError(c ErrorCode, desc string) RuleError {
	return RuleError{ErrorCode: c, Description: desc}
}
