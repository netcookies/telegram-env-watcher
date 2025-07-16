package watcher

import (
	"context"
	"log"
	"regexp"
	"strings"
	"fmt"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/telegram"

	"telegram-env-watcher/ql"
	"telegram-env-watcher/utils"
)

var exportRegexp = regexp.MustCompile(`(?m)^export\s+(\w+)=["']([^"']+)["']`)

type WatchTargets struct {
	Channels []tg.InputChannelClass
	Users   []tg.InputPeerClass
}

func RegisterHandlers(d *tg.UpdateDispatcher, client *telegram.Client, cfg *utils.Config, targets *WatchTargets) {
	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok || msg == nil {
			log.Println("频道消息类型断言失败，忽略")
			return nil
		}
		id := utils.PeerIDFromPeer(msg.PeerID)
		if !containsChannel(targets.Channels, id) {
			return nil
		}
		log.Printf("📢 来自频道 [%s] by [%s]\n内容: %s\n",
			resolvePeerName(msg.PeerID, e),
			resolveSenderName(msg.FromID, e),
			msg.Message)
		return handleMessage(ctx, client, cfg, msg)
	})

	//监听普通群（旧版TG，现在新版都是超级群，走的是Channel）
	d.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok || msg == nil {
			log.Println("群聊消息类型断言失败，忽略")
			return nil
		}
		id := utils.PeerIDFromPeer(msg.PeerID)
		if !containsUser(targets.Users, id) {
			return nil
		}
		log.Printf("💬 来自群组 [%s] by [%s]\n内容: %s\n",
			resolvePeerName(msg.PeerID, e),
			resolveSenderName(msg.FromID, e),
			msg.Message)
		return handleMessage(ctx, client, cfg, msg)
	})
}

func containsChannel(channels []tg.InputChannelClass, id int64) bool {
	for _, ch := range channels {
		peer, ok := ch.(*tg.InputChannel)
		if !ok {
			continue
		}
		if utils.PeerIDFromPeer(&tg.PeerChannel{ChannelID: peer.ChannelID}) == id {
			return true
		}
	}
	return false
}

func containsUser(users []tg.InputPeerClass, id int64) bool {
	for _, g := range users {
		switch v := g.(type) {
		case *tg.InputPeerChat:
			if utils.PeerIDFromPeer(&tg.PeerChat{ChatID: v.ChatID}) == id {
				return true
			}
		case *tg.InputPeerUser:
			if utils.PeerIDFromPeer(&tg.PeerUser{UserID: v.UserID}) == id {
				return true
			}
		case *tg.InputPeerChannel:
			if utils.PeerIDFromPeer(&tg.PeerChannel{ChannelID: v.ChannelID}) == id {
				return true
			}
		}
	}
	return false
}

func handleMessage(ctx context.Context, client *telegram.Client, cfg *utils.Config, msg *tg.Message) error {
	if msg == nil || msg.Message == "" {
		return nil
	}

	matches := exportRegexp.FindAllStringSubmatch(msg.Message, -1)
	if len(matches) == 0 {
		log.Println("❌ 消息中未匹配到任何 export 变量")
		return nil
	}

	var updatedVars []string
	var runScripts []string
	var notifyErrs []string

	for _, match := range matches {
		key := strings.TrimSpace(match[1])
		value := strings.TrimSpace(match[2])
		log.Printf("🔍 检测到变量: %s = %s\n", key, value)

		if err := ql.UpdateQLEnv(cfg, key, value); err != nil {
			errMsg := fmt.Sprintf("❌ 更新 %s 失败: %v", key, err)
			log.Println(errMsg)
			notifyErrs = append(notifyErrs, errMsg)
			continue
		}

		log.Printf("✅ 青龙环境变量 %s 更新成功", key)
		updatedVars = append(updatedVars, fmt.Sprintf("%s = %s", key, value))

		prefix := utils.ExtractPrefix(key)
		log.Printf("🔍 提取的前缀: %s", prefix)

		scripts, err := ql.SearchCrons(cfg, prefix)
		if err != nil {
			log.Printf("⚠️ 搜索脚本失败 (前缀: %s): %v", prefix, err)
			notifyErrs = append(notifyErrs, fmt.Sprintf("搜索脚本失败（%s）: %v", prefix, err))
			continue
		}

		if len(scripts) == 0 {
			log.Printf("⚠️ 未找到任何匹配脚本（前缀: %s）", prefix)
			continue
		}

		log.Printf("📜 找到 %d 个匹配脚本", len(scripts))
		for _, s := range scripts {
			runScripts = append(runScripts, fmt.Sprintf("%s (ID: %d)", s.Name, s.ID))
		}

		if err := ql.RunCrons(cfg, scripts); err != nil {
			log.Printf("❌ 脚本运行失败: %v", err)
			notifyErrs = append(notifyErrs, fmt.Sprintf("脚本运行失败（前缀: %s）: %v", prefix, err))
		}
	}

	// 📣 构造最终通知消息
	var notifyMsg string
	if len(updatedVars) > 0 {
		notifyMsg += "✅ 已更新以下环境变量:\n"
		for _, kv := range updatedVars {
			notifyMsg += "- " + kv + "\n"
		}
	}

	if len(runScripts) > 0 {
		notifyMsg += "\n🚀 已执行以下脚本:\n"
		for _, name := range runScripts {
			notifyMsg += "- " + name + "\n"
		}
	}

	if len(notifyErrs) > 0 {
		notifyMsg += "\n❗发生以下错误:\n"
		for _, err := range notifyErrs {
			notifyMsg += "- " + err + "\n"
		}
	}

	if notifyMsg == "" {
		notifyMsg = "⚠️ 未检测到变量或脚本更新"
	}

	// ✅ 最终统一发送通知
	ql.SendNotifyViaQL(cfg, "📥 青龙处理结果通知", notifyMsg)
	return nil
}

func resolvePeerName(peer tg.PeerClass, entities tg.Entities) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		if user, ok := entities.Users[p.UserID]; ok {
			if user.Username != "" {
				return "@" + user.Username
			}
			return strings.TrimSpace(user.FirstName + " " + user.LastName)
		}
	case *tg.PeerChat:
		if chat, ok := entities.Chats[p.ChatID]; ok {
			return chat.Title
		}
	case *tg.PeerChannel:
		if ch, ok := entities.Channels[p.ChannelID]; ok {
			return ch.Title
		}
	}
	return "❓未知来源"
}

func resolveSenderName(from tg.PeerClass, entities tg.Entities) string {
	switch p := from.(type) {
	case *tg.PeerUser:
		if user, ok := entities.Users[p.UserID]; ok {
			if user.Username != "" {
				return "@" + user.Username
			}
			return strings.TrimSpace(user.FirstName + " " + user.LastName)
		}
	case *tg.PeerChannel:
		if ch, ok := entities.Channels[p.ChannelID]; ok {
			return ch.Title
		}
	}
	return "👤未知发送者"
}
