package sms

const (
	CodeOther        = 0
	CodeSuccess      = 1
	CodeNoSender     = 2
	CodeSuccessPart  = 3 // 成功了一部分
	CodeInvalidParam = 4 // 不合法的参数
)

var CodeText = map[int]string{
	1: "success",
	2: "no sender",
}
