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

	"telegram-env-watcher/ql"
	"telegram-env-watcher/auth"
	"telegram-env-watcher/utils"
	"telegram-env-watcher/watcher"
)

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
		UpdateHandler:  handlerWrapper{fn: gaps.Handle},
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

		log.Println("✅ Telegram 登录成功")

		// 解析监听目标（动态获取 AccessHash）
		var targets watcher.WatchTargets
		for _, ch := range cfg.Listen.Channels {
			inputCh, _, title, about, err := utils.ResolveTarget(ctx, client, ch.Username)
			if err != nil {
				log.Printf("❌ 解析频道 @%s 失败: %v", ch.Username, err)
				continue
			}
			log.Printf("📢 监听频道: %s\n简介: %s\n", title, about)
			targets.Channels = append(targets.Channels, inputCh)
		}
		for _, us := range cfg.Listen.Users {
			_, inputUser, title, about, err := utils.ResolveTarget(ctx, client, us.Username)
			if err != nil {
				log.Printf("❌ 解析用户 @%s 失败: %v", us.Username, err)
				continue
			}
			log.Printf("💬 监听用户: %s\n简介: %s\n", title, about)
			targets.Users = append(targets.Users, inputUser)
		}

		if len(targets.Channels) == 0 && len(targets.Users) == 0 {
			log.Fatal("❌ 没有可用的监听目标，程序退出")
		}

		// 注册回调处理器
		watcher.RegisterHandlers(&disp, client, cfg, &targets)

		user, err := client.Self(ctx)
		if err != nil {
			return err
		}

		log.Printf("🚀 Telegram 已登录，用户ID: %d\n", user.ID)
		// ✅ 启动时立即 Flush 上次未发出的通知
		if err := ql.FlushNotifyBuffer(cfg); err != nil {
			log.Printf("⚠️ 启动时通知缓存发送失败: %v", err)
		}

		// ✅ 启动定时器，等待整点执行
		ql.StartNotifyScheduler(cfg)
		return gaps.Run(ctx, client.API(), user.ID, updates.AuthOptions{})
	})

	if err != nil {
		log.Fatalf("❌ 运行失败: %v", err)
	}
}
