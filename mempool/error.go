package mempool

import (
	"BtcoinProject/blockchain"
	"BtcoinProject/wire"
)

//RuleError标识规则冲突.
//用来表示
//由于许多验证之一，事务处理失败规则。
//调用方可以使用类型断言来确定失败是否是
//特别是由于违反规则，使用err字段访问
//基础错误，它将是txruleerror或
//区块链规则错误。
// 定义区块链规则错误
type RuleError struct {
	Err error
}

// 满足错误接口并打印人类可读的错误,实现系统的Error方法
func (e RuleError) Error() string {
	if e.Err == nil {
		return "<nil>"
	}
	return e.Err.Error()
}

//txRuleError标识规则违规.
//它用来表示
//由于许多验证之一，事务处理失败
//规则。调用方可以使用类型断言来确定失败是否是
//特别是由于违反规则，访问错误代码字段
//确定违反规则的具体原因。

type TxRuleError struct {
	RejectCode  wire.RejectCode // 与拒绝消息一起发送的代码
	Description string          // 人类可读的问题描述
}

//满足满足、错误接口打印人类可读的错误
func (e TxRuleError) Error() string {
	return e.Description
}

// TXRuleError使用给定的一组参数并返回一个封装它的RuelError
func txRuleError(c wire.RejectCode, desc string) RuleError {
	return RuleError{
		Err: TxRuleError{RejectCode: c, Description: desc},
	}
}

// ChanRuleError返回一个封装给定的规则错误
// 区块链规则错误
func chainRuleError(chainErr blockchain.RuleError) RuleError {
	return RuleError{
		Err: chainErr,
	}

}

// ExtractRejectCode尝试返回给定错误的相关拒绝代码
// 通过检查已知类型的错误。如果一个代码已成功提取
func extractRejectCode(err error) (wire.RejectCode, bool) {
	// 从RuleError中提取基础错误
	if rerr, ok := err.(RuleError); ok {
		err = rerr.Err
	}

	switch err := err.(type) {

	case blockchain.RuleError:
		//将链错误转为拒绝代码
		var code wire.RejectCode
		switch err.ErrorCode {
		// 因重复而被拒绝
		case blockchain.ErrDuplicateBlock:
			code = wire.RejectDuplicate

		// 因版本过时而被拒绝
		case blockchain.ErrBlockVersionTooOld:
			code = wire.RejectObsolete

		// 由于检查点拒绝
		case blockchain.ErrCheckpointTimeTooOld:
			fallthrough
		case blockchain.ErrDifficultyTooLow:
			fallthrough
		case blockchain.ErrBadCheckpoint:
			fallthrough
		case blockchain.ErrForkTooOld:
			code = wire.RejectCheckpoint

		// 其他事务一切都是由于块或事务无效
		default:
			code = wire.RejectInvalid
		}

		return code, true

	case TxRuleError:
		return err.RejectCode, true
	case nil:
		return wire.RejectInvalid, false

	}

	return wire.RejectInvalid, false

}

//errtorejecterr检查错误的基础类型并返回拒绝
//适合在Wire.msgreject消息中发送的代码和字符串
func ErrToRejectErr(err error) (wire.RejectCode, string) {
	//如果可能的话，将拒绝代码与错误文本一起返回
	//从错误中提取
	rejectCode, found := extractRejectCode(err)
	if found {
		return rejectCode, err.Error()
	}
	//如果没有错误，则返回一般拒绝字符串。这真的
	//除非其他地方的代码没有设置错误，否则不应发生
	//应该是这样，但最好是安全的，并且只返回一个泛型
	//字符串，而不允许以下代码取消引用
	//害怕恐慌。

	if err == nil {

		return wire.RejectInvalid, "rejected"
	}
	//如果基础错误不是上述情况之一，则返回
	//WIRE.REJECTINVALID，包含一个被拒绝的通用字符串和错误
	//文本。

	return wire.RejectInvalid, "rejected:" + err.Error()

}


// 这个页面完结

