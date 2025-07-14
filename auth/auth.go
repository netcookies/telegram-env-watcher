package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/telegram/auth"
)

// CodePrompt 是输入验证码的回调
func CodePrompt(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	fmt.Print("请输入验证码：")
	code, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

// NewFlow 返回一个认证流程，配合 client.Auth().IfNecessary 使用
func NewFlow(phone string) auth.Flow {
	return auth.NewFlow(
		auth.Constant(phone, "", auth.CodeAuthenticatorFunc(CodePrompt)),
		auth.SendCodeOptions{},
	)
}
