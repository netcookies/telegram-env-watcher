package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

type ChannelTarget struct {
	Username string `json:"username"`
}

type Config struct {
	Debug bool `json:"debug"`
	Telegram struct {
		APIID   int    `json:"api_id"`
		APIHash string `json:"api_hash"`
		Phone   string `json:"phone"`
	} `json:"telegram"`

	QL struct {
		BaseURL      string `json:"base_url"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`

		Notify struct {
			ScriptFile string `json:"scriptfile"`
			ScriptPath string `json:"scriptpath"`
			Template   string `json:"template"`
		} `json:"notify"`
	} `json:"ql"`

	Listen struct {
		Channels []ChannelTarget `json:"channels"`
		Users    []ChannelTarget `json:"users"`
	} `json:"listen"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	log.Println("✅ 配置读取成功")
	return &cfg, nil
}

// ResolveTarget 支持解析频道/超级群和普通群
func ResolveTarget(ctx context.Context, client *telegram.Client, username string) (
	tg.InputChannelClass, tg.InputPeerClass, string, string, error,
) {
	res, err := client.API().ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, nil, "", "", fmt.Errorf("❌ 无法解析 @%s: %v", username, err)
	}
	if len(res.Chats) == 0 {
		return nil, nil, "", "", fmt.Errorf("❌ @%s 没有找到任何聊天", username)
	}

	chat := res.Chats[0]
	switch ch := chat.(type) {
	case *tg.Chat:
		return nil, &tg.InputPeerChat{ChatID: ch.ID}, ch.Title, "", nil
	case *tg.Channel:
		peer := &tg.InputChannel{ChannelID: ch.ID, AccessHash: ch.AccessHash}
		full, err := client.API().ChannelsGetFullChannel(ctx, peer)
		if err != nil {
			return nil, nil, "", "", fmt.Errorf("❌ 拉取频道信息失败: %v", err)
		}
		about := ""
		if f, ok := full.FullChat.(*tg.ChannelFull); ok {
			about = f.About
		}
		return peer, nil, ch.Title, about, nil
	default:
		return nil, nil, "", "", fmt.Errorf("❌ @%s 是不支持的聊天类型", username)
	}
}

func PeerIDFromPeer(p tg.PeerClass) int64 {
	switch v := p.(type) {
	case *tg.PeerChannel:
		return -1000000000000 - int64(v.ChannelID)
	case *tg.PeerChat:
		return -2000000000000 - int64(v.ChatID)
	case *tg.PeerUser:
		return int64(v.UserID)
	default:
		return 0
	}
}

func ExtractPrefix(key string) string {
	lastUnderscore := strings.LastIndex(key, "_")
	if lastUnderscore == -1 {
		return key // 没有下划线就返回原始 key
	}
	return key[:lastUnderscore]
}
