package ql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

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

func SendNotifyViaQL(cfg *utils.Config, title string, body string) error {
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

