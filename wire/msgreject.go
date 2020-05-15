package wire

import "fmt"

//rejectcode表示一个数值，远程对等机通过该数值表示
//拒绝邮件的原因。
type RejectCode uint8

// 这些常量定义了各种支持的拒绝代码

const (
	RejectMalformed       RejectCode = 0x01
	RejectInvalid         RejectCode = 0x10
	RejectObsolete        RejectCode = 0x11
	RejectDuplicate       RejectCode = 0x12
	RejectNonstandard     RejectCode = 0x40
	RejectDust            RejectCode = 0x41
	RejectInsufficientFee RejectCode = 0x42
	RejectCheckpoint      RejectCode = 0x43
)

// 拒绝代码的映射返回字符串以进行漂亮的打印
var rejectCodeStrings = map[RejectCode]string{

	RejectMalformed:       "REJECT_MALFORMED",
	RejectInvalid:         "REJECT_INVALID",
	RejectObsolete:        "REJECT_OBSOLETE",
	RejectDuplicate:       "REJECT_DUPLICATE",
	RejectNonstandard:     "REJECT_NONSTANDARD",
	RejectDust:            "REJECT_DUST",
	RejectInsufficientFee: "REJECT_INSUFFICIENTFEE",
	RejectCheckpoint:      "REJECT_CHECKPOINT",
}

// 字符串以可读形式返回拒绝代码
func (code RejectCode) String() string {
	if s, ok := rejectCodeStrings[code]; ok {
		return s
	}
	return fmt.Sprintf("Unkonwn RejectCode(%d)", uint8(code))
}

// msgreject实现消息接口并表示比特币拒绝消息
// 在协议版本被拒绝之前，未添加此消息
type MsgReject struct {
	//cmd是被拒绝的消息的命令，例如
	//作为命令块或命令x。还可以从命令函数中获得消息的
	Cmd string

	//REJECTCODE是一个指示命令被拒绝原因的代码。它
	//在线路上编码为uint8。
	Code RejectCode

	//原因是一个人类可读的字符串，具有特定的详细信息（超过和
	//上面的拒绝代码）关于命令被拒绝的原因。
	Reason string

	//哈希标识被拒绝的特定块或事务
	//因此只应用msgblock和msgtx消息。

	Hash chainhash.Hash
}

// to do
