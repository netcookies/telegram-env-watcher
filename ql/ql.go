package ql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"os"
	"time"

	"telegram-env-watcher/utils"
)

type Env struct {
    ID    *int64 `json:"id,omitempty"`  // 用指针，omitempty 让它为空时不序列化
    Name  string `json:"name"`
    Value string `json:"value"`
}

type tokenResp struct {
	Code int `json:"code"`
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

type ScriptInfo struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

var notifyCacheFile = "./ql_notify_buffer.json"

type NotifyEntry struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Time  int64  `json:"time"` // Unix 时间戳
}

func GetQLToken(cfg *utils.Config) (string, error) {
	url := fmt.Sprintf("%s/open/auth/token?client_id=%s&client_secret=%s",
		cfg.QL.BaseURL, cfg.QL.ClientID, cfg.QL.ClientSecret)
	if cfg.Debug {
		log.Printf("🔗 请求地址: %s\n", url)
	}
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Code != 200 {
		return "", fmt.Errorf("❌ 获取 token 失败，响应码: %d", r.Code)
	}
	return r.Data.Token, nil
}

func UpdateQLEnv(cfg *utils.Config, name, value string) error {
	token, err := GetQLToken(cfg)
	if err != nil {
		return err
	}

	searchURL := fmt.Sprintf("%s/open/envs?searchValue=%s", cfg.QL.BaseURL, name)
	req, _ := http.NewRequest("GET", searchURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var search struct {
		Data []Env `json:"data"`
	}
	body, _ := ioutil.ReadAll(resp.Body)
	_ = json.Unmarshal(body, &search)

  var (
      data   []byte
      method string
      url    = fmt.Sprintf("%s/open/envs", cfg.QL.BaseURL)
  )
	if len(search.Data) > 0 {
			// 更新：单个对象
			payload := Env{ID: search.Data[0].ID, Name: name, Value: value}
			data, err = json.Marshal(payload)
			if err != nil {
					return err
			}
			method = "PUT"
	} else {
			// 新增：数组形式
			payload := []Env{{Name: name, Value: value}}
			data, err = json.Marshal(payload)
			if err != nil {
					return err
			}
			method = "POST"
	}
	
	if cfg.Debug {
		log.Printf("🔗 请求地址: %s\n", url)
		log.Printf("📦 请求方法: %s\n", method)
		log.Printf("🔐 Authorization: Bearer %s\n", token)
		log.Printf("📝 请求 Body: %s\n", string(data))
	}
	req, _ = http.NewRequest(method, url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		b, _ := ioutil.ReadAll(resp.Body)
		log.Printf("❌ 青龙响应失败：%s", string(b))
		return fmt.Errorf("青龙响应码: %d", resp.StatusCode)
	}
	return nil
}

func RunScriptContent(cfg *utils.Config, filename, path, content string) error {
	token, err := GetQLToken(cfg)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/open/scripts/run", cfg.QL.BaseURL)

	payload := map[string]string{
		"filename": filename,
	}

	if path != "" {
		payload["path"] = path
	}

	if content != "" {
		payload["content"] = content
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if cfg.Debug {
		log.Printf("🔗 请求地址: %s\n", url)
		log.Printf("📦 请求方法: PUT\n")
		log.Printf("🔐 Authorization: Bearer %s\n", token)
		log.Printf("📝 请求 Body: %s\n", string(data))
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(data))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		log.Printf("❌ 青龙脚本运行失败：%s", string(respBody))
		return fmt.Errorf("青龙响应码: %d", resp.StatusCode)
	}

	log.Printf("✅ 青龙脚本运行成功\n")
	return nil
}

func RenderTemplate(tpl string, vars map[string]string) string {
	for k, v := range vars {
		tpl = strings.ReplaceAll(tpl, "{{"+k+"}}", v)
	}
	return tpl
}

func SendNotifyNowViaQL(cfg *utils.Config, title string, body string) error {
	content := RenderTemplate(cfg.QL.Notify.Template, map[string]string{
		"title": title,
		"body":  body,
	})
	return RunScriptContent(cfg,
		cfg.QL.Notify.ScriptFile,
		cfg.QL.Notify.ScriptPath,
		content,
	)
}

func SearchCrons(cfg *utils.Config, keyword string) ([]ScriptInfo, error) {
	token, err := GetQLToken(cfg)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/open/crons?searchValue=%s", cfg.QL.BaseURL, keyword)
	if cfg.Debug {
		log.Printf("🔎 搜索脚本: %s\n", url)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("❌ 搜索脚本失败：%s", string(body))
	}

	// 正确嵌套结构
	var result struct {
		Code int `json:"code"`
		Data struct {
			Data  []ScriptInfo `json:"data"`
			Total int          `json:"total"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if cfg.Debug {
		log.Printf("📦 获取到 %d 个脚本", len(result.Data.Data))
		for _, s := range result.Data.Data {
			log.Printf("🔧 脚本: id=%d name=%s command=%s", s.ID, s.Name, s.Command)
		}
	}

	return result.Data.Data, nil
}
  
func RunCrons(cfg *utils.Config, scripts []ScriptInfo) error {
	token, err := GetQLToken(cfg)
	if err != nil {
		return err
	}

	var ids []int
	log.Printf("🚀 即将执行脚本 (%d 个):", len(scripts))
	for _, script := range scripts {
		ids = append(ids, script.ID)
		log.Printf("  - %s (ID: %d) - %s", script.Name, script.ID, script.Command)
	}

	bodyBytes, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("❌ 编码 ID 列表失败: %v", err)
	}

	url := fmt.Sprintf("%s/open/crons/run", cfg.QL.BaseURL)
	if cfg.Debug {
		log.Printf("📤 PUT 请求 URL: %s", url)
		log.Printf("📤 PUT 请求体 JSON: %s", string(bodyBytes))
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("❌ 执行失败，状态码: %d，响应: %s", resp.StatusCode, string(respBody))
	}

	if cfg.Debug {
		log.Printf("✅ 执行成功: %s", string(respBody))
	}

	return nil
}

func SendNotifyViaQL(cfg *utils.Config, title string, body string) error {
	entry := NotifyEntry{
		Title: title,
		Body:  body,
		Time:  time.Now().Unix(),
	}

	// 加载旧数据
	var buffer []NotifyEntry
	if data, err := os.ReadFile(notifyCacheFile); err == nil {
		_ = json.Unmarshal(data, &buffer)
	}

	buffer = append(buffer, entry)

	// 保存到文件
	data, _ := json.MarshalIndent(buffer, "", "  ")
	return os.WriteFile(notifyCacheFile, data, 0644)
}

func StartNotifyScheduler(cfg *utils.Config) {
	go func() {
		for {
			now := time.Now()
			// 计算下一个整点
			next := now.Truncate(time.Hour).Add(time.Hour)
			duration := time.Until(next)
			log.Printf("⏳ 等待到下一个整点: %s", next.Format("15:04:05"))
			time.Sleep(duration)

			if err := FlushNotifyBuffer(cfg); err != nil {
				log.Printf("❌ 通知缓冲发送失败: %v", err)
			}
		}
	}()
}

func FlushNotifyBuffer(cfg *utils.Config) error {
	if _, err := os.Stat(notifyCacheFile); os.IsNotExist(err) {
		log.Println("📭 无需发送通知（无缓存文件）")
		return nil
	}

	data, err := os.ReadFile(notifyCacheFile)
	if err != nil {
		return fmt.Errorf("读取通知缓存失败: %v", err)
	}

	var entries []NotifyEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("解析通知缓存失败: %v", err)
	}

	if len(entries) == 0 {
		log.Println("📭 通知缓存为空，无需发送")
		return nil
	}

	// 构造合并消息
	var body strings.Builder
	for _, e := range entries {
		body.WriteString(fmt.Sprintf("🕒 %s\n📌 %s\n%s\n\n",
			time.Unix(e.Time, 0).Format("15:04:05"),
			e.Title, e.Body))
	}

	// 发送一次合并消息
	log.Println("📨 整点发送合并通知")
	err = RunScriptContent(cfg, cfg.QL.Notify.ScriptFile, cfg.QL.Notify.ScriptPath,
		RenderTemplate(cfg.QL.Notify.Template, map[string]string{
			"title": "📥 每小时通知汇总",
			"body":  body.String(),
		}),
	)
	if err != nil {
		return err
	}

	// 清空缓存
	return os.WriteFile(notifyCacheFile, []byte("[]"), 0644)
}

