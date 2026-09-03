package utils

import (
	"crypto/tls"
	"fmt"
	"go-infinitechat/service/user/api/internal/types/constants"

	"net/smtp"
)

// 发送邮箱验证码
func SendEmailCode(targetEmail, authCode string) error {
	subject := constants.EmailSubject
	//邮箱的内容
	body := fmt.Sprintf("您的验证码为:%s(五分钟内有效)", authCode)

	msg := []byte("From: " + constants.EmailName + " <" + constants.EmailUserName + ">\r\n" +
		"To: " + targetEmail + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body + "\r\n")

	// 登录用户名、登录凭证、登录哪台服务器
	auth := smtp.PlainAuth("", constants.EmailUserName, constants.EmailPassword, constants.EmailHostName)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         constants.EmailHostName,
		MinVersion:         tls.VersionTLS12,
	}

	// 建立 SSL 连接(465端口)
	conn, err := tls.Dial("tcp", constants.EmailHostName+":465", tlsConfig)
	if err != nil {
		return fmt.Errorf("邮件连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, constants.EmailHostName)
	if err != nil {
		return fmt.Errorf("创建smtp客户端失败: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("邮件认证失败: %w", err)
	}

	if err = client.Mail(constants.EmailUserName); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	if err = client.Rcpt(targetEmail); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取邮件写入流失败: %w", err)
	}

	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("关闭邮件写入流失败: %w", err)
	}

	return client.Quit()
}
