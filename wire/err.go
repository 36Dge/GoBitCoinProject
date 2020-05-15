package wire

import "fmt"

// messageerror带有消息的问题
//一些潜在问题的例子是来自错误比特币的信息
//网络、无效命令、不匹配的校验以及超过最大有效负载
// 这位调用者提供了一种将错误断言到区分一般IO错误
// (如IO.EOF)和邮件格式不正确

// 1. 定义自己的结构体错误类型
type MessageError struct {
	Func        string // 函数名
	Description string //人类可读的问题描述
}

// 2. 用自己的结构体错误类型实现系统的Error()方法
func (e *MessageError) Error() string {
	if e.Func != "" {
		return fmt.Sprintf("%v:%v", e.Func, e.Description)
	}
	return e.Description
}

// messageerror为给定的函数和说明创建错误
func messageError(f string, desc string) *MessageError {
	return &MessageError{Func: f, Description: desc}
}
