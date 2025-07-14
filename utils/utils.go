package utils

import (
	"encoding/json"
	// "fmt"
	"io/ioutil"
	"log"

	"github.com/gotd/td/tg"
)

type Config struct {
	Telegram struct {
		APIID   int    `json:"api_id"`
		APIHash string `json:"api_hash"`
		Phone   string `json:"phone"`
	} `json:"telegram"`

	QL struct {
		BaseURL      string `json:"base_url"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"ql"`

	GroupID int64 `json:"group_id"`
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

func PeerID(p tg.PeerClass) int64 {
	switch v := p.(type) {
	case *tg.PeerChannel:
		return -100 + int64(v.ChannelID)
	case *tg.PeerChat:
		return int64(-1 * v.ChatID)
	case *tg.PeerUser:
		return int64(v.UserID)
	default:
		return 0
	}
}
