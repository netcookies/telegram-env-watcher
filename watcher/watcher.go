package watcher

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/telegram"

	"telegram-env-watcher/ql"
	"telegram-env-watcher/utils"
)

var exportRegexp = regexp.MustCompile(`(?m)^export\s+(\w+)=["']([^"']+)["']`)

func RegisterHandlers(d *tg.UpdateDispatcher, client *telegram.Client, cfg *utils.Config) {
	// 频道消息
	d.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok || msg == nil {
			log.Println("频道消息类型断言失败，忽略")
			return nil
		}
		peerID := utils.PeerID(msg.PeerID)
		if peerID != cfg.GroupID {
			return nil // 非目标频道
		}
		log.Printf("📢 来自频道 [%s] by [%s]\n内容: %s\n",
			resolvePeerName(msg.PeerID, e),
			resolveSenderName(msg.FromID, e),
			msg.Message)

		return handleMessage(ctx, client, cfg, msg)
	})

	// 群聊消息
	d.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		msg, ok := update.Message.(*tg.Message)
		if !ok || msg == nil {
			log.Println("群聊消息类型断言失败，忽略")
			return nil
		}

		peerID := utils.PeerID(msg.PeerID)
		if peerID != cfg.GroupID {
			return nil // 非目标群组
		}

		log.Printf("💬 来自群组 [%s] by [%s]\n内容: %s\n",
			resolvePeerName(msg.PeerID, e),
			resolveSenderName(msg.FromID, e),
			msg.Message)

		return handleMessage(ctx, client, cfg, msg)
	})
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

	for _, match := range matches {
		key := strings.TrimSpace(match[1])
		value := strings.TrimSpace(match[2])
		log.Printf("🔍 检测到变量: %s = %s\n", key, value)

		if err := ql.UpdateQLEnv(cfg, key, value); err != nil {
			log.Printf("❌ 更新青龙失败: %v\n", err)
		} else {
			log.Printf("✅ 青龙环境变量 %s 更新成功", key)
		}
	}
	return nil
}

// 解析 Peer 名称（频道 / 群聊 / 用户）
func resolvePeerName(peer tg.PeerClass, entities tg.Entities) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		if user, ok := entities.Users[p.UserID]; ok {
			if user.Username != "" {
				return "@" + user.Username
			}
			return strings.TrimSpace(user.FirstName + " " + user.LastName)
		}
		return "👤未知用户"
	case *tg.PeerChat:
		if chat, ok := entities.Chats[p.ChatID]; ok {
			return chat.Title
		}
		return "💬未知群"
	case *tg.PeerChannel:
		if ch, ok := entities.Channels[p.ChannelID]; ok {
			return ch.Title
		}
		return "📢未知频道"
	default:
		return "❓未知来源"
	}
}

// 获取发送者名称
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
