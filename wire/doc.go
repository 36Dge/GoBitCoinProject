

// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

/*
Package wire implements the bitcoin wire protocol.

For the complete details of the bitcoin protocol, see the official wiki entry
at https://en.bitcoin.it/wiki/Protocol_specification.  The following only serves
as a quick overview to provide information on how to use the package.

At a high level, this package provides support for marshalling and unmarshalling
supported bitcoin messages to and from the wire.  This package does not deal
with the specifics of message handling such as what to do when a message is
received.  This provides the caller with a high level of flexibility.

Bitcoin Message Overview

The bitcoin protocol consists of exchanging messages between peers.  Each
message is preceded by a header which identifies information about it such as
which bitcoin network it is a part of, its type, how big it is, and a checksum
to verify validity.  All encoding and decoding of message headers is handled by
this package.

To accomplish this, there is a generic interface for bitcoin messages named
Message which allows messages of any type to be read, written, or passed around
through channels, functions, etc.  In addition, concrete implementations of most
of the currently supported bitcoin messages are provided.  For these supported
messages, all of the details of marshalling and unmarshalling to and from the
wire using bitcoin encoding are handled so the caller doesn't have to concern
themselves with the specifics.

Message Interaction

The following provides a quick summary of how the bitcoin messages are intended
to interact with one another.  As stated above, these interactions are not
directly handled by this package.  For more in-depth details about the
appropriate interactions, see the official bitcoin protocol wiki entry at
https://en.bitcoin.it/wiki/Protocol_specification.

The initial handshake consists of two peers sending each other a version message
(MsgVersion) followed by responding with a verack message (MsgVerAck).  Both
peers use the information in the version message (MsgVersion) to negotiate
things such as protocol version and supported services with each other.  Once
the initial handshake is complete, the following chart indicates message
interactions in no particular order.

	Peer A Sends                          Peer B Responds
	----------------------------------------------------------------------------
	getaddr message (MsgGetAddr)          addr message (MsgAddr)
	getblocks message (MsgGetBlocks)      inv message (MsgInv)
	inv message (MsgInv)                  getdata message (MsgGetData)
	getdata message (MsgGetData)          block message (MsgBlock) -or-
	                                      tx message (MsgTx) -or-
	                                      notfound message (MsgNotFound)
	getheaders message (MsgGetHeaders)    headers message (MsgHeaders)
	ping message (MsgPing)                pong message (MsgHeaders)* -or-
	                                      (none -- Ability to send message is enough)

	NOTES:
	* The pong message was not added until later protocol versions as defined
	  in BIP0031.  The BIP0031Version constant can be used to detect a recent
	  enough protocol version for this purpose (version > BIP0031Version).

Common Parameters

There are several common parameters that arise when using this package to read
and write bitcoin messages.  The following sections provide a quick overview of
these parameters so the next sections can build on them.

Protocol Version

The protocol version should be negotiated with the remote peer at a higher
level than this package via the version (MsgVersion) message exchange, however,
this package provides the wire.ProtocolVersion constant which indicates the
latest protocol version this package supports and is typically the value to use
for all outbound connections before a potentially lower protocol version is
negotiated.

Bitcoin Network

The bitcoin network is a magic number which is used to identify the start of a
message and which bitcoin network the message applies to.  This package provides
the following constants:

	wire.MainNet
	wire.TestNet  (Regression test network)
	wire.TestNet3 (Test network version 3)
	wire.SimNet   (Simulation test network)

Determining Message Type

As discussed in the bitcoin message overview section, this package reads
and writes bitcoin messages using a generic interface named Message.  In
order to determine the actual concrete type of the message, use a type
switch or type assertion.  An example of a type switch follows:

	// Assumes msg is already a valid concrete message such as one created
	// via NewMsgVersion or read via ReadMessage.
	switch msg := msg.(type) {
	case *wire.MsgVersion:
		// The message is a pointer to a MsgVersion struct.
		fmt.Printf("Protocol version: %v", msg.ProtocolVersion)
	case *wire.MsgBlock:
		// The message is a pointer to a MsgBlock struct.
		fmt.Printf("Number of tx in block: %v", msg.Header.TxnCount)
	}

Reading Messages

In order to unmarshall bitcoin messages from the wire, use the ReadMessage
function.  It accepts any io.Reader, but typically this will be a net.Conn to
a remote node running a bitcoin peer.  Example syntax is:

	// Reads and validates the next bitcoin message from conn using the
	// protocol version pver and the bitcoin network btcnet.  The returns
	// are a wire.Message, a []byte which contains the unmarshalled
	// raw payload, and a possible error.
	msg, rawPayload, err := wire.ReadMessage(conn, pver, btcnet)
	if err != nil {
		// Log and handle the error
	}

Writing Messages

In order to marshall bitcoin messages to the wire, use the WriteMessage
function.  It accepts any io.Writer, but typically this will be a net.Conn to
a remote node running a bitcoin peer.  Example syntax to request addresses
from a remote peer is:

	// Create a new getaddr bitcoin message.
	msg := wire.NewMsgGetAddr()

	// Writes a bitcoin message msg to conn using the protocol version
	// pver, and the bitcoin network btcnet.  The return is a possible
	// error.
	err := wire.WriteMessage(conn, msg, pver, btcnet)
	if err != nil {
		// Log and handle the error
	}

Errors

Errors returned by this package are either the raw errors provided by underlying
calls to read/write from streams such as io.EOF, io.ErrUnexpectedEOF, and
io.ErrShortWrite, or of type wire.MessageError.  This allows the caller to
differentiate between general IO errors and malformed messages through type
assertions.

Bitcoin Improvement Proposals

This package includes spec changes outlined by the following BIPs:

	BIP0014 (https://github.com/bitcoin/bips/blob/master/bip-0014.mediawiki)
	BIP0031 (https://github.com/bitcoin/bips/blob/master/bip-0031.mediawiki)
	BIP0035 (https://github.com/bitcoin/bips/blob/master/bip-0035.mediawiki)
	BIP0037 (https://github.com/bitcoin/bips/blob/master/bip-0037.mediawiki)
	BIP0111	(https://github.com/bitcoin/bips/blob/master/bip-0111.mediawiki)
	BIP0130 (https://github.com/bitcoin/bips/blob/master/bip-0130.mediawiki)
	BIP0133 (https://github.com/bitcoin/bips/blob/master/bip-0133.mediawiki)
*/


/*
封装线实现比特币线协议。

有关比特币协议的完整详细信息，请参见官方wiki条目。
网址：https://en.bitcoin.it/wiki/protocol_-specification。以下仅适用于
作为快速概述，提供有关如何使用包的信息。

在较高的层次上，此包为编组和解组提供支持
支持比特币信息进出线。这个包裹不行
消息处理的细节，例如当消息
收到。这为调用者提供了高度的灵活性。

比特币信息概述

比特币协议包括在对等方之间交换消息。各
消息前面有一个标题，用于标识有关消息的信息，例如
它是哪个比特币网络的一部分，它的类型，它有多大，以及校验和
验证有效性。消息头的所有编码和解码都由
这个包裹。

为了实现这一点，有一个名为
允许读取、写入或传递任何类型的消息的消息
通过渠道、功能等，另外，大多数
提供了当前支持的比特币信息。对于这些支持
消息，所有与
使用比特币编码的电线被处理，因此呼叫方不必担心
他们的具体情况。

消息交互

以下简要介绍了比特币信息的用途
相互交流。如上所述，这些相互作用不是
由这个包裹直接处理。有关
适当的互动，参见官方比特币协议wiki条目
https://en.bitcoin.it/wiki/protocol_规范。

最初的握手是由两个对等端互相发送版本消息组成的
（msgversion）然后用verack消息（msgverack）响应。两个
对等方使用版本消息（msgversion）中的信息进行协商
协议版本和相互支持的服务。一次
初始握手完成，下表显示消息
没有特定顺序的交互。

 对等A发送对等B响应
 ————————————————————————————————————————————————————————————————————————————————————————————————————————————————
 getaddr消息（msggetaddr）addr消息（msgaddr）
 getBlocks消息（msggetBlocks）inv消息（msginv）
 inv message（msginv）getdata message（msggetdata）
 getdata message（msggetdata）block message（msgblock）-或-
                                       Tx消息（MSGTX）-或-
                                       找不到消息（msgnotfound）
 getheaders消息（msggetaders）headers消息（msgheaders）
 ping消息（msgping）pong消息（msgheaders）*-或-
                                       （无——发送消息的能力就足够了）

 笔记：
 *在定义的更高协议版本之前，没有添加pong消息。
   在BIP031中。bip0031version常量可用于检测最近
   为此目的提供足够的协议版本（版本>bip0031版本）。

常用参数

在使用此包进行读取时，会出现几个常见的参数
写比特币信息。以下部分简要介绍了
这些参数使下一节可以建立在它们之上。

协议版本

协议版本应该在更高的位置与远程对等机协商。
但是，通过版本（msgversion）消息交换使其级别高于此包，
此包提供Wire.ProtocolVersion常量，该常量指示
此包支持的最新协议版本，通常是要使用的值
对于可能更低的协议版本之前的所有出站连接
谈判。

比特币网络

比特币网络是一个神奇的数字，用于识别
消息和消息适用的比特币网络。此包提供
以下常量：

 有线电视网
 Wire.testnet（回归测试网络）
 Wire.testNet3（测试网络版本3）
 Wire.Simnet（模拟测试网络）

确定消息类型

如比特币消息概述部分所述，此包
并使用名为message的通用接口编写比特币消息。在
要确定消息的实际具体类型，请使用类型
开关或类型断言。类型开关的示例如下：

 //假定msg已经是有效的具体消息，例如
 //通过newmsgversion或readmessage读取。
 开关消息：=msg.（类型）
 外壳*电线.msg版本：
  //该消息是指向msgversion结构的指针。
  fmt.printf（“协议版本：%v”，msg.protocol version）
 外壳*线.msgblock：
  //消息是指向msgblock结构的指针。
  fmt.printf（“块中Tx的数目：%v”，msg.header.txncount）
 }

正在读取消息

为了取消比特币信息的标记，请使用readmessage
功能。它接受任何IO.reader，但通常这是一个net.conn to
运行比特币对等机的远程节点。示例语法为：

 //读取并验证来自conn的下一比特币消息，使用
 //协议版本pver和比特币网络btcnet。回报
 //是Wire.Message，包含未编址的
 //原始负载和可能的错误。
 msg，rawpayload，err:=wire.readmessage（conn，pver，btcnet）
 如果犯错！= nIL{
  //记录并处理错误
 }

正在写入消息

要将比特币消息整理到有线，请使用writemessage
功能。它接受任何IO.Writer，但通常这是一个net.conn to
运行比特币对等机的远程节点。请求地址的语法示例
从远程对等机：

 //创建新的getaddr比特币消息。
 消息：=wire.newmsggetaddr（）

 //使用协议版本将比特币消息msg写入conn
 //pver和比特币网络btcnet。返回是可能的
 /错误。
 错误：=Wire.WriteMessage（conn、msg、pver、btcnet）
 如果犯错！= nIL{
  //记录并处理错误
 }

错误

此包返回的错误可能是底层提供的原始错误
从诸如io.eof、io.errUnexpectedeof和
IO.errShortWrite或Wire.MessageError类型。这允许呼叫者
通过类型区分常规IO错误和格式错误的消息
断言。

比特币改进建议

此包包括以下BIP概述的规范更改：

 bip0014（https://github.com/bitcoin/bips/blob/master/bip-0014.mediawiki）
 bip0031（https://github.com/bitcoin/bips/blob/master/bip-0031.mediawiki）
 bip0035（https://github.com/bitcoin/bips/blob/master/bip-0035.mediawiki）
 bip0037（https://github.com/bitcoin/bips/blob/master/bip-0037.mediawiki）
 bip0111（https://github.com/bitcoin/bips/blob/master/bip-0111.mediawiki）
 bip0130（https://github.com/bitcoin/bips/blob/master/bip-0130.mediawiki）
 bip0133（https://github.com/bitcoin/bips/blob/master/bip-0133.mediawiki）
**/

package wire
