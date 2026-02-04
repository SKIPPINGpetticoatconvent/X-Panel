package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/util/random"

	"github.com/skip2/go-qrcode"

	tu "github.com/mymmrac/telego/telegoutil"
)

// ================== 远程创建一键配置入站 ==================

func (t *Tgbot) remoteCreateOneClickInbound(configType string, chatId int64) {
	var err error
	var newInbound *model.Inbound
	var ufwWarning string

	switch configType {
	case "reality":
		newInbound, ufwWarning, err = t.buildRealityInbound("")
	case "xhttp_reality":
		newInbound, ufwWarning, err = t.buildXhttpRealityInbound("")
	case "tls":
		newInbound, ufwWarning, err = t.buildTlsInbound()
	case "switch_vision":
		t.SendMsgToTgbot(chatId, "此协议组合的功能还在开发中 ............暂不可用...")
		return
	default:
		err = common.NewError("未知的配置类型")
	}

	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: %v", err))
		return
	}

	// 检查端口和 tag 冲突
	inboundService := InboundService{}

	// 检查端口是否已被使用
	portExist, err := inboundService.getInboundRepo().CheckPortExist(newInbound.Listen, newInbound.Port, 0)
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: 检查端口时出错: %v", err))
		return
	}
	if portExist {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: 端口 %d 已被使用", newInbound.Port))
		return
	}

	// 检查 tag 是否已被使用
	tagExist, err := inboundService.getInboundRepo().CheckTagExist(newInbound.Tag, 0)
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: 检查标签时出错: %v", err))
		return
	}
	if tagExist {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: 标签 %s 已被使用", newInbound.Tag))
		return
	}

	createdInbound, _, err := inboundService.AddInbound(newInbound)
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: 保存入站时出错: %v", err))
		return
	}

	logger.Infof("TG 机器人远程创建入站 %s 成功！", createdInbound.Remark)

	if ufwWarning != "" {
		t.SendMsgToTgbot(chatId, ufwWarning)
	}

	err = t.SendOneClickConfig(createdInbound, false, chatId)
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("⚠️ 入站创建成功，但通知消息发送失败: %v", err))
		logger.Errorf("TG Bot: 远程创建入站成功，但发送通知失败: %v", err)
	} else {
		t.SendMsgToTgbot(chatId, "✅ <b>入站已创建，【二维码/配置链接】已发送至管理员私信。</b>")
	}

	usageMessage := "<b>用法说明：</b>\n\n" +
		"1、该功能已自动生成现今比较主流的入站协议，简单/直接，不用慢慢配置。\n" +
		"2、【一键配置】生成功能中的最前面两种协议组合，适合【优化线路】去直连使用。\n" +
		"3、随机分配一个可用端口，TG端会【自动放行】该端口，生成后请直接复制【<b>链接地址</b>】。\n" +
		"4、TG端 的【一键配置】生成功能，与后台 Web端 类似，跟【入站】的数据是打通的。\n" +
		"5、你可以在\"一键创建\"后于列表中，手动查看/复制或编辑详细信息，以便添加其他参数。"

	t.SendMsgToTgbot(chatId, usageMessage)
}

// ================== 构建入站配置 ==================

func (t *Tgbot) buildRealityInbound(targetDest ...string) (*model.Inbound, string, error) {
	keyPairMsg, err := t.serverService.GetNewX25519Cert()
	if err != nil {
		return nil, "", fmt.Errorf("获取 Reality 密钥对失败: %v", err)
	}
	uuidMsg, err := t.serverService.GetNewUUID()
	if err != nil {
		return nil, "", fmt.Errorf("获取 UUID 失败: %v", err)
	}

	keyPair := keyPairMsg.(map[string]any)
	privateKey, publicKey := keyPair["privateKey"].(string), keyPair["publicKey"].(string)
	uuid := uuidMsg["uuid"]
	remark := random.Seq(8)

	port := 10000 + random.Num(55535-10000+1)

	ufwWarning := ""

	if err := t.openPortWithFirewalld(port); err != nil {
		logger.Warningf("自动放行端口 %d 失败: %v", port, err)
		ufwWarning = fmt.Sprintf("⚠️ <b>警告：端口放行失败</b>\n\n自动执行 <code>firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --reload</code> 命令失败，入站创建流程已继续，但请务必<b>手动</b>在您的 VPS 上放行端口 <code>%d</code>，否则服务将无法访问。失败详情：%v", port, port, err)
	}

	tag := fmt.Sprintf("inbound-%d", port)

	realityDests := t.GetRealityDestinations()
	var randomDest string
	if len(targetDest) > 0 && targetDest[0] != "" {
		randomDest = targetDest[0]
	} else {
		if t.serverService != nil {
			randomDest = t.serverService.GetNewSNI()
		} else {
			randomDest = realityDests[random.Num(len(realityDests))]
		}
	}

	serverNamesList := GenerateRealityServerNames(randomDest)
	shortIds := t.generateShortIds()

	settings, _ := json.Marshal(map[string]any{
		"clients": []map[string]any{{
			"id":     uuid,
			"flow":   "xtls-rprx-vision",
			"email":  remark,
			"level":  0,
			"enable": true,
		}},
		"decryption": "none",
		"fallbacks":  []any{},
	})

	streamSettings, _ := json.Marshal(map[string]any{
		"network":  "tcp",
		"security": "reality",
		"realitySettings": map[string]any{
			"show":        false,
			"target":      randomDest,
			"xver":        0,
			"serverNames": serverNamesList,
			"settings": map[string]any{
				"publicKey":     publicKey,
				"spiderX":       "/",
				"mldsa65Verify": "",
			},
			"privateKey":   privateKey,
			"maxClientVer": "",
			"minClientVer": "",
			"maxTimediff":  0,
			"mldsa65Seed":  "",
			"shortIds":     shortIds,
		},
		"tcpSettings": map[string]any{
			"acceptProxyProtocol": false,
			"header": map[string]any{
				"type": "none",
			},
		},
	})

	sniffing, _ := json.Marshal(map[string]any{
		"enabled":      true,
		"destOverride": []string{"http", "tls", "quic", "fakedns"},
		"metadataOnly": false,
		"routeOnly":    false,
	})

	return &model.Inbound{
		UserId:         1,
		Remark:         remark,
		Enable:         true,
		Listen:         "",
		Port:           port,
		Tag:            tag,
		Protocol:       "vless",
		ExpiryTime:     0,
		DeviceLimit:    0,
		Settings:       string(settings),
		StreamSettings: string(streamSettings),
		Sniffing:       string(sniffing),
	}, ufwWarning, nil
}

func (t *Tgbot) buildTlsInbound() (*model.Inbound, string, error) {
	encMsg, err := t.serverService.GetNewVlessEnc()
	if err != nil {
		return nil, "", fmt.Errorf("获取 VLESS 加密配置失败: %v", err)
	}
	uuidMsg, err := t.serverService.GetNewUUID()
	if err != nil {
		return nil, "", fmt.Errorf("获取 UUID 失败: %v", err)
	}

	var decryption, encryption string

	encMsgMap, ok := encMsg.(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("VLESS 加密配置格式不正确: 期望得到 map[string]interface {}，但收到了 %T", encMsg)
	}

	authsVal, found := encMsgMap["auths"]

	if !found {
		return nil, "", common.NewError("VLESS 加密配置 auths 格式不正确: 未能在响应中找到 'auths' 数组")
	}

	auths, ok := authsVal.([]map[string]string)
	if !ok {
		return nil, "", fmt.Errorf("VLESS 加密配置 auths 格式不正确: 'auths' 数组的内部元素类型应为 map[string]string，但收到了 %T", authsVal)
	}

	for _, auth := range auths {
		if label, ok2 := auth["label"]; ok2 && label == "ML-KEM-768, Post-Quantum" {
			decryption = auth["decryption"]
			encryption = auth["encryption"]
			break
		}
	}

	if decryption == "" || encryption == "" {
		return nil, "", common.NewError("未能在 auths 数组中找到 ML-KEM-768 加密密钥，请检查 Xray 版本")
	}

	domain, err := t.getDomain()
	if err != nil {
		return nil, "", err
	}

	uuid := uuidMsg["uuid"]
	remark := random.Seq(8)
	allowedPorts := []int{2053, 2083, 2087, 2096, 8443}
	port := allowedPorts[random.Num(len(allowedPorts))]

	ufwWarning := ""

	if err := t.openPortWithFirewalld(port); err != nil {
		logger.Warningf("自动放行端口 %d 失败: %v", port, err)
		ufwWarning = fmt.Sprintf("⚠️ <b>警告：端口放行失败</b>\n\n自动执行 <code>firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --reload</code> 命令失败，入站创建流程已继续，但请务必<b>手动</b>在您的 VPS 上放行端口 <code>%d</code>，否则服务将无法访问。失败详情：%v", port, port, err)
	}

	tag := fmt.Sprintf("inbound-%d", port)
	path := "/" + random.SeqWithCharset(8, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	certPath := fmt.Sprintf("/root/cert/%s/fullchain.pem", domain)
	keyPath := fmt.Sprintf("/root/cert/%s/privkey.pem", domain)

	settings, _ := json.Marshal(map[string]any{
		"clients": []map[string]any{{
			"id":       uuid,
			"email":    remark,
			"level":    0,
			"password": "",
			"enable":   true,
		}},
		"decryption":   decryption,
		"encryption":   encryption,
		"selectedAuth": "ML-KEM-768, Post-Quantum",
	})

	streamSettings, _ := json.Marshal(map[string]any{
		"network":  "xhttp",
		"security": "tls",
		"tlsSettings": map[string]any{
			"alpn": []string{"h2", "http/1.1"},
			"certificates": []map[string]any{{
				"buildChain":      false,
				"certificateFile": certPath,
				"keyFile":         keyPath,
				"oneTimeLoading":  false,
				"usage":           "encipherment",
			}},
			"cipherSuites":            "",
			"disableSystemRoot":       false,
			"echForceQuery":           "none",
			"echServerKeys":           "",
			"enableSessionResumption": false,
			"maxVersion":              "1.3",
			"minVersion":              "1.2",
			"rejectUnknownSni":        false,
			"serverName":              domain,
		},
		"xhttpSettings": map[string]any{
			"headers":              map[string]any{},
			"host":                 "",
			"mode":                 "packet-up",
			"noSSEHeader":          false,
			"path":                 path,
			"scMaxBufferedPosts":   30,
			"scMaxEachPostBytes":   "1000000",
			"scStreamUpServerSecs": "20-80",
			"xPaddingBytes":        "100-1000",
		},
	})

	sniffing, _ := json.Marshal(map[string]any{
		"enabled":      false,
		"destOverride": []string{"http", "tls", "quic", "fakedns"},
		"metadataOnly": false,
		"routeOnly":    false,
	})

	return &model.Inbound{
		UserId:         1,
		Remark:         remark,
		Enable:         true,
		Listen:         "",
		Port:           port,
		Tag:            tag,
		Protocol:       "vless",
		ExpiryTime:     0,
		DeviceLimit:    0,
		Settings:       string(settings),
		StreamSettings: string(streamSettings),
		Sniffing:       string(sniffing),
	}, ufwWarning, nil
}

func (t *Tgbot) buildXhttpRealityInbound(targetDest ...string) (*model.Inbound, string, error) {
	keyPairMsg, err := t.serverService.GetNewX25519Cert()
	if err != nil {
		return nil, "", fmt.Errorf("获取 Reality 密钥对失败: %v", err)
	}
	uuidMsg, err := t.serverService.GetNewUUID()
	if err != nil {
		return nil, "", fmt.Errorf("获取 UUID 失败: %v", err)
	}

	keyPair := keyPairMsg.(map[string]any)
	privateKey, publicKey := keyPair["privateKey"].(string), keyPair["publicKey"].(string)
	uuid := uuidMsg["uuid"]
	remark := random.Seq(8)

	port := 10000 + random.Num(55535-10000+1)
	path := "/" + random.SeqWithCharset(8, "abcdefghijklmnopqrstuvwxyz")

	var ufwWarning string
	if err := t.openPortWithFirewalld(port); err != nil {
		logger.Warningf("自动放行端口 %d 失败: %v", port, err)
		ufwWarning = fmt.Sprintf("⚠️ <b>警告：端口放行失败</b>\n\n自动执行 <code>firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --reload</code> 命令失败，但入站创建已继续。请务必<b>手动</b>在您的 VPS 上放行端口 <code>%d</code>，否则服务将无法访问。", port, port)
	}

	tag := fmt.Sprintf("inbound-%d", port)

	realityDests := t.GetRealityDestinations()
	var randomDest string
	if len(targetDest) > 0 && targetDest[0] != "" {
		randomDest = targetDest[0]
	} else {
		if t.serverService != nil {
			randomDest = t.serverService.GetNewSNI()
		} else {
			randomDest = realityDests[random.Num(len(realityDests))]
		}
	}

	serverNamesList := GenerateRealityServerNames(randomDest)
	shortIds := t.generateShortIds()

	settings, _ := json.Marshal(map[string]any{
		"clients": []map[string]any{{
			"id":       uuid,
			"flow":     "",
			"email":    remark,
			"level":    0,
			"password": "",
			"enable":   true,
		}},
		"decryption":   "none",
		"selectedAuth": "X25519, not Post-Quantum",
	})

	streamSettings, _ := json.Marshal(map[string]any{
		"network":  "xhttp",
		"security": "reality",
		"realitySettings": map[string]any{
			"show":         false,
			"target":       randomDest,
			"xver":         0,
			"serverNames":  serverNamesList,
			"privateKey":   privateKey,
			"maxClientVer": "",
			"minClientVer": "",
			"maxTimediff":  0,
			"mldsa65Seed":  "",
			"shortIds":     shortIds,
			"settings": map[string]any{
				"publicKey":     publicKey,
				"spiderX":       "/",
				"mldsa65Verify": "",
			},
		},
		"xhttpSettings": map[string]any{
			"headers":              map[string]any{},
			"host":                 "",
			"mode":                 "stream-up",
			"noSSEHeader":          false,
			"path":                 path,
			"scMaxBufferedPosts":   30,
			"scMaxEachPostBytes":   "1000000",
			"scStreamUpServerSecs": "20-80",
			"xPaddingBytes":        "100-1000",
		},
	})

	sniffing, _ := json.Marshal(map[string]any{
		"enabled":      true,
		"destOverride": []string{"http", "tls", "quic", "fakedns"},
		"metadataOnly": false,
		"routeOnly":    false,
	})

	return &model.Inbound{
		UserId:         1,
		Remark:         remark,
		Enable:         true,
		Listen:         "",
		Port:           port,
		Tag:            tag,
		Protocol:       "vless",
		ExpiryTime:     0,
		DeviceLimit:    0,
		Settings:       string(settings),
		StreamSettings: string(streamSettings),
		Sniffing:       string(sniffing),
	}, ufwWarning, nil
}

// ================== 发送配置消息 ==================

func (t *Tgbot) SendOneClickConfig(inbound *model.Inbound, inFromPanel bool, targetChatId int64) error {
	if targetChatId == 0 {
		if len(adminIds) == 0 {
			return fmt.Errorf("无法发送 TG 通知: 未配置管理员 Chat ID")
		}
		var lastErr error
		for _, adminId := range adminIds {
			if err := t.SendOneClickConfig(inbound, inFromPanel, adminId); err != nil {
				lastErr = err
			}
		}
		return lastErr
	}

	var link string
	var err error
	var linkType string
	var dbLinkType string

	var streamSettings map[string]any
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings)

	if security, ok := streamSettings["security"].(string); ok {
		switch security {
		case "reality":
			if network, ok := streamSettings["network"].(string); ok && network == "xhttp" {
				link, err = t.generateXhttpRealityLink(inbound)
				linkType = "VLESS + XHTTP + Reality"
				dbLinkType = "vless_xhttp_reality"
			} else {
				link, err = t.generateRealityLink(inbound)
				linkType = "VLESS + TCP + Reality"
				dbLinkType = "vless_reality"
			}
		case "tls":
			link, err = t.generateTlsLink(inbound)
			linkType = "Vless Encryption + XHTTP + TLS"
			dbLinkType = "vless_tls_encryption"
		default:
			return fmt.Errorf("未知的入站 security 类型: %s", security)
		}
	} else {
		return common.NewError("无法解析 streamSettings 中的 security 字段")
	}

	if err != nil {
		return err
	}

	qrCodeBytes, err := qrcode.Encode(link, qrcode.Medium, 256)
	if err != nil {
		logger.Warningf("生成二维码失败，将尝试发送纯文本链接: %v", err)
		qrCodeBytes = nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	baseCaption := fmt.Sprintf(
		"入站备注（用户 Email）：\n\n------->>>  <code>%s</code>\n\n对应端口号：\n\n---------->>>>>  <code>%d</code>\n\n协议类型：\n\n<code>%s</code>\n\n设备限制：0（无限制）\n\n生成时间：\n\n<code>%s</code>",
		inbound.Remark,
		inbound.Port,
		linkType,
		now,
	)

	var caption string
	if inFromPanel {
		caption = fmt.Sprintf("✅ <b>面板【一键配置】入站已创建成功！</b>\n\n%s\n\n👇 <b>可点击下方链接直接【复制/导入】</b> 👇", baseCaption)
	} else {
		caption = fmt.Sprintf("✅ <b>TG端 远程【一键配置】创建成功！</b>\n\n%s\n\n👇 <b>可点击下方链接直接【复制/导入】</b> 👇", baseCaption)
	}

	if len(qrCodeBytes) > 0 {
		photoParams := tu.Photo(
			tu.ID(targetChatId),
			tu.FileFromBytes(qrCodeBytes, "qrcode.png"),
		).WithCaption(caption).WithParseMode("HTML")

		if _, err := bot.SendPhoto(context.Background(), photoParams); err != nil {
			logger.Warningf("发送带二维码的 TG 消息给 %d 失败: %v", targetChatId, err)
			t.SendMsgToTgbot(targetChatId, caption)
		}
	} else {
		t.SendMsgToTgbot(targetChatId, caption)
	}

	t.SendMsgToTgbot(targetChatId, link)
	t.saveLinkToHistory(dbLinkType, link)

	return nil
}

// ================== 链接生成 ==================

func (t *Tgbot) generateRealityLink(inbound *model.Inbound) (string, error) {
	var settings map[string]any
	_ = json.Unmarshal([]byte(inbound.Settings), &settings)
	clients, _ := settings["clients"].([]interface{})
	client := clients[0].(map[string]interface{})
	uuid := client["id"].(string)

	var streamSettings map[string]any
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings)
	realitySettings := streamSettings["realitySettings"].(map[string]interface{})
	serverNames := realitySettings["serverNames"].([]interface{})
	sni := serverNames[0].(string)

	settingsMap, ok := realitySettings["settings"].(map[string]interface{})
	if !ok {
		return "", common.NewError("realitySettings中缺少settings子对象")
	}
	publicKey, ok := settingsMap["publicKey"].(string)
	if !ok {
		return "", common.NewError("publicKey字段缺失或格式错误")
	}

	shortIdsInterface := realitySettings["shortIds"].([]interface{})
	if len(shortIdsInterface) == 0 {
		return "", common.NewError("无法生成 Reality 链接: Short IDs 列表为空")
	}
	sid := shortIdsInterface[random.Num(len(shortIdsInterface))].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	return fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F&flow=xtls-rprx-vision#%s-%s",
		uuid, domain, inbound.Port, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

func (t *Tgbot) generateRealityLinkWithClient(inbound *model.Inbound, client model.Client) (string, error) {
	uuid := client.ID

	var streamSettings map[string]any
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings)
	realitySettings := streamSettings["realitySettings"].(map[string]interface{})
	serverNames := realitySettings["serverNames"].([]interface{})
	sni := serverNames[0].(string)

	settingsMap, ok := realitySettings["settings"].(map[string]interface{})
	if !ok {
		return "", common.NewError("realitySettings中缺少settings子对象")
	}
	publicKey, ok := settingsMap["publicKey"].(string)
	if !ok {
		return "", common.NewError("publicKey字段缺失或格式错误")
	}

	shortIdsInterface := realitySettings["shortIds"].([]interface{})
	if len(shortIdsInterface) == 0 {
		return "", common.NewError("无法生成 Reality 链接: Short IDs 列表为空")
	}
	sid := shortIdsInterface[random.Num(len(shortIdsInterface))].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	return fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F&flow=xtls-rprx-vision#%s-%s",
		uuid, domain, inbound.Port, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

func (t *Tgbot) generateTlsLink(inbound *model.Inbound) (string, error) {
	var settings map[string]any
	_ = json.Unmarshal([]byte(inbound.Settings), &settings)
	clients, _ := settings["clients"].([]interface{})
	client := clients[0].(map[string]interface{})
	uuid := client["id"].(string)
	encryption := settings["encryption"].(string)

	var streamSettings map[string]any
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings)
	tlsSettings := streamSettings["tlsSettings"].(map[string]interface{})
	sni := tlsSettings["serverName"].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	xhttpSettings, _ := streamSettings["xhttpSettings"].(map[string]interface{})
	path := ""
	if xhttpSettings != nil {
		path, _ = xhttpSettings["path"].(string)
	}

	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=%s&path=%s&security=tls&fp=chrome&alpn=http%%2F1.1&sni=%s#%s-%s",
		uuid, domain, inbound.Port, encryption, url.QueryEscape(path), sni, inbound.Remark, inbound.Remark), nil
}

func (t *Tgbot) generateTlsLinkWithClient(inbound *model.Inbound, client model.Client) (string, error) {
	uuid := client.ID

	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return "", err
	}
	encryption := settings["encryption"].(string)

	var streamSettings map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings); err != nil {
		return "", err
	}
	tlsSettings := streamSettings["tlsSettings"].(map[string]interface{})
	sni := tlsSettings["serverName"].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	xhttpSettings, _ := streamSettings["xhttpSettings"].(map[string]interface{})
	path := ""
	if xhttpSettings != nil {
		path, _ = xhttpSettings["path"].(string)
	}

	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=%s&path=%s&security=tls&fp=chrome&alpn=http%%2F1.1&sni=%s#%s-%s",
		uuid, domain, inbound.Port, encryption, url.QueryEscape(path), sni, client.Email, inbound.Remark), nil
}

func (t *Tgbot) generateXhttpRealityLink(inbound *model.Inbound) (string, error) {
	var settings map[string]any
	if err := json.Unmarshal([]byte(inbound.Settings), &settings); err != nil {
		return "", err
	}
	clients, _ := settings["clients"].([]interface{})
	client := clients[0].(map[string]interface{})
	uuid := client["id"].(string)

	var streamSettings map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings); err != nil {
		return "", err
	}

	realitySettings := streamSettings["realitySettings"].(map[string]interface{})
	serverNames := realitySettings["serverNames"].([]interface{})
	sni := serverNames[0].(string)

	settingsMap, _ := realitySettings["settings"].(map[string]interface{})
	publicKey, _ := settingsMap["publicKey"].(string)

	shortIdsInterface, _ := realitySettings["shortIds"].([]interface{})
	if len(shortIdsInterface) == 0 {
		return "", common.NewError("无法生成 Reality 链接: Short IDs 列表为空")
	}
	sid := shortIdsInterface[random.Num(len(shortIdsInterface))].(string)

	xhttpSettings, _ := streamSettings["xhttpSettings"].(map[string]interface{})
	path := xhttpSettings["path"].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	escapedPath := url.QueryEscape(path)
	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=none&path=%s&host=&mode=stream-up&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F#%s-%s",
		uuid, domain, inbound.Port, escapedPath, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

func (t *Tgbot) generateXhttpRealityLinkWithClient(inbound *model.Inbound, client model.Client) (string, error) {
	uuid := client.ID

	var streamSettings map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings); err != nil {
		return "", err
	}

	realitySettings := streamSettings["realitySettings"].(map[string]interface{})
	serverNames := realitySettings["serverNames"].([]interface{})
	sni := serverNames[0].(string)

	settingsMap, _ := realitySettings["settings"].(map[string]interface{})
	publicKey, _ := settingsMap["publicKey"].(string)

	shortIdsInterface, _ := realitySettings["shortIds"].([]interface{})
	if len(shortIdsInterface) == 0 {
		return "", common.NewError("无法生成 Reality 链接: Short IDs 列表为空")
	}
	sid := shortIdsInterface[random.Num(len(shortIdsInterface))].(string)

	xhttpSettings, _ := streamSettings["xhttpSettings"].(map[string]interface{})
	path := xhttpSettings["path"].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	escapedPath := url.QueryEscape(path)
	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=none&path=%s&host=&mode=stream-up&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F#%s-%s",
		uuid, domain, inbound.Port, escapedPath, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

// ================== 辅助函数 ==================

func (t *Tgbot) getDomain() (string, error) {
	cmd := exec.Command("/usr/local/x-ui/x-ui", "setting", "-getCert", "true")
	output, err := cmd.Output()
	if err != nil {
		return "", common.NewError("执行命令获取证书路径失败，请确保已为面板配置 SSL 证书")
	}

	lines := strings.Split(string(output), "\n")
	certLine := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "cert:") {
			certLine = line
			break
		}
	}

	if certLine == "" {
		return "", common.NewError("无法从 x-ui 命令输出中找到证书路径")
	}

	certPath := strings.TrimSpace(strings.TrimPrefix(certLine, "cert:"))
	if certPath == "" {
		return "", common.NewError("证书路径为空，请确保已为面板配置 SSL 证书")
	}

	domain := filepath.Base(filepath.Dir(certPath))
	return domain, nil
}

func (t *Tgbot) generateShortIds() []string {
	chars := "0123456789abcdef"
	lengths := []int{2, 4, 6, 8, 10, 12, 14, 16}
	shortIds := make([]string, len(lengths))
	for i, length := range lengths {
		shortIds[i] = random.SeqWithCharset(length, chars)
	}
	return shortIds
}
