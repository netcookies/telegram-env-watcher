package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"

	"telegram-env-watcher/auth"
	"telegram-env-watcher/utils"
	"telegram-env-watcher/watcher"
)

// 添加在 main.go 顶部 import 之后
type handlerWrapper struct {
	fn func(ctx context.Context, u tg.UpdatesClass) error
}

func (h handlerWrapper) Handle(ctx context.Context, u tg.UpdatesClass) error {
	return h.fn(ctx, u)
}

func main() {
	cfg, err := utils.LoadConfig("config.json")
	if err != nil {
		log.Fatalf("❌ 配置文件读取失败: %v", err)
	}

	disp := tg.NewUpdateDispatcher()
	gaps := updates.New(updates.Config{
		Handler: &disp,
	})

	client := telegram.NewClient(cfg.Telegram.APIID, cfg.Telegram.APIHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: "session.json"},
		UpdateHandler: handlerWrapper{fn: gaps.Handle},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		<-c
		cancel()
	}()

	err = client.Run(ctx, func(ctx context.Context) error {
		flow := auth.NewFlow(cfg.Telegram.Phone)
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			return err
		}

		log.Println("✅ Telegram 登录成功，开始监听消息...")

		// 注册回调处理器
		watcher.RegisterHandlers(&disp, client, cfg)

		user, err := client.Self(ctx)
		if err != nil {
			return err
		}

		log.Printf("🚀 Telegram 已登录，用户ID: %d\n", user.ID)
		return gaps.Run(ctx, client.API(), user.ID, updates.AuthOptions{})
	})
	if err != nil {
		log.Fatalf("❌ 运行失败: %v", err)
	}
}
