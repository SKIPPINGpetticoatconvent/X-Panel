package service

import (
	"context"
	"encoding/json" // 新增：用于 json.Marshal / Unmarshal
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"       // 新增：用于 exec.Command（getDomain 等）
	"path/filepath" // 新增：用于 filepath.Base / Dir（getDomain 用到）
	"strconv"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/util/common"

	"x-ui/xray"

	"github.com/skip2/go-qrcode"

	"github.com/google/uuid"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// checkBBRSupport 检查内核版本和 BBR 模块支持
func (t *Tgbot) checkBBRSupport() (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 获取内核版本
	kernelCmd := exec.CommandContext(ctx, "uname", "-r")
	kernelOutput, err := kernelCmd.Output()
	if err != nil {
		return "", false, fmt.Errorf("获取内核版本失败: %v", err)
	}
	kernelVersion := strings.TrimSpace(string(kernelOutput))

	// 解析内核版本号
	kernelParts := strings.Split(kernelVersion, ".")
	if len(kernelParts) < 2 {
		return kernelVersion, false, fmt.Errorf("无法解析内核版本: %s", kernelVersion)
	}

	majorVersion, err := strconv.Atoi(kernelParts[0])
	if err != nil {
		return kernelVersion, false, fmt.Errorf("解析主版本号失败: %v", err)
	}

	minorVersion, err := strconv.Atoi(strings.Split(kernelParts[1], "-")[0])
	if err != nil {
		return kernelVersion, false, fmt.Errorf("解析次版本号失败: %v", err)
	}

	// 检查内核版本是否支持 BBR (需要 4.9+)
	supportsBBR := majorVersion > 4 || (majorVersion == 4 && minorVersion >= 9)

	if !supportsBBR {
		return kernelVersion, false, nil
	}

	// 检查 BBR 模块是否可用
	modprobeCtx, modprobeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer modprobeCancel()

	modprobeCmd := exec.CommandContext(modprobeCtx, "bash", "-c", "modprobe tcp_bbr 2>/dev/null && echo 'supported' || echo 'not_supported'")
	modprobeOutput, err := modprobeCmd.Output()
	if err != nil {
		return kernelVersion, false, fmt.Errorf("检查 BBR 模块失败: %v", err)
	}

	bbrAvailable := strings.TrimSpace(string(modprobeOutput)) == "supported"

	return kernelVersion, bbrAvailable, nil
}

func (t *Tgbot) answerCallback(callbackQuery *telego.CallbackQuery, isAdmin bool) {
	chatId := callbackQuery.Message.GetChat().ID

	if isAdmin {
		// get query from hash storage
		decodedQuery, err := t.decodeQuery(callbackQuery.Data)
		if err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.noQuery"))
			return
		}
		dataArray := strings.Split(decodedQuery, " ")

		if len(dataArray) >= 2 && len(dataArray[1]) > 0 {
			switch dataArray[0] {
			case "update_xray_ask":
				version := dataArray[1]
				confirmKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("✅ 确认更新").WithCallbackData(t.encodeQuery(fmt.Sprintf("update_xray_confirm %s", version))),
					),
					tu.InlineKeyboardRow(
						tu.InlineKeyboardButton("❌ 取消").WithCallbackData(t.encodeQuery("update_xray_cancel")),
					),
				)
				t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), confirmKeyboard)
			case "update_xray_confirm":
				version := dataArray[1]
				t.sendCallbackAnswerTgBot(callbackQuery.ID, "正在启动 Xray 更新任务...")
				t.SendMsgToTgbot(chatId, fmt.Sprintf("🚀 正在更新 Xray 到版本 %s，更新任务已在后台启动...", version))
				go func() {
					err := t.serverService.UpdateXray(version)
					if err != nil {
						t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ Xray 更新失败: %v", err))
					} else {
						t.SendMsgToTgbot(chatId, fmt.Sprintf("✅ Xray 成功更新到版本 %s", version))
					}
				}()
			case "update_xray_cancel":
				t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
				t.sendCallbackAnswerTgBot(callbackQuery.ID, "已取消")
				return
			case "set_log_level":
				// 解析级别参数
				if len(dataArray) < 2 {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 参数错误")
					return
				}
				newLevel := dataArray[1]
				// 验证级别
				validLevels := map[string]bool{"error": true, "warn": true, "warning": true, "info": true, "debug": true}
				if !validLevels[newLevel] {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 无效的日志级别")
					return
				}
				// 标准化级别名称
				if newLevel == "warning" {
					newLevel = "warn"
				}
				err := t.settingService.SetTgLogLevel(newLevel)
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 设置失败")
					return
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ 日志级别已设置为 %s", newLevel))
				t.showLogSettings(chatId)
			case "fetch_logs":
				// 解析数量参数
				count := 20 // 默认
				if len(dataArray) > 1 {
					if c, err := strconv.Atoi(dataArray[1]); err == nil && c > 0 {
						count = c
					}
				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("📄 获取最近 %d 条日志...", count))
				// 获取配置的日志级别
				level, err := t.settingService.GetTgLogLevel()
				if err != nil {
					level = "info" // 默认级别
				}
				logs := logger.GetLogs(count, level)
				if len(logs) == 0 {
					t.SendMsgToTgbot(chatId, "📋 **最近日志**\n\n❌ 未找到符合级别的日志记录")
				} else {
					content := strings.Join(logs, "\n")
					t.sendLongMessage(chatId, content)
				}
			default:
				email := dataArray[1]
				switch dataArray[0] {
				case "client_get_usage":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.messages.email", "Email=="+email))
					t.searchClient(chatId, email)
				case "client_refresh":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.clientRefreshSuccess", "Email=="+email))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "client_cancel":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "ips_refresh":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.IpRefreshSuccess", "Email=="+email))
					t.searchClientIps(chatId, email, callbackQuery.Message.GetMessageID())
				case "ips_cancel":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
					t.searchClientIps(chatId, email, callbackQuery.Message.GetMessageID())
				case "tgid_refresh":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.TGIdRefreshSuccess", "Email=="+email))
					t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.GetMessageID())
				case "tgid_cancel":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+email))
					t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.GetMessageID())
				case "reset_traffic":
					inlineKeyboard := tu.InlineKeyboard(
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmResetTraffic")).WithCallbackData(t.encodeQuery("reset_traffic_c "+email)),
						),
					)
					t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
				case "reset_traffic_c":
					err := t.inboundService.ResetClientTrafficByEmail(email)
					if err == nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.resetTrafficSuccess", "Email=="+email))
						t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					}
				case "limit_traffic":
					inlineKeyboard := tu.InlineKeyboard(
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 0")),
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" 0")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("1 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 1")),
							tu.InlineKeyboardButton("5 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 5")),
							tu.InlineKeyboardButton("10 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 10")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("20 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 20")),
							tu.InlineKeyboardButton("30 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 30")),
							tu.InlineKeyboardButton("40 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 40")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("50 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 50")),
							tu.InlineKeyboardButton("60 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 60")),
							tu.InlineKeyboardButton("80 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 80")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("100 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 100")),
							tu.InlineKeyboardButton("150 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 150")),
							tu.InlineKeyboardButton("200 GB").WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" 200")),
						),
					)
					t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
				case "limit_traffic_c":
					if len(dataArray) == 3 {
						limitTraffic, err := strconv.Atoi(dataArray[2])
						if err == nil {
							needRestart, err := t.inboundService.ResetClientTrafficLimitByEmail(email, limitTraffic)
							if needRestart {
								t.xrayService.SetToNeedRestart()
							}
							if err == nil {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.setTrafficLimitSuccess", "Email=="+email))
								t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
								return
							}
						}
					}
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "limit_traffic_in":
					if len(dataArray) >= 3 {
						oldInputNumber, err := strconv.Atoi(dataArray[2])
						inputNumber := oldInputNumber
						if err == nil {
							if len(dataArray) == 4 {
								num, err := strconv.Atoi(dataArray[3])
								if err == nil {
									switch num {
									case -2:
										inputNumber = 0
									case -1:
										if inputNumber > 0 {
											inputNumber = (inputNumber / 10)
										}
									default:
										inputNumber = (inputNumber * 10) + num
									}
								}
								if inputNumber == oldInputNumber {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
									return
								}
								if inputNumber >= 999999 {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
									return
								}
							}
							inlineKeyboard := tu.InlineKeyboard(
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumberAdd", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("limit_traffic_c "+email+" "+strconv.Itoa(inputNumber))),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 1")),
									tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 2")),
									tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 3")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 4")),
									tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 5")),
									tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 6")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 7")),
									tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 8")),
									tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 9")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" -2")),
									tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" 0")),
									tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("limit_traffic_in "+email+" "+strconv.Itoa(inputNumber)+" -1")),
								),
							)
							t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
							return
						}
					}
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "add_client_limit_traffic_c":
					limitTraffic, _ := strconv.Atoi(dataArray[1])
					client_TotalGB = int64(limitTraffic) * 1024 * 1024 * 1024
					messageId := callbackQuery.Message.GetMessageID()
					inbound, err := t.inboundService.GetInbound(receiver_inbound_ID)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					message_text, err := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}

					t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
				case "add_client_limit_traffic_in":
					if len(dataArray) >= 2 {
						oldInputNumber, err := strconv.Atoi(dataArray[1])
						inputNumber := oldInputNumber
						if err == nil {
							if len(dataArray) == 3 {
								num, err := strconv.Atoi(dataArray[2])
								if err == nil {
									switch num {
									case -2:
										inputNumber = 0
									case -1:
										if inputNumber > 0 {
											inputNumber = (inputNumber / 10)
										}
									default:
										inputNumber = (inputNumber * 10) + num
									}
								}
								if inputNumber == oldInputNumber {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
									return
								}
								if inputNumber >= 999999 {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
									return
								}
							}
							inlineKeyboard := tu.InlineKeyboard(
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumberAdd", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("add_client_limit_traffic_c "+strconv.Itoa(inputNumber))),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 1")),
									tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 2")),
									tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 3")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 4")),
									tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 5")),
									tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 6")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 7")),
									tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 8")),
									tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 9")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" -2")),
									tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" 0")),
									tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("add_client_limit_traffic_in "+strconv.Itoa(inputNumber)+" -1")),
								),
							)
							t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
							return
						}
					}
				case "reset_exp":
					inlineKeyboard := tu.InlineKeyboard(
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 0")),
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("reset_exp_in "+email+" 0")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 7 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 7")),
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 10 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 10")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 14 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 14")),
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 20 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 20")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 1 "+t.I18nBot("tgbot.month")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 30")),
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 3 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 90")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 6 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 180")),
							tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 12 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" 365")),
						),
					)
					t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
				case "reset_exp_c":
					if len(dataArray) == 3 {
						days, err := strconv.Atoi(dataArray[2])
						if err == nil {
							var date int64 = 0
							if days > 0 {
								traffic, err := t.inboundService.GetClientTrafficByEmail(email)
								if err != nil {
									logger.Warning(err)
									msg := t.I18nBot("tgbot.wentWrong")
									t.SendMsgToTgbot(chatId, msg)
									return
								}
								if traffic == nil {
									msg := t.I18nBot("tgbot.noResult")
									t.SendMsgToTgbot(chatId, msg)
									return
								}

								if traffic.ExpiryTime > 0 {
									if traffic.ExpiryTime-time.Now().Unix()*1000 < 0 {
										date = -int64(days * 24 * 60 * 60000)
									} else {
										date = traffic.ExpiryTime + int64(days*24*60*60000)
									}
								} else {
									date = traffic.ExpiryTime - int64(days*24*60*60000)
								}

							}
							needRestart, err := t.inboundService.ResetClientExpiryTimeByEmail(email, date)
							if needRestart {
								t.xrayService.SetToNeedRestart()
							}
							if err == nil {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.expireResetSuccess", "Email=="+email))
								t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
								return
							}
						}
					}
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "reset_exp_in":
					if len(dataArray) >= 3 {
						oldInputNumber, err := strconv.Atoi(dataArray[2])
						inputNumber := oldInputNumber
						if err == nil {
							if len(dataArray) == 4 {
								num, err := strconv.Atoi(dataArray[3])
								if err == nil {
									switch num {
									case -2:
										inputNumber = 0
									case -1:
										if inputNumber > 0 {
											inputNumber = (inputNumber / 10)
										}
									default:
										inputNumber = (inputNumber * 10) + num
									}
								}
								if inputNumber == oldInputNumber {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
									return
								}
								if inputNumber >= 999999 {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
									return
								}
							}
							inlineKeyboard := tu.InlineKeyboard(
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumber", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("reset_exp_c "+email+" "+strconv.Itoa(inputNumber))),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 1")),
									tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 2")),
									tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 3")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 4")),
									tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 5")),
									tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 6")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 7")),
									tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 8")),
									tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 9")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" -2")),
									tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" 0")),
									tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("reset_exp_in "+email+" "+strconv.Itoa(inputNumber)+" -1")),
								),
							)
							t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
							return
						}
					}
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "add_client_reset_exp_c":
					client_ExpiryTime = 0
					days, _ := strconv.Atoi(dataArray[1])
					var date int64
					if client_ExpiryTime > 0 {
						if client_ExpiryTime-time.Now().Unix()*1000 < 0 {
							date = -int64(days * 24 * 60 * 60000)
						} else {
							date = client_ExpiryTime + int64(days*24*60*60000)
						}
					} else {
						date = client_ExpiryTime - int64(days*24*60*60000)
					}
					client_ExpiryTime = date

					messageId := callbackQuery.Message.GetMessageID()
					inbound, err := t.inboundService.GetInbound(receiver_inbound_ID)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					message_text, err := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}

					t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
				case "add_client_reset_exp_in":
					if len(dataArray) >= 2 {
						oldInputNumber, err := strconv.Atoi(dataArray[1])
						inputNumber := oldInputNumber
						if err == nil {
							if len(dataArray) == 3 {
								num, err := strconv.Atoi(dataArray[2])
								if err == nil {
									switch num {
									case -2:
										inputNumber = 0
									case -1:
										if inputNumber > 0 {
											inputNumber = (inputNumber / 10)
										}
									default:
										inputNumber = (inputNumber * 10) + num
									}
								}
								if inputNumber == oldInputNumber {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
									return
								}
								if inputNumber >= 999999 {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
									return
								}
							}
							inlineKeyboard := tu.InlineKeyboard(
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumberAdd", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("add_client_reset_exp_c "+strconv.Itoa(inputNumber))),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 1")),
									tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 2")),
									tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 3")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 4")),
									tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 5")),
									tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 6")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 7")),
									tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 8")),
									tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 9")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" -2")),
									tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" 0")),
									tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("add_client_reset_exp_in "+strconv.Itoa(inputNumber)+" -1")),
								),
							)
							t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
							return
						}
					}
				case "ip_limit":
					inlineKeyboard := tu.InlineKeyboard(
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelIpLimit")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 0")),
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("ip_limit_in "+email+" 0")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 1")),
							tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 2")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 3")),
							tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 4")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 5")),
							tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 6")),
							tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 7")),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 8")),
							tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 9")),
							tu.InlineKeyboardButton("10").WithCallbackData(t.encodeQuery("ip_limit_c "+email+" 10")),
						),
					)
					t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
				case "ip_limit_c":
					if len(dataArray) == 3 {
						count, err := strconv.Atoi(dataArray[2])
						if err == nil {
							needRestart, err := t.inboundService.ResetClientIpLimitByEmail(email, count)
							if needRestart {
								t.xrayService.SetToNeedRestart()
							}
							if err == nil {
								t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.resetIpSuccess", "Email=="+email, "Count=="+strconv.Itoa(count)))
								t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
								return
							}
						}
					}
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "ip_limit_in":
					if len(dataArray) >= 3 {
						oldInputNumber, err := strconv.Atoi(dataArray[2])
						inputNumber := oldInputNumber
						if err == nil {
							if len(dataArray) == 4 {
								num, err := strconv.Atoi(dataArray[3])
								if err == nil {
									switch num {
									case -2:
										inputNumber = 0
									case -1:
										if inputNumber > 0 {
											inputNumber = (inputNumber / 10)
										}
									default:
										inputNumber = (inputNumber * 10) + num
									}
								}
								if inputNumber == oldInputNumber {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
									return
								}
								if inputNumber >= 999999 {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
									return
								}
							}
							inlineKeyboard := tu.InlineKeyboard(
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumber", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("ip_limit_c "+email+" "+strconv.Itoa(inputNumber))),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 1")),
									tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 2")),
									tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 3")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 4")),
									tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 5")),
									tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 6")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 7")),
									tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 8")),
									tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 9")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" -2")),
									tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" 0")),
									tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("ip_limit_in "+email+" "+strconv.Itoa(inputNumber)+" -1")),
								),
							)
							t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
							return
						}
					}
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
				case "add_client_ip_limit_c":
					if len(dataArray) == 2 {
						count, _ := strconv.Atoi(dataArray[1])
						client_LimitIP = count
					}

					messageId := callbackQuery.Message.GetMessageID()
					inbound, err := t.inboundService.GetInbound(receiver_inbound_ID)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					message_text, err := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}

					t.addClient(callbackQuery.Message.GetChat().ID, message_text, messageId)
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
				case "add_client_ip_limit_in":
					if len(dataArray) >= 2 {
						oldInputNumber, err := strconv.Atoi(dataArray[1])
						inputNumber := oldInputNumber
						if err == nil {
							if len(dataArray) == 3 {
								num, err := strconv.Atoi(dataArray[2])
								if err == nil {
									switch num {
									case -2:
										inputNumber = 0
									case -1:
										if inputNumber > 0 {
											inputNumber = (inputNumber / 10)
										}
									default:
										inputNumber = (inputNumber * 10) + num
									}
								}
								if inputNumber == oldInputNumber {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
									return
								}
								if inputNumber >= 999999 {
									t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
									return
								}
							}
							inlineKeyboard := tu.InlineKeyboard(
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_ip_limit")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmNumber", "Num=="+strconv.Itoa(inputNumber))).WithCallbackData(t.encodeQuery("add_client_ip_limit_c "+strconv.Itoa(inputNumber))),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 1")),
									tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 2")),
									tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 3")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 4")),
									tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 5")),
									tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 6")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 7")),
									tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 8")),
									tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 9")),
								),
								tu.InlineKeyboardRow(
									tu.InlineKeyboardButton("🔄").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" -2")),
									tu.InlineKeyboardButton("0").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" 0")),
									tu.InlineKeyboardButton("⬅️").WithCallbackData(t.encodeQuery("add_client_ip_limit_in "+strconv.Itoa(inputNumber)+" -1")),
								),
							)
							t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
							return
						}
					}
				case "clear_ips":
					inlineKeyboard := tu.InlineKeyboard(
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("ips_cancel "+email)),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmClearIps")).WithCallbackData(t.encodeQuery("clear_ips_c "+email)),
						),
					)
					t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
				case "clear_ips_c":
					err := t.inboundService.ClearClientIps(email)
					if err == nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.clearIpSuccess", "Email=="+email))
						t.searchClientIps(chatId, email, callbackQuery.Message.GetMessageID())
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					}
				case "ip_log":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getIpLog", "Email=="+email))
					t.searchClientIps(chatId, email)
				case "tg_user":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.getUserInfo", "Email=="+email))
					t.clientTelegramUserInfo(chatId, email)
				case "tgid_remove":
					inlineKeyboard := tu.InlineKeyboard(
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("tgid_cancel "+email)),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmRemoveTGUser")).WithCallbackData(t.encodeQuery("tgid_remove_c "+email)),
						),
					)
					t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
				case "tgid_remove_c":
					traffic, err := t.inboundService.GetClientTrafficByEmail(email)
					if err != nil || traffic == nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
						return
					}
					needRestart, err := t.inboundService.SetClientTelegramUserID(traffic.Id, EmptyTelegramUserID)
					if needRestart {
						t.xrayService.SetToNeedRestart()
					}
					if err == nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.removedTGUserSuccess", "Email=="+email))
						t.clientTelegramUserInfo(chatId, email, callbackQuery.Message.GetMessageID())
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					}
				case "toggle_enable":
					inlineKeyboard := tu.InlineKeyboard(
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("client_cancel "+email)),
						),
						tu.InlineKeyboardRow(
							tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmToggle")).WithCallbackData(t.encodeQuery("toggle_enable_c "+email)),
						),
					)
					t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
				case "toggle_enable_c":
					enabled, needRestart, err := t.inboundService.ToggleClientEnableByEmail(email)
					if needRestart {
						t.xrayService.SetToNeedRestart()
					}
					if err == nil {
						if enabled {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.enableSuccess", "Email=="+email))
						} else {
							t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.disableSuccess", "Email=="+email))
						}
						t.searchClient(chatId, email, callbackQuery.Message.GetMessageID())
					} else {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.errorOperation"))
					}
				case "get_clients":
					inboundId := dataArray[1]
					inboundIdInt, err := strconv.Atoi(inboundId)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					inbound, err := t.inboundService.GetInbound(inboundIdInt)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					clients, err := t.getInboundClients(inboundIdInt)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), clients)
				case "log_settings":
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "📝 正在打开日志设置...")
					t.showLogSettings(chatId)
				case "add_client_to":
					// assign default values to clients variables
					client_Id = uuid.New().String()
					client_Flow = ""
					client_Email = t.randomLowerAndNum(8)
					client_LimitIP = 0
					client_TotalGB = 0
					client_ExpiryTime = 0
					client_Enable = true
					client_TgID = ""
					client_SubID = t.randomLowerAndNum(16)
					client_Comment = ""
					client_Reset = 0
					client_Security = "auto"
					client_ShPassword = t.randomShadowSocksPassword()
					client_TrPassword = t.randomLowerAndNum(10)
					client_Method = ""

					inboundId := dataArray[1]
					inboundIdInt, err := strconv.Atoi(inboundId)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					receiver_inbound_ID = inboundIdInt
					inbound, err := t.inboundService.GetInbound(inboundIdInt)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}

					message_text, err := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}

					t.addClient(callbackQuery.Message.GetChat().ID, message_text)
				}
				return
			}

			// 【修复】: 统一使用 decodedQuery 进行 switch 判断，确保哈希策略变更时的兼容性
			switch decodedQuery {
			case "get_inbounds":
				inbounds, err := t.getInbounds()
				if err != nil {
					t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
					return

				}
				t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.allClients"))
				t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), inbounds)
			}

		}
	}

	// 【修复】: 统一使用 decodedQuery 进行 switch 判断
	// 先解码 callbackQuery.Data（对于非管理员用户也需要解码）
	decodedQueryForAll, decodeErr := t.decodeQuery(callbackQuery.Data)
	if decodeErr != nil {
		decodedQueryForAll = callbackQuery.Data // 如果解码失败，使用原始数据
	}

	switch decodedQueryForAll {
	case "get_usage":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.serverUsage"))
		t.getServerUsage(chatId)
	case "usage_refresh":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
		t.getServerUsage(chatId, callbackQuery.Message.GetMessageID())
	case "inbounds":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.getInbounds"))
		t.SendMsgToTgbot(chatId, t.getInboundUsages())
	case "deplete_soon":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.depleteSoon"))
		t.getExhausted(chatId)
	case "get_backup":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.dbBackup"))
		t.sendBackup(chatId)
	case "get_banlogs":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.getBanLogs"))
		t.sendBanLogs(chatId, true)
	case "client_traffic":
		tgUserID := callbackQuery.From.ID
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.clientUsage"))
		t.getClientUsage(chatId, tgUserID)
	case "client_commands":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.commands"))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.helpClientCommands"))
	case "onlines":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.onlines"))
		t.onlineClients(chatId)
	case "onlines_refresh":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.successfulOperation"))
		t.onlineClients(chatId, callbackQuery.Message.GetMessageID())
	case "commands":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.commands"))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.commands.helpAdminCommands"))
	case "add_client":
		// assign default values to clients variables
		client_Id = uuid.New().String()
		client_Flow = ""
		client_Email = t.randomLowerAndNum(8)
		client_LimitIP = 0
		client_TotalGB = 0
		client_ExpiryTime = 0
		client_Enable = true
		client_TgID = ""
		client_SubID = t.randomLowerAndNum(16)
		client_Comment = ""
		client_Reset = 0
		client_Security = "auto"
		client_ShPassword = t.randomShadowSocksPassword()
		client_TrPassword = t.randomLowerAndNum(10)
		client_Method = ""

		inbounds, err := t.getInboundsAddClient()
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.buttons.addClient"))
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseInbound"), inbounds)
	case "add_client_ch_default_email":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_email"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.email_prompt", "ClientEmail=="+client_Email)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_id":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_id"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.id_prompt", "ClientId=="+client_Id)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_pass_tr":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_password_tr"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.pass_prompt", "ClientPassword=="+client_TrPassword)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_pass_sh":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_password_sh"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.pass_prompt", "ClientPassword=="+client_ShPassword)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_comment":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		userStates[chatId] = "awaiting_comment"
		cancel_btn_markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.use_default")).WithCallbackData("add_client_default_info"),
			),
		)
		prompt_message := t.I18nBot("tgbot.messages.comment_prompt", "ClientComment=="+client_Comment)
		t.SendMsgToTgbot(chatId, prompt_message, cancel_btn_markup)
	case "add_client_ch_default_traffic":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_limit_traffic_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 1")),
				tu.InlineKeyboardButton("5 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 5")),
				tu.InlineKeyboardButton("10 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 10")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("20 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 20")),
				tu.InlineKeyboardButton("30 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 30")),
				tu.InlineKeyboardButton("40 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 40")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("50 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 50")),
				tu.InlineKeyboardButton("60 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 60")),
				tu.InlineKeyboardButton("80 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 80")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("100 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 100")),
				tu.InlineKeyboardButton("150 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 150")),
				tu.InlineKeyboardButton("200 GB").WithCallbackData(t.encodeQuery("add_client_limit_traffic_c 200")),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
	case "add_client_ch_default_exp":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_traffic_exp")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_reset_exp_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 7 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 7")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 10 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 10")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 14 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 14")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 20 "+t.I18nBot("tgbot.days")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 20")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 1 "+t.I18nBot("tgbot.month")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 30")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 3 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 90")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 6 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 180")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.add")+" 12 "+t.I18nBot("tgbot.months")).WithCallbackData(t.encodeQuery("add_client_reset_exp_c 365")),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
	case "add_client_ch_default_ip_limit":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData(t.encodeQuery("add_client_default_ip_limit")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.unlimited")).WithCallbackData(t.encodeQuery("add_client_ip_limit_c 0")),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.custom")).WithCallbackData(t.encodeQuery("add_client_ip_limit_in 0")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("1").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 1")),
				tu.InlineKeyboardButton("2").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 2")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("3").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 3")),
				tu.InlineKeyboardButton("4").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 4")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("5").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 5")),
				tu.InlineKeyboardButton("6").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 6")),
				tu.InlineKeyboardButton("7").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 7")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("8").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 8")),
				tu.InlineKeyboardButton("9").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 9")),
				tu.InlineKeyboardButton("10").WithCallbackData(t.encodeQuery("add_client_ip_limit_c 10")),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), inlineKeyboard)
	case "add_client_default_info":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.SendMsgToTgbotDeleteAfter(chatId, t.I18nBot("tgbot.messages.using_default_value"), 3, tu.ReplyKeyboardRemove())
		delete(userStates, chatId)
		inbound, _ := t.inboundService.GetInbound(receiver_inbound_ID)
		message_text, _ := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
		t.addClient(chatId, message_text)
	case "add_client_cancel":
		delete(userStates, chatId)
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.SendMsgToTgbotDeleteAfter(chatId, t.I18nBot("tgbot.messages.cancel"), 3, tu.ReplyKeyboardRemove())
	case "add_client_default_traffic_exp":
		messageId := callbackQuery.Message.GetMessageID()
		inbound, err := t.inboundService.GetInbound(receiver_inbound_ID)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		message_text, err := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		t.addClient(chatId, message_text, messageId)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+client_Email))
	case "add_client_default_ip_limit":
		messageId := callbackQuery.Message.GetMessageID()
		inbound, err := t.inboundService.GetInbound(receiver_inbound_ID)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		message_text, err := t.BuildInboundClientDataMessage(inbound.Remark, inbound.Protocol)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
			return
		}
		t.addClient(chatId, message_text, messageId)
		t.sendCallbackAnswerTgBot(callbackQuery.ID, t.I18nBot("tgbot.answers.canceled", "Email=="+client_Email))
	case "add_client_submit_disable":
		client_Enable = false
		_, err := t.SubmitAddClient()
		if err != nil {
			errorMessage := fmt.Sprintf("%v", err)
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.error_add_client", "error=="+errorMessage), tu.ReplyKeyboardRemove())
		} else {
			t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.successfulOperation"), tu.ReplyKeyboardRemove())
		}
	case "add_client_submit_enable":
		client_Enable = true
		_, err := t.SubmitAddClient()
		if err != nil {
			errorMessage := fmt.Sprintf("%v", err)
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.error_add_client", "error=="+errorMessage), tu.ReplyKeyboardRemove())
		} else {
			t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.successfulOperation"), tu.ReplyKeyboardRemove())
		}
	case "reset_all_traffics_cancel":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.SendMsgToTgbotDeleteAfter(chatId, t.I18nBot("tgbot.messages.cancel"), 1, tu.ReplyKeyboardRemove())
	case "reset_all_traffics":
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancelReset")).WithCallbackData(t.encodeQuery("reset_all_traffics_cancel")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.confirmResetTraffic")).WithCallbackData(t.encodeQuery("reset_all_traffics_c")),
			),
		)
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.AreYouSure"), inlineKeyboard)
	case "reset_all_traffics_c":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		emails, err := t.inboundService.getAllEmails()
		if err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"), tu.ReplyKeyboardRemove())
			return
		}

		for _, email := range emails {
			err := t.inboundService.ResetClientTrafficByEmail(email)
			if err == nil {
				msg := t.I18nBot("tgbot.messages.SuccessResetTraffic", "ClientEmail=="+email)
				t.SendMsgToTgbot(chatId, msg, tu.ReplyKeyboardRemove())
			} else {
				msg := t.I18nBot("tgbot.messages.FailedResetTraffic", "ClientEmail=="+email, "ErrorMessage=="+err.Error())
				t.SendMsgToTgbot(chatId, msg, tu.ReplyKeyboardRemove())
			}
		}

		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.messages.FinishProcess"), tu.ReplyKeyboardRemove())
	case "get_sorted_traffic_usage_report":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		emails, err := t.inboundService.getAllEmails()
		if err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"), tu.ReplyKeyboardRemove())
			return
		}
		valid_emails, extra_emails, err := t.inboundService.FilterAndSortClientEmails(emails)
		if err != nil {
			t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.errorOperation"), tu.ReplyKeyboardRemove())
			return
		}

		for _, valid_emails := range valid_emails {
			traffic, err := t.inboundService.GetClientTrafficByEmail(valid_emails)
			if err != nil {
				logger.Warning(err)
				msg := t.I18nBot("tgbot.wentWrong")
				t.SendMsgToTgbot(chatId, msg)
				continue
			}
			if traffic == nil {
				msg := t.I18nBot("tgbot.noResult")
				t.SendMsgToTgbot(chatId, msg)
				continue
			}

			output := t.clientInfoMsg(traffic, false, false, false, false, true, false)
			t.SendMsgToTgbot(chatId, output, tu.ReplyKeyboardRemove())
		}
		for _, extra_emails := range extra_emails {
			msg := fmt.Sprintf("📧 %s\n%s", extra_emails, t.I18nBot("tgbot.noResult"))
			t.SendMsgToTgbot(chatId, msg, tu.ReplyKeyboardRemove())

		}

	// 【重构后】: 处理分层菜单的回调
	case "oneclick_options":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "请选择配置类型...")
		t.sendOneClickOptions(chatId)

	case "oneclick_category_relay":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "正在进入中转类别...")
		t.sendRelayOptions(chatId)

	case "oneclick_category_direct":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "正在进入直连类别...")
		t.sendDirectOptions(chatId)

	case "oneclick_reality":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🚀 正在创建 Vless + TCP + Reality 节点...")
		t.SendMsgToTgbot(chatId, "🚀 正在远程创建  ------->>>>\n\n【Vless + TCP + Reality】节点，请稍候......")
		t.remoteCreateOneClickInbound("reality", chatId)

	case "oneclick_xhttp_reality":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "⚡ 正在创建 Vless + XHTTP + Reality 节点...")
		t.SendMsgToTgbot(chatId, "⚡ 正在远程创建  ------->>>>\n\n【Vless + XHTTP + Reality】节点，请稍候......")
		t.remoteCreateOneClickInbound("xhttp_reality", chatId)

	case "oneclick_tls":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🛡️ 正在创建 Vless Encryption + XHTTP + TLS 节点...")
		t.SendMsgToTgbot(chatId, "🛡️ 正在远程创建  ------->>>>\n\n【Vless Encryption + XHTTP + TLS】节点，请稍候......")
		t.remoteCreateOneClickInbound("tls", chatId)

	case "oneclick_switch_vision":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🌀 Switch + Vision Seed 协议组合的功能还在开发中 ...........")
		t.SendMsgToTgbot(chatId, "🌀 Switch + Vision Seed 协议组合的功能还在开发中 ........")
		t.remoteCreateOneClickInbound("switch_vision", chatId)

	// 〔中文注释〕: 【新增回调处理】 - 重启面板、娱乐抽奖、VPS推荐
	case "restart_panel":
		// 〔中文注释〕: 用户从菜单点击重启，删除主菜单并发送确认消息
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "请确认操作")
		confirmKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("✅ 是，立即重启").WithCallbackData(t.encodeQuery("restart_panel_confirm")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("❌ 否，我再想想").WithCallbackData(t.encodeQuery("restart_panel_cancel")),
			),
		)
		t.SendMsgToTgbot(chatId, "🤔 您“现在的操作”是要确定进行，\n\n重启〔X-Panel 面板〕服务吗？\n\n这也会同时重启 Xray Core，\n\n会使面板在短时间内无法访问。", confirmKeyboard)

	case "restart_panel_confirm":
		// 〔中文注释〕: 用户确认重启
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "指令已发送，请稍候...")
		t.SendMsgToTgbot(chatId, "⏳ 【重启命令】已在 VPS 中远程执行，\n\n正在等待面板恢复（约30秒），并进行验证检查...")

		// 〔中文注释〕: 在后台协程中执行重启，避免阻塞机器人
		go func() {
			err := t.serverService.RestartPanel()
			// 〔中文注释〕: 等待20秒，让面板有足够的时间重启
			time.Sleep(20 * time.Second)
			if err != nil {
				// 〔中文注释〕: 如果执行出错，发送失败消息
				t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 面板重启命令执行失败！\n\n错误信息已记录到日志，请检查命令或权限。\n\n`%v`", err))
			} else {
				// 〔中文注释〕: 执行成功，发送成功消息
				t.SendMsgToTgbot(chatId, "🚀 面板重启成功！服务已成功恢复！")
			}
		}()

	case "restart_panel_cancel":
		// 〔中文注释〕: 用户取消重启
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "操作已取消")
		// 〔中文注释〕: 发送一个临时消息提示用户，3秒后自动删除
		t.SendMsgToTgbotDeleteAfter(chatId, "已取消重启操作。", 3)

	case "vps_recommend":
		// VPS推荐功能已移除
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "VPS推荐功能已移除")

	// 【新增代码】: 处理 Xray 版本管理相关回调
	case "xrayversion":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🚀 请选择要更新的版本...")
		t.sendXrayVersionOptions(chatId)

	case "update_xray_ask":
		// 处理 Xray 版本更新请求
		tempDataArray := strings.Split(decodedQueryForAll, " ")
		if len(tempDataArray) >= 2 && len(tempDataArray[1]) > 0 {
			version := tempDataArray[1]
			confirmKeyboard := tu.InlineKeyboard(
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("✅ 确认更新").WithCallbackData(t.encodeQuery(fmt.Sprintf("update_xray_confirm %s", version))),
				),
				tu.InlineKeyboardRow(
					tu.InlineKeyboardButton("❌ 取消").WithCallbackData(t.encodeQuery("update_xray_cancel")),
				),
			)
			t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), confirmKeyboard)
		}

	case "update_xray_confirm":
		// 处理 Xray 版本更新确认
		tempDataArray := strings.Split(decodedQueryForAll, " ")
		if len(tempDataArray) >= 2 && len(tempDataArray[1]) > 0 {
			version := tempDataArray[1]
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "正在启动 Xray 更新任务...")
			t.SendMsgToTgbot(chatId, fmt.Sprintf("🚀 正在更新 Xray 到版本 %s，更新任务已在后台启动...", version))
			go func() {
				err := t.serverService.UpdateXray(version)
				if err != nil {
					t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ Xray 更新失败: %v", err))
				} else {
					t.SendMsgToTgbot(chatId, fmt.Sprintf("✅ Xray 成功更新到版本 %s", version))
				}
			}()
		}

	case "update_xray_cancel":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "已取消")
		return
	case "copy_all_links":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "📋 正在生成所有客户端链接...")
		err := t.copyAllLinks(chatId)
		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 生成链接失败: %v", err))
		}
		return

	// 【新增代码】: 处理机器优化一键方案相关回调
	case "machine_optimization":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "⚡ 正在打开机器优化选项...")
		t.sendMachineOptimizationOptions(chatId)

	case "optimize_1c1g":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🖥️ 正在打开1C1G优化选项...")
		t.performOptimization1C1G(chatId, callbackQuery.Message.GetMessageID())

	case "optimize_1c1g_confirm":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🚀 正在执行1C1G优化...")
		t.executeOptimization1C1G(chatId, callbackQuery.Message.GetMessageID())

	case "optimize_generic":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🚀 正在执行通用/高配优化...")
		t.executeGenericOptimization(chatId, callbackQuery.Message.GetMessageID())

	// 【新增代码】: 处理防火墙管理相关回调
	case "firewall_menu":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🔥 正在打开防火墙管理菜单...")
		t.sendFirewallMenu(chatId)

	// 【新增代码】: 处理程序更新相关回调
	case "check_panel_update":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🔄 正在检查最新版本...")
		t.checkPanelUpdate(chatId)

	case "confirm_panel_update":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "✅ 更新指令已发送")
		t.SendMsgToTgbot(chatId, "🔄 **X-Panel 更新任务已在后台启动**\n\n⏳ 请稍候，更新完成后将收到通知...")
		err := t.serverService.UpdatePanel("")
		if err != nil {
			t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 发送更新指令失败: %v", err))
		}

	case "cancel_panel_update":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "已取消")
		t.SendMsgToTgbotDeleteAfter(chatId, "已取消面板更新操作。", 3)

	case "update_geodata_ask":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🌍 准备更新 Geo 数据...")
		confirmKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("✅ 确认更新").WithCallbackData(t.encodeQuery("update_geodata_confirm")),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton("❌ 取消").WithCallbackData(t.encodeQuery("update_geodata_cancel")),
			),
		)
		t.editMessageCallbackTgBot(chatId, callbackQuery.Message.GetMessageID(), confirmKeyboard)
		text := "🌍 **Geo 数据更新确认**\n\n" +
			"这将从官方源下载最新的 GeoIP 和 GeoSite 数据，并自动重启 Xray 服务。\n\n" +
			"⚠️ **注意：**\n" +
			"• 更新期间 Xray 服务会短暂中断\n" +
			"• 下载可能需要一些时间，请耐心等待\n\n" +
			"确认要继续吗？"
		t.SendMsgToTgbot(chatId, text, confirmKeyboard)

	case "firewall_check_status":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🔍 正在检测防火墙状态...")
		t.checkFirewallStatus(chatId)

	case "firewall_install_firewalld":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "📦 正在安装 Firewalld...")
		t.installFirewalld(chatId)

	case "firewall_install_fail2ban":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "📦 正在安装 Fail2Ban...")
		t.installFail2Ban(chatId)

	case "firewall_enable":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "✅ 正在启用防火墙...")
		t.enableFirewall(chatId)

	case "firewall_disable":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 正在禁用防火墙...")
		t.disableFirewall(chatId)

	case "firewall_open_port":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🔓 正在开放端口...")
		t.openPort(chatId)

	case "firewall_close_port":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🔒 正在关闭端口...")
		t.closePort(chatId)

	case "firewall_list_rules":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "📋 正在获取规则列表...")
		t.listFirewallRules(chatId)

	case "firewall_open_xpanel_ports":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🚀 正在开放 X-Panel 端口...")
		t.openXPanelPorts(chatId)

	// 【新增代码】: 处理 Geo 数据更新相关回调
	case "update_geodata_confirm":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "✅ 指令已发送")
		t.SendMsgToTgbot(chatId, "🌍 **Geo 数据更新任务已在后台启动**\n\n⏳ 请稍候，更新完成后将收到通知...")

		// 调用 ServerService 的 UpdateGeoData 方法
		if t.serverService != nil {
			err := t.serverService.UpdateGeoData()
			if err != nil {
				t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 发送 Geo 数据更新指令失败: %v", err))
			}
		} else {
			t.SendMsgToTgbot(chatId, "❌ 服务未初始化，无法执行更新")
		}

	case "update_geodata_cancel":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "已取消")
		t.SendMsgToTgbotDeleteAfter(chatId, "已取消 Geo 数据更新操作。", 3)

	// 日志设置相关回调
	case "log_settings":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "📝 正在打开日志设置...")
		t.showLogSettings(chatId)

	case "toggle_local_log":
		current, err := t.settingService.GetLocalLogEnabled()
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 获取状态失败")
			return
		}
		err = t.settingService.SetLocalLogEnabled(!current)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 设置失败")
			return
		}
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "✅ 已切换本地日志状态")
		t.showLogSettings(chatId)

	case "cycle_log_level":
		current, err := t.settingService.GetTgLogLevel()
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 获取级别失败")
			return
		}
		var newLevel string
		switch current {
		case "info":
			newLevel = "warn"
		case "warn":
			newLevel = "error"
		case "error":
			newLevel = "info"
		default:
			newLevel = "warn"
		}
		err = t.settingService.SetTgLogLevel(newLevel)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 设置失败")
			return
		}
		t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ 日志级别已设置为 %s", newLevel))
		t.showLogSettings(chatId)

	case "set_log_level":
		// 解析级别参数
		tempDataArray := strings.Split(decodedQueryForAll, " ")
		if len(tempDataArray) < 2 {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 参数错误")
			return
		}
		newLevel := tempDataArray[1]
		// 验证级别
		validLevels := map[string]bool{"error": true, "warn": true, "warning": true, "info": true, "debug": true}
		if !validLevels[newLevel] {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 无效的日志级别")
			return
		}
		// 标准化级别名称
		if newLevel == "warning" {
			newLevel = "warn"
		}
		err := t.settingService.SetTgLogLevel(newLevel)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 设置失败")
			return
		}
		t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("✅ 日志级别已设置为 %s", newLevel))
		t.showLogSettings(chatId)

	case "back_to_main":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "返回主菜单")
		t.SendAnswer(chatId, "请选择操作:", true)
	case "fetch_logs":
		// 解析数量参数
		tempDataArray := strings.Split(decodedQueryForAll, " ")
		count := 20 // 默认
		if len(tempDataArray) > 1 {
			if c, err := strconv.Atoi(tempDataArray[1]); err == nil && c > 0 {
				count = c
			}
		}
		t.sendCallbackAnswerTgBot(callbackQuery.ID, fmt.Sprintf("📄 获取最近 %d 条日志...", count))
		// 获取配置的日志级别
		level, err := t.settingService.GetTgLogLevel()
		if err != nil {
			level = "info" // 默认级别
		}
		logs := logger.GetLogs(count, level)
		if len(logs) == 0 {
			t.SendMsgToTgbot(chatId, "📋 **最近日志**\n\n❌ 未找到符合级别的日志记录")
		} else {
			content := strings.Join(logs, "\n")
			t.sendLongMessage(chatId, content)
		}
	case "toggle_log_forward":
		current, err := t.settingService.GetTgLogForwardEnabled()
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 获取状态失败")
			return
		}
		err = t.settingService.SetTgLogForwardEnabled(!current)
		if err != nil {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "❌ 设置失败")
			return
		}
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "✅ 已切换 TG 转发状态")
		t.showLogMenu(chatId)

	case "close_menu":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "已关闭菜单")
	}
}

func (t *Tgbot) BuildInboundClientDataMessage(inbound_remark string, protocol model.Protocol) (string, error) {
	var message string

	currentTime := time.Now()
	timestampMillis := currentTime.UnixNano() / int64(time.Millisecond)

	expiryTime := ""
	diff := client_ExpiryTime/1000 - timestampMillis
	if client_ExpiryTime == 0 {
		expiryTime = t.I18nBot("tgbot.unlimited")
	} else if diff > 172800 {
		expiryTime = time.Unix((client_ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
	} else if client_ExpiryTime < 0 {
		expiryTime = fmt.Sprintf("%d %s", client_ExpiryTime/-86400000, t.I18nBot("tgbot.days"))
	} else {
		expiryTime = fmt.Sprintf("%d %s", diff/3600, t.I18nBot("tgbot.hours"))
	}

	traffic_value := ""
	if client_TotalGB == 0 {
		traffic_value = "♾️ Unlimited(Reset)"
	} else {
		traffic_value = common.FormatTraffic(client_TotalGB)
	}

	ip_limit := ""
	if client_LimitIP == 0 {
		ip_limit = "♾️ Unlimited(Reset)"
	} else {
		ip_limit = fmt.Sprint(client_LimitIP)
	}

	switch protocol {
	case model.VMESS, model.VLESS:
		message = t.I18nBot("tgbot.messages.inbound_client_data_id", "InboundRemark=="+inbound_remark, "ClientId=="+client_Id, "ClientEmail=="+client_Email, "ClientTraffic=="+traffic_value, "ClientExp=="+expiryTime, "IpLimit=="+ip_limit, "ClientComment=="+client_Comment)

	case model.Trojan:
		message = t.I18nBot("tgbot.messages.inbound_client_data_pass", "InboundRemark=="+inbound_remark, "ClientPass=="+client_TrPassword, "ClientEmail=="+client_Email, "ClientTraffic=="+traffic_value, "ClientExp=="+expiryTime, "IpLimit=="+ip_limit, "ClientComment=="+client_Comment)

	case model.Shadowsocks:
		message = t.I18nBot("tgbot.messages.inbound_client_data_pass", "InboundRemark=="+inbound_remark, "ClientPass=="+client_ShPassword, "ClientEmail=="+client_Email, "ClientTraffic=="+traffic_value, "ClientExp=="+expiryTime, "IpLimit=="+ip_limit, "ClientComment=="+client_Comment)

	default:
		return "", errors.New("unknown protocol")
	}

	return message, nil
}

func (t *Tgbot) BuildJSONForProtocol(protocol model.Protocol) (string, error) {
	var jsonString string

	switch protocol {
	case model.VMESS:
		jsonString = fmt.Sprintf(`{
            "clients": [{
                "id": "%s",
                "security": "%s",
                "email": "%s",
                "limitIp": %d,
                "totalGB": %d,
                "expiryTime": %d,
                "enable": %t,
                "tgId": "%s",
                "subId": "%s",
                "comment": "%s",
                "reset": %d
            }]
        }`, client_Id, client_Security, client_Email, client_LimitIP, client_TotalGB, client_ExpiryTime, client_Enable, client_TgID, client_SubID, client_Comment, client_Reset)

	case model.VLESS:
		jsonString = fmt.Sprintf(`{
            "clients": [{
                "id": "%s",
                "flow": "%s",
                "email": "%s",
                "limitIp": %d,
                "totalGB": %d,
                "expiryTime": %d,
                "enable": %t,
                "tgId": "%s",
                "subId": "%s",
                "comment": "%s",
                "reset": %d
            }]
        }`, client_Id, client_Flow, client_Email, client_LimitIP, client_TotalGB, client_ExpiryTime, client_Enable, client_TgID, client_SubID, client_Comment, client_Reset)

	case model.Trojan:
		jsonString = fmt.Sprintf(`{
            "clients": [{
                "password": "%s",
                "email": "%s",
                "limitIp": %d,
                "totalGB": %d,
                "expiryTime": %d,
                "enable": %t,
                "tgId": "%s",
                "subId": "%s",
                "comment": "%s",
                "reset": %d
            }]
        }`, client_TrPassword, client_Email, client_LimitIP, client_TotalGB, client_ExpiryTime, client_Enable, client_TgID, client_SubID, client_Comment, client_Reset)

	case model.Shadowsocks:
		jsonString = fmt.Sprintf(`{
            "clients": [{
                "method": "%s",
                "password": "%s",
                "email": "%s",
                "limitIp": %d,
                "totalGB": %d,
                "expiryTime": %d,
                "enable": %t,
                "tgId": "%s",
                "subId": "%s",
                "comment": "%s",
                "reset": %d
            }]
        }`, client_Method, client_ShPassword, client_Email, client_LimitIP, client_TotalGB, client_ExpiryTime, client_Enable, client_TgID, client_SubID, client_Comment, client_Reset)

	default:
		return "", errors.New("unknown protocol")
	}

	return jsonString, nil
}

func (t *Tgbot) SubmitAddClient() (bool, error) {
	inbound, err := t.inboundService.GetInbound(receiver_inbound_ID)
	if err != nil {
		logger.Warning("getIboundClients run failed:", err)
		return false, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	jsonString, err := t.BuildJSONForProtocol(inbound.Protocol)
	if err != nil {
		logger.Warning("BuildJSONForProtocol run failed:", err)
		return false, errors.New("failed to build JSON for protocol")
	}

	newInbound := &model.Inbound{
		Id:       receiver_inbound_ID,
		Settings: jsonString,
	}

	return t.inboundService.AddInboundClient(newInbound)
}

func checkAdmin(tgId int64) bool {
	for _, adminId := range adminIds {
		if adminId == tgId {
			return true
		}
	}
	return false
}

func (t *Tgbot) SendAnswer(chatId int64, msg string, isAdmin bool) {
	numericKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.serverUsage")).WithCallbackData(t.encodeQuery("get_usage")),
			tu.InlineKeyboardButton("♻️ 重启面板").WithCallbackData(t.encodeQuery("restart_panel")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.SortedTrafficUsageReport")).WithCallbackData(t.encodeQuery("get_sorted_traffic_usage_report")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.ResetAllTraffics")).WithCallbackData(t.encodeQuery("reset_all_traffics")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.dbBackup")).WithCallbackData(t.encodeQuery("get_backup")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getBanLogs")).WithCallbackData(t.encodeQuery("get_banlogs")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.getInbounds")).WithCallbackData(t.encodeQuery("inbounds")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.depleteSoon")).WithCallbackData(t.encodeQuery("deplete_soon")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.commands")).WithCallbackData(t.encodeQuery("commands")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.onlines")).WithCallbackData(t.encodeQuery("onlines")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.allClients")).WithCallbackData(t.encodeQuery("get_inbounds")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.addClient")).WithCallbackData(t.encodeQuery("add_client")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📋 批量复制链接").WithCallbackData(t.encodeQuery("copy_all_links")),
		),

		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🆕 Xray 版本管理").WithCallbackData(t.encodeQuery("xrayversion")),
			tu.InlineKeyboardButton("🔥 防火墙").WithCallbackData(t.encodeQuery("firewall_menu")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔄 程序更新").WithCallbackData(t.encodeQuery("check_panel_update")),
			tu.InlineKeyboardButton("⚡ 机器优化一键方案").WithCallbackData(t.encodeQuery("machine_optimization")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("📝 日志设置").WithCallbackData(t.encodeQuery("log_settings")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🌍 更新 Geo 数据").WithCallbackData(t.encodeQuery("update_geodata_ask")),
		),
		// VPS推荐按钮已移除
		// TODOOOOOOOOOOOOOO: Add restart button here.
	)
	numericKeyboardClient := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clientUsage")).WithCallbackData(t.encodeQuery("client_traffic")),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.commands")).WithCallbackData(t.encodeQuery("client_commands")),
		),
	)

	var ReplyMarkup telego.ReplyMarkup
	if isAdmin {
		ReplyMarkup = numericKeyboard
	} else {
		ReplyMarkup = numericKeyboardClient
	}
	t.SendMsgToTgbot(chatId, msg, ReplyMarkup)
}

func (t *Tgbot) SendMsgToTgbot(chatId int64, msg string, replyMarkup ...telego.ReplyMarkup) {
	if !isRunning {
		return
	}

	if msg == "" {
		logger.Info("[tgbot] message is empty!")
		return
	}

	var allMessages []string
	limit := 2000

	// paging message if it is big
	if len(msg) > limit {
		messages := strings.Split(msg, "\r\n\r\n")
		lastIndex := -1

		for _, message := range messages {
			if (len(allMessages) == 0) || (len(allMessages[lastIndex])+len(message) > limit) {
				allMessages = append(allMessages, message)
				lastIndex++
			} else {
				allMessages[lastIndex] += "\r\n\r\n" + message
			}
		}
		if strings.TrimSpace(allMessages[len(allMessages)-1]) == "" {
			allMessages = allMessages[:len(allMessages)-1]
		}
	} else {
		allMessages = append(allMessages, msg)
	}
	for n, message := range allMessages {
		params := telego.SendMessageParams{
			ChatID:    tu.ID(chatId),
			Text:      message,
			ParseMode: "HTML",
		}
		// only add replyMarkup to last message
		if len(replyMarkup) > 0 && n == (len(allMessages)-1) {
			params.ReplyMarkup = replyMarkup[0]
		}
		_, err := bot.SendMessage(context.Background(), &params)
		if err != nil {
			logger.Warning("Error sending telegram message :", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (t *Tgbot) SendMsgToTgbotAdmins(msg string, replyMarkup ...telego.ReplyMarkup) {
	if len(replyMarkup) > 0 {
		for _, adminId := range adminIds {
			t.SendMsgToTgbot(adminId, msg, replyMarkup[0])
		}
	} else {
		for _, adminId := range adminIds {
			t.SendMsgToTgbot(adminId, msg)
		}
	}
}

func (t *Tgbot) SendBackupToAdmins() {
	if !t.IsRunning() {
		return
	}
	for _, adminId := range adminIds {
		t.sendBackup(int64(adminId))
	}
}

func (t *Tgbot) getServerUsage(chatId int64, messageID ...int) string {
	info := t.prepareServerUsageInfo()

	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("usage_refresh"))))

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], info, keyboard)
	} else {
		t.SendMsgToTgbot(chatId, info, keyboard)
	}

	return info
}

func (t *Tgbot) prepareServerUsageInfo() string {
	info, ipv4, ipv6 := "", "", ""

	// get latest status of server
	t.lastStatus = t.serverService.GetStatus(t.lastStatus)
	onlines := p.GetOnlineClients()

	info += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	info += t.I18nBot("tgbot.messages.version", "Version=="+config.GetVersion())
	info += t.I18nBot("tgbot.messages.xrayVersion", "XrayVersion=="+fmt.Sprint(t.lastStatus.Xray.Version))

	// get ip address
	netInterfaces, err := net.Interfaces()
	if err != nil {
		logger.Error("net.Interfaces failed, err: ", err.Error())
		info += t.I18nBot("tgbot.messages.ip", "IP=="+t.I18nBot("tgbot.unknown"))
		info += "\r\n"
	} else {
		for i := 0; i < len(netInterfaces); i++ {
			if (netInterfaces[i].Flags & net.FlagUp) != 0 {
				addrs, _ := netInterfaces[i].Addrs()

				for _, address := range addrs {
					if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							ipv4 += ipnet.IP.String() + " "
						} else if ipnet.IP.To16() != nil && !ipnet.IP.IsLinkLocalUnicast() {
							ipv6 += ipnet.IP.String() + " "
						}
					}
				}
			}
		}

		info += t.I18nBot("tgbot.messages.ipv4", "IPv4=="+ipv4)
		info += t.I18nBot("tgbot.messages.ipv6", "IPv6=="+ipv6)
	}

	info += t.I18nBot("tgbot.messages.serverUpTime", "UpTime=="+strconv.FormatUint(t.lastStatus.Uptime/86400, 10), "Unit=="+t.I18nBot("tgbot.days"))
	info += t.I18nBot("tgbot.messages.serverLoad", "Load1=="+strconv.FormatFloat(t.lastStatus.Loads[0], 'f', 2, 64), "Load2=="+strconv.FormatFloat(t.lastStatus.Loads[1], 'f', 2, 64), "Load3=="+strconv.FormatFloat(t.lastStatus.Loads[2], 'f', 2, 64))
	//nolint:gosec
	info += t.I18nBot("tgbot.messages.serverMemory", "Current=="+common.FormatTraffic(int64(t.lastStatus.Mem.Current)), "Total=="+common.FormatTraffic(int64(t.lastStatus.Mem.Total)))
	info += t.I18nBot("tgbot.messages.onlinesCount", "Count=="+fmt.Sprint(len(onlines)))
	info += t.I18nBot("tgbot.messages.tcpCount", "Count=="+strconv.Itoa(t.lastStatus.TcpCount))
	info += t.I18nBot("tgbot.messages.udpCount", "Count=="+strconv.Itoa(t.lastStatus.UdpCount))
	//nolint:gosec
	info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent+t.lastStatus.NetTraffic.Recv)), "Upload=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Sent)), "Download=="+common.FormatTraffic(int64(t.lastStatus.NetTraffic.Recv)))
	info += t.I18nBot("tgbot.messages.xrayStatus", "State=="+fmt.Sprint(t.lastStatus.Xray.State))
	return info
}

func (t *Tgbot) UserLoginNotify(username string, password string, ip string, time string, status LoginStatus) {
	if !t.IsRunning() {
		return
	}

	if username == "" || ip == "" || time == "" {
		logger.Warning("UserLoginNotify failed, invalid info!")
		return
	}

	loginNotifyEnabled, err := t.settingService.GetTgBotLoginNotify()
	if err != nil || !loginNotifyEnabled {
		return
	}

	msg := ""
	switch status {
	case LoginSuccess:
		msg += t.I18nBot("tgbot.messages.loginSuccess")
		msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
	case LoginFail:
		msg += t.I18nBot("tgbot.messages.loginFailed")
		msg += t.I18nBot("tgbot.messages.hostname", "Hostname=="+hostname)
		msg += t.I18nBot("tgbot.messages.password", "Password=="+password)
	}
	msg += t.I18nBot("tgbot.messages.username", "Username=="+username)
	msg += t.I18nBot("tgbot.messages.ip", "IP=="+ip)
	msg += t.I18nBot("tgbot.messages.time", "Time=="+time)
	t.SendMsgToTgbotAdmins(msg)
}

func (t *Tgbot) getInboundUsages() string {
	info := ""
	// get traffic
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		info += t.I18nBot("tgbot.answers.getInboundsFailed")
	} else {
		// NOTE:If there no any sessions here,need to notify here
		// TODO:Sub-node push, automatic conversion format
		for _, inbound := range inbounds {
			info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
			info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
			info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

			if inbound.ExpiryTime == 0 {
				info += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
			} else {
				info += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
			info += "\r\n"
		}
	}
	return info
}

func (t *Tgbot) getInbounds() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	if len(inbounds) == 0 {
		logger.Warning("No inbounds found")
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		callbackData := t.encodeQuery(fmt.Sprintf("%s %d", "get_clients", inbound.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(fmt.Sprintf("%v - %v", inbound.Remark, status)).WithCallbackData(callbackData))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

func (t *Tgbot) getInboundsAddClient() (*telego.InlineKeyboardMarkup, error) {
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("GetAllInbounds run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	if len(inbounds) == 0 {
		logger.Warning("No inbounds found")
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}

	excludedProtocols := map[model.Protocol]bool{
		model.Tunnel:    true,
		model.Socks:     true,
		model.WireGuard: true,
		model.HTTP:      true,
	}

	var buttons []telego.InlineKeyboardButton
	for _, inbound := range inbounds {
		if excludedProtocols[inbound.Protocol] {
			continue
		}

		status := "❌"
		if inbound.Enable {
			status = "✅"
		}
		callbackData := t.encodeQuery(fmt.Sprintf("%s %d", "add_client_to", inbound.Id))
		buttons = append(buttons, tu.InlineKeyboardButton(fmt.Sprintf("%v - %v", inbound.Remark, status)).WithCallbackData(callbackData))
	}

	cols := 1
	if len(buttons) >= 6 {
		cols = 2
	}

	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
	return keyboard, nil
}

func (t *Tgbot) getInboundClients(id int) (*telego.InlineKeyboardMarkup, error) {
	inbound, err := t.inboundService.GetInbound(id)
	if err != nil {
		logger.Warning("getIboundClients run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	}
	clients, err := t.inboundService.GetClients(inbound)
	var buttons []telego.InlineKeyboardButton

	if err != nil {
		logger.Warning("GetInboundClients run failed:", err)
		return nil, errors.New(t.I18nBot("tgbot.answers.getInboundsFailed"))
	} else {
		if len(clients) > 0 {
			for _, client := range clients {
				buttons = append(buttons, tu.InlineKeyboardButton(client.Email).WithCallbackData(t.encodeQuery("client_get_usage "+client.Email)))
			}
		} else {
			return nil, errors.New(t.I18nBot("tgbot.answers.getClientsFailed"))
		}
	}
	cols := 0
	if len(buttons) < 6 {
		cols = 3
	} else {
		cols = 2
	}
	keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))

	return keyboard, nil
}

func (t *Tgbot) clientInfoMsg(
	traffic *xray.ClientTraffic,
	printEnabled bool,
	printOnline bool,
	printActive bool,
	printDate bool,
	printTraffic bool,
	printRefreshed bool,
) string {
	now := time.Now().Unix()
	expiryTime := ""
	flag := false
	diff := traffic.ExpiryTime/1000 - now
	if traffic.ExpiryTime == 0 {
		expiryTime = t.I18nBot("tgbot.unlimited")
	} else if diff > 172800 || !traffic.Enable {
		expiryTime = time.Unix((traffic.ExpiryTime / 1000), 0).Format("2006-01-02 15:04:05")
		if diff > 0 {
			days := diff / 86400
			hours := (diff % 86400) / 3600
			minutes := (diff % 3600) / 60
			remainingTime := ""
			if days > 0 {
				remainingTime += fmt.Sprintf("%d %s ", days, t.I18nBot("tgbot.days"))
			}
			if hours > 0 {
				remainingTime += fmt.Sprintf("%d %s ", hours, t.I18nBot("tgbot.hours"))
			}
			if minutes > 0 {
				remainingTime += fmt.Sprintf("%d %s", minutes, t.I18nBot("tgbot.minutes"))
			}
			expiryTime += fmt.Sprintf(" (%s)", remainingTime)
		}
	} else if traffic.ExpiryTime < 0 {
		expiryTime = fmt.Sprintf("%d %s", traffic.ExpiryTime/-86400000, t.I18nBot("tgbot.days"))
		flag = true
	} else {
		expiryTime = fmt.Sprintf("%d %s", diff/3600, t.I18nBot("tgbot.hours"))
		flag = true
	}

	total := ""
	if traffic.Total == 0 {
		total = t.I18nBot("tgbot.unlimited")
	} else {
		total = common.FormatTraffic((traffic.Total))
	}

	enabled := ""
	isEnabled, err := t.inboundService.checkIsEnabledByEmail(traffic.Email)
	if err != nil {
		logger.Warning(err)
		enabled = t.I18nBot("tgbot.wentWrong")
	} else if isEnabled {
		enabled = t.I18nBot("tgbot.messages.yes")
	} else {
		enabled = t.I18nBot("tgbot.messages.no")
	}

	active := ""
	if traffic.Enable {
		active = t.I18nBot("tgbot.messages.yes")
	} else {
		active = t.I18nBot("tgbot.messages.no")
	}

	status := t.I18nBot("tgbot.offline")
	if p.IsRunning() {
		for _, online := range p.GetOnlineClients() {
			if online == traffic.Email {
				status = t.I18nBot("tgbot.online")
				break
			}
		}
	}

	output := ""
	output += t.I18nBot("tgbot.messages.email", "Email=="+traffic.Email)
	if printEnabled {
		output += t.I18nBot("tgbot.messages.enabled", "Enable=="+enabled)
	}
	if printOnline {
		output += t.I18nBot("tgbot.messages.online", "Status=="+status)
	}
	if printActive {
		output += t.I18nBot("tgbot.messages.active", "Enable=="+active)
	}
	if printDate {
		if flag {
			output += t.I18nBot("tgbot.messages.expireIn", "Time=="+expiryTime)
		} else {
			output += t.I18nBot("tgbot.messages.expire", "Time=="+expiryTime)
		}
	}
	if printTraffic {
		output += t.I18nBot("tgbot.messages.upload", "Upload=="+common.FormatTraffic(traffic.Up))
		output += t.I18nBot("tgbot.messages.download", "Download=="+common.FormatTraffic(traffic.Down))
		output += t.I18nBot("tgbot.messages.total", "UpDown=="+common.FormatTraffic((traffic.Up+traffic.Down)), "Total=="+total)
	}
	return output
}

func (t *Tgbot) getClientUsage(chatId int64, tgUserID int64, email ...string) {
	traffics, err := t.inboundService.GetClientTrafficTgBot(tgUserID)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	if len(traffics) == 0 {
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.askToAddUserId", "TgUserID=="+strconv.FormatInt(tgUserID, 10)))
		return
	}

	output := ""

	if len(traffics) > 0 {
		if len(email) > 0 {
			for _, traffic := range traffics {
				if traffic.Email == email[0] {
					output := t.clientInfoMsg(traffic, true, true, true, true, true, true)
					t.SendMsgToTgbot(chatId, output)
					return
				}
			}
			msg := t.I18nBot("tgbot.noResult")
			t.SendMsgToTgbot(chatId, msg)
			return
		} else {
			for _, traffic := range traffics {
				output += t.clientInfoMsg(traffic, true, true, true, true, true, false)
				output += "\r\n"
			}
		}
	}

	t.SendMsgToTgbot(chatId, output)
	output = t.I18nBot("tgbot.commands.pleaseChoose")
	t.SendAnswer(chatId, output, false)
}

func (t *Tgbot) searchClientIps(chatId int64, email string, messageID ...int) {
	ips, err := t.inboundService.GetInboundClientIps(email)
	if err != nil || len(ips) == 0 {
		ips = t.I18nBot("tgbot.noIpRecord")
	}

	output := ""
	output += t.I18nBot("tgbot.messages.email", "Email=="+email)
	output += t.I18nBot("tgbot.messages.ips", "IPs=="+ips)

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("ips_refresh "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.clearIPs")).WithCallbackData(t.encodeQuery("clear_ips "+email)),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
	}
}

func (t *Tgbot) clientTelegramUserInfo(chatId int64, email string, messageID ...int) {
	traffic, client, err := t.inboundService.GetClientByEmail(email)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if client == nil {
		msg := t.I18nBot("tgbot.noResult")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	tgId := "None"
	if client.TgID != 0 {
		tgId = strconv.FormatInt(client.TgID, 10)
	}

	output := ""
	output += t.I18nBot("tgbot.messages.email", "Email=="+email)
	output += t.I18nBot("tgbot.messages.TGUser", "TelegramID=="+tgId)

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("tgid_refresh "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.removeTGUser")).WithCallbackData(t.encodeQuery("tgid_remove "+email)),
		),
	)

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
		requestUser := telego.KeyboardButtonRequestUsers{
			//nolint:gosec
			RequestID: int32(traffic.Id),
			UserIsBot: new(bool),
		}
		keyboard := tu.Keyboard(
			tu.KeyboardRow(
				tu.KeyboardButton(t.I18nBot("tgbot.buttons.selectTGUser")).WithRequestUsers(&requestUser),
			),
			tu.KeyboardRow(
				tu.KeyboardButton(t.I18nBot("tgbot.buttons.closeKeyboard")),
			),
		).WithIsPersistent().WithResizeKeyboard()
		t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.buttons.selectOneTGUser"), keyboard)
	}
}

func (t *Tgbot) searchClient(chatId int64, email string, messageID ...int) {
	traffic, err := t.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if traffic == nil {
		msg := t.I18nBot("tgbot.noResult")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	output := t.clientInfoMsg(traffic, true, true, true, true, true, true)

	inlineKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("client_refresh "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetTraffic")).WithCallbackData(t.encodeQuery("reset_traffic "+email)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.limitTraffic")).WithCallbackData(t.encodeQuery("limit_traffic "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetExpire")).WithCallbackData(t.encodeQuery("reset_exp "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.ipLog")).WithCallbackData(t.encodeQuery("ip_log "+email)),
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.ipLimit")).WithCallbackData(t.encodeQuery("ip_limit "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.setTGUser")).WithCallbackData(t.encodeQuery("tg_user "+email)),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.toggle")).WithCallbackData(t.encodeQuery("toggle_enable "+email)),
		),
	)
	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, inlineKeyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, inlineKeyboard)
	}
}

func (t *Tgbot) addClient(chatId int64, msg string, messageID ...int) {
	inbound, err := t.inboundService.GetInbound(receiver_inbound_ID)
	if err != nil {
		t.SendMsgToTgbot(chatId, err.Error())
		return
	}

	protocol := inbound.Protocol

	switch protocol {
	case model.VMESS, model.VLESS:
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_email")).WithCallbackData("add_client_ch_default_email"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_id")).WithCallbackData("add_client_ch_default_id"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.limitTraffic")).WithCallbackData("add_client_ch_default_traffic"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetExpire")).WithCallbackData("add_client_ch_default_exp"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_comment")).WithCallbackData("add_client_ch_default_comment"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.ipLimit")).WithCallbackData("add_client_ch_default_ip_limit"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitDisable")).WithCallbackData("add_client_submit_disable"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitEnable")).WithCallbackData("add_client_submit_enable"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("add_client_cancel"),
			),
		)
		if len(messageID) > 0 {
			t.editMessageTgBot(chatId, messageID[0], msg, inlineKeyboard)
		} else {
			t.SendMsgToTgbot(chatId, msg, inlineKeyboard)
		}
	case model.Trojan:
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_email")).WithCallbackData("add_client_ch_default_email"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_password")).WithCallbackData("add_client_ch_default_pass_tr"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.limitTraffic")).WithCallbackData("add_client_ch_default_traffic"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetExpire")).WithCallbackData("add_client_ch_default_exp"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_comment")).WithCallbackData("add_client_ch_default_comment"),
				tu.InlineKeyboardButton("ip limit").WithCallbackData("add_client_ch_default_ip_limit"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitDisable")).WithCallbackData("add_client_submit_disable"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitEnable")).WithCallbackData("add_client_submit_enable"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("add_client_cancel"),
			),
		)
		if len(messageID) > 0 {
			t.editMessageTgBot(chatId, messageID[0], msg, inlineKeyboard)
		} else {
			t.SendMsgToTgbot(chatId, msg, inlineKeyboard)
		}
	case model.Shadowsocks:
		inlineKeyboard := tu.InlineKeyboard(
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_email")).WithCallbackData("add_client_ch_default_email"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_password")).WithCallbackData("add_client_ch_default_pass_sh"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.limitTraffic")).WithCallbackData("add_client_ch_default_traffic"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.resetExpire")).WithCallbackData("add_client_ch_default_exp"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.change_comment")).WithCallbackData("add_client_ch_default_comment"),
				tu.InlineKeyboardButton("ip limit").WithCallbackData("add_client_ch_default_ip_limit"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitDisable")).WithCallbackData("add_client_submit_disable"),
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.submitEnable")).WithCallbackData("add_client_submit_enable"),
			),
			tu.InlineKeyboardRow(
				tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.cancel")).WithCallbackData("add_client_cancel"),
			),
		)

		if len(messageID) > 0 {
			t.editMessageTgBot(chatId, messageID[0], msg, inlineKeyboard)
		} else {
			t.SendMsgToTgbot(chatId, msg, inlineKeyboard)
		}
	}
}

func (t *Tgbot) searchInbound(chatId int64, remark string) {
	inbounds, err := t.inboundService.SearchInbounds(remark)
	if err != nil {
		logger.Warning(err)
		msg := t.I18nBot("tgbot.wentWrong")
		t.SendMsgToTgbot(chatId, msg)
		return
	}
	if len(inbounds) == 0 {
		msg := t.I18nBot("tgbot.noInbounds")
		t.SendMsgToTgbot(chatId, msg)
		return
	}

	for _, inbound := range inbounds {
		info := ""
		info += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
		info += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
		info += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))

		if inbound.ExpiryTime == 0 {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
		} else {
			info += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
		}
		t.SendMsgToTgbot(chatId, info)

		if len(inbound.ClientStats) > 0 {
			output := ""
			for _, traffic := range inbound.ClientStats {
				output += t.clientInfoMsg(&traffic, true, true, true, true, true, true)
			}
			t.SendMsgToTgbot(chatId, output)
		}
	}
}

func (t *Tgbot) getExhausted(chatId int64) {
	trDiff := int64(0)
	exDiff := int64(0)
	now := time.Now().Unix() * 1000
	var exhaustedInbounds []model.Inbound
	var exhaustedClients []xray.ClientTraffic
	var disabledInbounds []model.Inbound
	var disabledClients []xray.ClientTraffic

	TrafficThreshold, err := t.settingService.GetTrafficDiff()
	if err == nil && TrafficThreshold > 0 {
		//nolint:gosec
		trDiff = int64(TrafficThreshold) * 1073741824
	}
	ExpireThreshold, err := t.settingService.GetExpireDiff()
	if err == nil && ExpireThreshold > 0 {
		//nolint:gosec
		exDiff = int64(ExpireThreshold) * 86400000
	}
	inbounds, err := t.inboundService.GetAllInbounds()
	if err != nil {
		logger.Warning("Unable to load Inbounds", err)
	}

	for _, inbound := range inbounds {
		if inbound.Enable {
			if (inbound.ExpiryTime > 0 && (inbound.ExpiryTime-now < exDiff)) ||
				(inbound.Total > 0 && (inbound.Total-(inbound.Up+inbound.Down) < trDiff)) {
				exhaustedInbounds = append(exhaustedInbounds, *inbound)
			}
			if len(inbound.ClientStats) > 0 {
				for _, client := range inbound.ClientStats {
					if client.Enable {
						if (client.ExpiryTime > 0 && (client.ExpiryTime-now < exDiff)) ||
							(client.Total > 0 && (client.Total-(client.Up+client.Down) < trDiff)) {
							exhaustedClients = append(exhaustedClients, client)
						}
					} else {
						disabledClients = append(disabledClients, client)
					}
				}
			}
		} else {
			disabledInbounds = append(disabledInbounds, *inbound)
		}
	}

	// Inbounds
	output := ""
	output += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.inbounds"))
	output += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(len(disabledInbounds)))
	output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(len(exhaustedInbounds)))

	if len(exhaustedInbounds) > 0 {
		output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+t.I18nBot("tgbot.inbounds"))

		for _, inbound := range exhaustedInbounds {
			output += t.I18nBot("tgbot.messages.inbound", "Remark=="+inbound.Remark)
			output += t.I18nBot("tgbot.messages.port", "Port=="+strconv.Itoa(inbound.Port))
			output += t.I18nBot("tgbot.messages.traffic", "Total=="+common.FormatTraffic((inbound.Up+inbound.Down)), "Upload=="+common.FormatTraffic(inbound.Up), "Download=="+common.FormatTraffic(inbound.Down))
			if inbound.ExpiryTime == 0 {
				output += t.I18nBot("tgbot.messages.expire", "Time=="+t.I18nBot("tgbot.unlimited"))
			} else {
				output += t.I18nBot("tgbot.messages.expire", "Time=="+time.Unix((inbound.ExpiryTime/1000), 0).Format("2006-01-02 15:04:05"))
			}
			output += "\r\n"
		}
	}

	// Clients
	exhaustedCC := len(exhaustedClients)
	output += t.I18nBot("tgbot.messages.exhaustedCount", "Type=="+t.I18nBot("tgbot.clients"))
	output += t.I18nBot("tgbot.messages.disabled", "Disabled=="+strconv.Itoa(len(disabledClients)))
	output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+strconv.Itoa(exhaustedCC))

	if exhaustedCC > 0 {
		output += t.I18nBot("tgbot.messages.depleteSoon", "Deplete=="+t.I18nBot("tgbot.clients"))
		var buttons []telego.InlineKeyboardButton
		for _, traffic := range exhaustedClients {
			output += t.clientInfoMsg(&traffic, true, false, false, true, true, false)
			output += "\r\n"
			buttons = append(buttons, tu.InlineKeyboardButton(traffic.Email).WithCallbackData(t.encodeQuery("client_get_usage "+traffic.Email)))
		}
		cols := 0
		if exhaustedCC < 11 {
			cols = 1
		} else {
			cols = 2
		}
		keyboard := tu.InlineKeyboardGrid(tu.InlineKeyboardCols(cols, buttons...))
		t.SendMsgToTgbot(chatId, output, keyboard)
	} else {
		t.SendMsgToTgbot(chatId, output)
	}
}

func (t *Tgbot) onlineClients(chatId int64, messageID ...int) {
	if !p.IsRunning() {
		return
	}

	onlines := p.GetOnlineClients()
	onlinesCount := len(onlines)
	output := t.I18nBot("tgbot.messages.onlinesCount", "Count=="+fmt.Sprint(onlinesCount))
	keyboard := tu.InlineKeyboard(tu.InlineKeyboardRow(
		tu.InlineKeyboardButton(t.I18nBot("tgbot.buttons.refresh")).WithCallbackData(t.encodeQuery("onlines_refresh"))))

	if onlinesCount > 0 {
		var buttons []telego.InlineKeyboardButton
		for _, online := range onlines {
			buttons = append(buttons, tu.InlineKeyboardButton(online).WithCallbackData(t.encodeQuery("client_get_usage "+online)))
		}
		cols := 0
		if onlinesCount < 21 {
			cols = 2
		} else if onlinesCount < 61 {
			cols = 3
		} else {
			cols = 4
		}
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, tu.InlineKeyboardCols(cols, buttons...)...)
	}

	if len(messageID) > 0 {
		t.editMessageTgBot(chatId, messageID[0], output, keyboard)
	} else {
		t.SendMsgToTgbot(chatId, output, keyboard)
	}
}

func (t *Tgbot) sendBackup(chatId int64) {
	output := t.I18nBot("tgbot.messages.backupTime", "Time=="+time.Now().Format("2006-01-02 15:04:05"))
	t.SendMsgToTgbot(chatId, output)

	// Update by manually trigger a checkpoint operation
	err := database.Checkpoint()
	if err != nil {
		logger.Error("Error in trigger a checkpoint operation: ", err)
	}

	file, err := os.Open(config.GetDBPath())
	if err == nil {
		document := tu.Document(
			tu.ID(chatId),
			tu.File(file),
		)
		_, err = bot.SendDocument(context.Background(), document)
		if err != nil {
			logger.Error("Error in uploading backup: ", err)
		}
	} else {
		logger.Error("Error in opening db file for backup: ", err)
	}

	file, err = os.Open(xray.GetConfigPath())
	if err == nil {
		document := tu.Document(
			tu.ID(chatId),
			tu.File(file),
		)
		_, err = bot.SendDocument(context.Background(), document)
		if err != nil {
			logger.Error("Error in uploading config.json: ", err)
		}
	} else {
		logger.Error("Error in opening config.json file for backup: ", err)
	}
}

func (t *Tgbot) sendBanLogs(chatId int64, dt bool) {
	if dt {
		output := t.I18nBot("tgbot.messages.datetime", "DateTime=="+time.Now().Format("2006-01-02 15:04:05"))
		t.SendMsgToTgbot(chatId, output)
	}

	file, err := os.Open(xray.GetIPLimitBannedPrevLogPath())
	if err == nil {
		// Check if the file is non-empty before attempting to upload
		fileInfo, _ := file.Stat()
		if fileInfo.Size() > 0 {
			document := tu.Document(
				tu.ID(chatId),
				tu.File(file),
			)
			_, err = bot.SendDocument(context.Background(), document)
			if err != nil {
				logger.Error("Error in uploading IPLimitBannedPrevLog: ", err)
			}
		} else {
			logger.Warning("IPLimitBannedPrevLog file is empty, not uploading.")
		}
		_ = file.Close()
	} else {
		logger.Error("Error in opening IPLimitBannedPrevLog file for backup: ", err)
	}

	file, err = os.Open(xray.GetIPLimitBannedLogPath())
	if err == nil {
		// Check if the file is non-empty before attempting to upload
		fileInfo, _ := file.Stat()
		if fileInfo.Size() > 0 {
			document := tu.Document(
				tu.ID(chatId),
				tu.File(file),
			)
			_, err = bot.SendDocument(context.Background(), document)
			if err != nil {
				logger.Error("Error in uploading IPLimitBannedLog: ", err)
			}
		} else {
			logger.Warning("IPLimitBannedLog file is empty, not uploading.")
		}
		_ = file.Close()
	} else {
		logger.Error("Error in opening IPLimitBannedLog file for backup: ", err)
	}
}

func (t *Tgbot) sendCallbackAnswerTgBot(id string, message string) {
	params := telego.AnswerCallbackQueryParams{
		CallbackQueryID: id,
		Text:            message,
	}
	if err := bot.AnswerCallbackQuery(context.Background(), &params); err != nil {
		logger.Warning(err)
	}
}

func (t *Tgbot) editMessageCallbackTgBot(chatId int64, messageID int, inlineKeyboard *telego.InlineKeyboardMarkup) {
	params := telego.EditMessageReplyMarkupParams{
		ChatID:      tu.ID(chatId),
		MessageID:   messageID,
		ReplyMarkup: inlineKeyboard,
	}
	if _, err := bot.EditMessageReplyMarkup(context.Background(), &params); err != nil {
		logger.Warning(err)
	}
}

func (t *Tgbot) editMessageTgBot(chatId int64, messageID int, text string, inlineKeyboard ...*telego.InlineKeyboardMarkup) {
	params := telego.EditMessageTextParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
		Text:      text,
		ParseMode: "HTML",
	}
	if len(inlineKeyboard) > 0 {
		params.ReplyMarkup = inlineKeyboard[0]
	}
	if _, err := bot.EditMessageText(context.Background(), &params); err != nil {
		logger.Warning(err)
	}
}

func (t *Tgbot) SendMsgToTgbotDeleteAfter(chatId int64, msg string, delayInSeconds int, replyMarkup ...telego.ReplyMarkup) {
	// Determine if replyMarkup was passed; otherwise, set it to nil
	var replyMarkupParam telego.ReplyMarkup
	if len(replyMarkup) > 0 {
		replyMarkupParam = replyMarkup[0] // Use the first element
	}

	// Send the message
	sentMsg, err := bot.SendMessage(context.Background(), &telego.SendMessageParams{
		ChatID:      tu.ID(chatId),
		Text:        msg,
		ReplyMarkup: replyMarkupParam, // Use the correct replyMarkup value
	})
	if err != nil {
		logger.Warning("Failed to send message:", err)
		return
	}

	// Delete the sent message after the specified number of seconds
	go func() {
		time.Sleep(time.Duration(delayInSeconds) * time.Second) // Wait for the specified delay
		t.deleteMessageTgBot(chatId, sentMsg.MessageID)         // Delete the message
		delete(userStates, chatId)
	}()
}

func (t *Tgbot) deleteMessageTgBot(chatId int64, messageID int) {
	params := telego.DeleteMessageParams{
		ChatID:    tu.ID(chatId),
		MessageID: messageID,
	}
	if err := bot.DeleteMessage(context.Background(), &params); err != nil {
		logger.Warning("Failed to delete message:", err)
	} else {
		logger.Info("Message deleted successfully")
	}
}

// 〔中文注释〕: 新增方法，实现 TelegramService 接口。
// 当设备限制任务需要发送消息时，会调用此方法。
// 该方法内部调用了已有的 SendMsgToTgbotAdmins 函数，将消息发送给所有管理员。
func (t *Tgbot) SendMessage(msg string) error {
	if !t.IsRunning() {
		// 〔中文注释〕: 如果 Bot 未运行，返回错误，防止程序出错。
		return errors.New("telegram bot is not running")
	}
	// 〔中文注释〕: 调用现有方法将消息发送给所有已配置的管理员。
	t.SendMsgToTgbotAdmins(msg)
	return nil
}

// 【新增函数】: 发送【一键配置】的选项按钮给用户
// 【重构后的函数】: 显示主分类菜单
func (t *Tgbot) sendOneClickOptions(chatId int64) {
	categoryKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔗 Direct Connection (直连)").WithCallbackData(t.encodeQuery("oneclick_category_direct")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔄 Relay (中转)").WithCallbackData(t.encodeQuery("oneclick_category_relay")),
		),
	)
	t.SendMsgToTgbot(chatId, "请选择【一键配置】类型：", categoryKeyboard)
}

// 【新增函数】: 显示中转类别的具体配置选项
func (t *Tgbot) sendRelayOptions(chatId int64) {
	relayKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🛡️ Vless Encryption + XHTTP + TLS").WithCallbackData(t.encodeQuery("oneclick_tls")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🌀 Switch + Vision Seed (开发中)").WithCallbackData(t.encodeQuery("oneclick_switch_vision")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("oneclick_options")),
		),
	)
	t.SendMsgToTgbot(chatId, "【中转】类别 - 适合需要中转的场景：\n\n🛡️ Vless Encryption + XHTTP + TLS: 加密传输，可配合CDN\n🌀 Switch + Vision Seed: 特殊配置（开发中）", relayKeyboard)
}

// 【新增函数】: 显示直连类别的具体配置选项
func (t *Tgbot) sendDirectOptions(chatId int64) {
	directKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🚀 Vless + TCP + Reality").WithCallbackData(t.encodeQuery("oneclick_reality")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⚡ Vless + XHTTP + Reality").WithCallbackData(t.encodeQuery("oneclick_xhttp_reality")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("oneclick_options")),
		),
	)
	t.SendMsgToTgbot(chatId, "【直连】类别 - 适合优化线路直连：\n\n🚀 Vless + TCP + Reality: 高性能直连，优秀兼容性\n⚡ Vless + XHTTP + Reality: 新型传输，更佳隐蔽性", directKeyboard)
}

// 远程创建【一键配置】入站，增加一个 type 参数
func (t *Tgbot) remoteCreateOneClickInbound(configType string, chatId int64) {
	var err error
	var newInbound *model.Inbound
	var ufwWarning string // 新增变量来捕获警告信息

	switch configType {
	case "reality":
		newInbound, ufwWarning, err = t.buildRealityInbound("")
	case "xhttp_reality":
		newInbound, ufwWarning, err = t.buildXhttpRealityInbound("")
	case "tls":
		newInbound, ufwWarning, err = t.buildTlsInbound()
	case "switch_vision": // 【新增】: 处理开发中的选项
		t.SendMsgToTgbot(chatId, "此协议组合的功能还在开发中 ............暂不可用...")
		return // 【中文注释】: 直接返回，不执行任何创建操作
	default:
		err = errors.New("未知的配置类型")
	}

	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: %v", err))
		return
	}

	// 〔中文注释〕: 创建一个 InboundService 实例，并将当前的 Tgbot 实例 (t) 作为 tgService 注入进去。
	inboundService := InboundService{}
	inboundService.SetTelegramService(t) // 将当前的 bot 实例注入

	createdInbound, _, err := inboundService.AddInbound(newInbound)
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 远程创建失败: 保存入站时出错: %v", err))
		return
	}

	logger.Infof("TG 机器人远程创建入站 %s 成功！", createdInbound.Remark)

	// 【新增功能】：如果端口放行失败，发送警告
	if ufwWarning != "" {
		t.SendMsgToTgbot(chatId, ufwWarning)
	} // END NEW LOGIC

	// 【调用 TG Bot 专属的通知方法】
	// inFromPanel 设置为 false，表示这是来自 TG 机器人的操作
	// 之前 SendOneClickConfig 的 inbound 参数是 *model.Inbound，所以我们传入 createdInbound
	// 将当前的 chatId 传入，确保配置消息发送给发起指令的用户
	err = t.SendOneClickConfig(createdInbound, false, chatId)
	if err != nil {
		// 如果发送通知失败，给用户一个提示，但不要中断流程
		t.SendMsgToTgbot(chatId, fmt.Sprintf("⚠️ 入站创建成功，但通知消息发送失败: %v", err))
		logger.Errorf("TG Bot: 远程创建入站成功，但发送通知失败: %v", err)
	} else {
		// 成功发送二维码/配置消息后，再给用户一个确认提示
		t.SendMsgToTgbot(chatId, "✅ **入站已创建，【二维码/配置链接】已发送至管理员私信。**")
	}
	// 【新增功能】：发送用法说明消息
	// 使用 ** 粗体标记，并使用多行字符串确保换行显示。
	usageMessage := `**用法说明：**
	
1、该功能已自动生成现今比较主流的入站协议，简单/直接，不用慢慢配置。
2、【一键配置】生成功能中的最前面两种协议组合，适合【优化线路】去直连使用。
3、随机分配一个可用端口，TG端会【自动放行】该端口，生成后请直接复制【**链接地址**】。
4、TG端 的【一键配置】生成功能，与后台 Web端 类似，跟【入站】的数据是打通的。
5、你可以在"一键创建"后于列表中，手动查看/复制或编辑详细信息，以便添加其他参数。`

	t.SendMsgToTgbot(chatId, usageMessage)
}

// 【新增函数】: 构建 Reality 配置对象 (1:1 复刻自 inbounds.html)
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
	remark := t.randomString(8, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

	port := 10000 + common.RandomInt(55535-10000+1)

	ufwWarning := "" // NEW

	// 【新增功能】：调用 firewalld 放行端口
	if err := t.openPortWithFirewalld(port); err != nil {
		// 【核心修改】：如果端口放行失败，不中断入站创建流程，但生成警告信息
		logger.Warningf("自动放行端口 %d 失败: %v", port, err)
		ufwWarning = fmt.Sprintf("⚠️ **警告：端口放行失败**\n\n自动执行 `firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --reload` 命令失败，入站创建流程已继续，但请务必**手动**在您的 VPS 上放行端口 `%d`，否则服务将无法访问。失败详情：%v", port, port, err)
	} // END NEW LOGIC

	// 按照要求格式：inbound-端口号
	tag := fmt.Sprintf("inbound-%d", port)

	// 使用统一的 SNI 域名列表
	realityDests := t.GetRealityDestinations()
	var randomDest string
	if len(targetDest) > 0 && targetDest[0] != "" {
		// 如果提供了指定的 SNI，使用它
		randomDest = targetDest[0]
	} else {
		// 使用 ServerService 中的 SNI 选择器
		if t.serverService != nil {
			randomDest = t.serverService.GetNewSNI()
		} else {
			// 回退到随机选择（防止空指针）
			randomDest = realityDests[common.RandomInt(len(realityDests))]
		}
	}

	// 使用新的 GenerateRealityServerNames 函数生成 SNI 列表
	serverNamesList := GenerateRealityServerNames(randomDest)
	shortIds := t.generateShortIds()

	// Settings (clients + decryption + fallbacks)
	settings, _ := json.Marshal(map[string]any{
		"clients": []map[string]any{{
			"id":     uuid,               // 客户端 UUID
			"flow":   "xtls-rprx-vision", // JS 中指定的 flow
			"email":  remark,
			"level":  0,
			"enable": true,
		}},
		"decryption": "none",
		"fallbacks":  []any{}, // 保留空数组（与前端一致）
	})

	// StreamSettings => reality
	streamSettings, _ := json.Marshal(map[string]any{
		"network":  "tcp",
		"security": "reality",
		"realitySettings": map[string]any{
			"show":        false,      // 前端 show: false
			"target":      randomDest, // e.g. "apple.com:443"
			"xver":        0,
			"serverNames": serverNamesList, // 使用新的 GenerateRealityServerNames 函数
			// 注意：realitySettings.settings 是一个对象（map），不是数组
			"settings": map[string]any{
				"publicKey":     publicKey,
				"spiderX":       "/", // 前端写了 spiderX: "/"
				"mldsa65Verify": "",
			},
			"privateKey":   privateKey,
			"maxClientVer": "",
			"minClientVer": "",
			"maxTimediff":  0,
			"mldsa65Seed":  "",       // 一般留空（JS 注释）
			"shortIds":     shortIds, // 传入的短 id 列表
		},
		// TCP 子对象
		"tcpSettings": map[string]any{
			"acceptProxyProtocol": false,
			"header": map[string]any{
				"type": "none",
			},
		},
	})

	// sniffing 完整保留（enabled + destOverride + metadataOnly + routeOnly）
	sniffing, _ := json.Marshal(map[string]any{
		"enabled":      true,
		"destOverride": []string{"http", "tls", "quic", "fakedns"},
		"metadataOnly": false,
		"routeOnly":    false,
	})

	// 返回 model.Inbound —— 请根据你项目中的 model.Inbound 增减字段（此处包含常见字段）
	return &model.Inbound{
		UserId:   1, // 示例：创建者/系统用户 ID，如需动态请替换
		Remark:   remark,
		Enable:   true,
		Listen:   "", // 对应前端 listen: ''
		Port:     port,
		Tag:      tag,
		Protocol: "vless",
		// 如果你的 model.Inbound 有这些字段（前端 data 也包含），可以设置或保持默认
		ExpiryTime:     0, // 前端 expiryTime: 0
		DeviceLimit:    0, // 前端 deviceLimit: 0
		Settings:       string(settings),
		StreamSettings: string(streamSettings),
		Sniffing:       string(sniffing),
	}, ufwWarning, nil // MODIFIED RETURN
}

// 【新增函数】: 构建 TLS 配置对象 (1:1 复刻自 inbounds.html)
func (t *Tgbot) buildTlsInbound() (*model.Inbound, string, error) { // 更改签名
	encMsg, err := t.serverService.GetNewVlessEnc()
	if err != nil {
		return nil, "", fmt.Errorf("获取 VLESS 加密配置失败: %v", err)
	}
	uuidMsg, err := t.serverService.GetNewUUID()
	if err != nil {
		return nil, "", fmt.Errorf("获取 UUID 失败: %v", err)
	}

	var decryption, encryption string

	// 确认顶层类型是 map[string]interface{}
	encMsgMap, ok := encMsg.(map[string]interface{})
	if !ok {
		return nil, "", fmt.Errorf("VLESS 加密配置格式不正确: 期望得到 map[string]interface {}，但收到了 %T", encMsg)
	}

	// 从顶层 map 中直接获取 "auths" 键的值
	authsVal, found := encMsgMap["auths"]

	if !found {
		return nil, "", errors.New("VLESS 加密配置 auths 格式不正确: 未能在响应中找到 'auths' 数组")
	}

	// 将 auths 的值断言为正确的类型 []map[string]string
	// 这是因为 server.go 中的 GetNewVlessEnc 明确返回这个类型。
	auths, ok := authsVal.([]map[string]string)
	if !ok {
		// 如果断言失败，则意味着 auths 数组的内部元素类型不匹配
		return nil, "", fmt.Errorf("VLESS 加密配置 auths 格式不正确: 'auths' 数组的内部元素类型应为 map[string]string，但收到了 %T", authsVal)
	}

	// 遍历 auths 数组寻找 ML-KEM-768
	for _, auth := range auths {
		// 现在 auth 已经是 map[string]string 类型，可以直接安全访问
		if label, ok2 := auth["label"]; ok2 && label == "ML-KEM-768, Post-Quantum" {
			decryption = auth["decryption"]
			encryption = auth["encryption"]
			break // 找到后跳出循环
		}
	}

	if decryption == "" || encryption == "" {
		return nil, "", errors.New("未能在 auths 数组中找到 ML-KEM-768 加密密钥，请检查 Xray 版本")
	}

	domain, err := t.getDomain()
	if err != nil {
		return nil, "", err
	}

	uuid := uuidMsg["uuid"]
	remark := t.randomString(8, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	allowedPorts := []int{2053, 2083, 2087, 2096, 8443}
	port := allowedPorts[common.RandomInt(len(allowedPorts))]

	ufwWarning := "" // NEW

	// 【新增功能】：调用 firewalld 放行端口
	if err := t.openPortWithFirewalld(port); err != nil {
		// 【核心修改】：如果端口放行失败，不中断入站创建流程，但生成警告信息
		logger.Warningf("自动放行端口 %d 失败: %v", port, err)
		ufwWarning = fmt.Sprintf("⚠️ **警告：端口放行失败**\n\n自动执行 `firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --reload` 命令失败，入站创建流程已继续，但请务必**手动**在您的 VPS 上放行端口 `%d`，否则服务将无法访问。失败详情：%v", port, port, err)
	} // END NEW LOGIC

	// 按照要求格式：inbound-端口号
	tag := fmt.Sprintf("inbound-%d", port)
	path := "/" + t.randomString(8, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	certPath := fmt.Sprintf("/root/cert/%s/fullchain.pem", domain)
	keyPath := fmt.Sprintf("/root/cert/%s/privkey.pem", domain)

	// Settings: clients + decryption + encryption + selectedAuth
	settings, _ := json.Marshal(map[string]any{
		"clients": []map[string]any{{
			"id": uuid,
			// 注意：XHTTP 传输不需要 flow 字段，完全省略以避免 Xray 警告
			"email":    remark,
			"level":    0,
			"password": "", // JS 中 password: ""
			"enable":   true,
		}},
		"decryption":   decryption,                 // 从 API 获取
		"encryption":   encryption,                 // 从 API 获取（新增）
		"selectedAuth": "ML-KEM-768, Post-Quantum", // 前端硬编码选择项
	})

	// streamSettings：network=xhttp, security=tls, tlsSettings + xhttpSettings
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
			"verifyPeerCertInNames":   []string{"dns.google", "cloudflare-dns.com"},
		},
		"xhttpSettings": map[string]any{
			"headers":              map[string]any{}, // 可按需填充（JS 为 {}）
			"host":                 "",               // 前端留空
			"mode":                 "packet-up",
			"noSSEHeader":          false,
			"path":                 path, // 随机 8 位路径
			"scMaxBufferedPosts":   30,
			"scMaxEachPostBytes":   "1000000",
			"scStreamUpServerSecs": "20-80",
			"xPaddingBytes":        "100-1000",
		},
	})

	// sniffing: 与前端一致（enabled:false）
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
	}, ufwWarning, nil // MODIFIED RETURN
}

// 【新增函数】: 构建 VLESS + XHTTP + Reality 配置对象
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
	remark := t.randomString(8, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

	port := 10000 + common.RandomInt(55535-10000+1)
	path := "/" + t.randomString(8, "abcdefghijklmnopqrstuvwxyz")

	var ufwWarning string
	if err := t.openPortWithFirewalld(port); err != nil {
		logger.Warningf("自动放行端口 %d 失败: %v", port, err)
		ufwWarning = fmt.Sprintf("⚠️ **警告：端口放行失败**\n\n自动执行 `firewall-cmd --permanent --add-port=%d/tcp && firewall-cmd --reload` 命令失败，但入站创建已继续。请务必**手动**在您的 VPS 上放行端口 `%d`，否则服务将无法访问。", port, port)
	}

	tag := fmt.Sprintf("inbound-%d", port)

	// 使用统一的 SNI 域名列表
	realityDests := t.GetRealityDestinations()
	var randomDest string
	if len(targetDest) > 0 && targetDest[0] != "" {
		// 如果提供了指定的 SNI，使用它
		randomDest = targetDest[0]
	} else {
		// 使用 ServerService 中的 SNI 选择器
		if t.serverService != nil {
			randomDest = t.serverService.GetNewSNI()
		} else {
			// 回退到随机选择（防止空指针）
			randomDest = realityDests[common.RandomInt(len(realityDests))]
		}
	}

	// 使用新的 GenerateRealityServerNames 函数生成 SNI 列表
	serverNamesList := GenerateRealityServerNames(randomDest)
	shortIds := t.generateShortIds()

	settings, _ := json.Marshal(map[string]any{
		"clients": []map[string]any{{
			"id": uuid,
			// 注意：XHTTP 传输不支持 xtls-rprx-vision，需留空。Xray 可能会输出 flow deprecated 警告，属已知现象。
			"flow":     "",
			"email":    remark,
			"level":    0,
			"password": "", // JS 中 password: ""
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
			"serverNames":  serverNamesList, // 使用新的 GenerateRealityServerNames 函数
			"privateKey":   privateKey,
			"maxClientVer": "",
			"minClientVer": "",
			"maxTimediff":  0,
			"mldsa65Seed":  "",
			"shortIds":     shortIds,
			"settings": map[string]any{
				"publicKey":     publicKey,
				"spiderX":       "/", // 前端写了 spiderX: "/"
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

// 【修改后函数】: 发送【一键配置】的专属消息，增加链接类型判断
func (t *Tgbot) SendOneClickConfig(inbound *model.Inbound, inFromPanel bool, targetChatId int64) error {
	// 【修复】: 当 targetChatId 为 0 时（来自面板的一键配置），向所有管理员发送消息
	if targetChatId == 0 {
		if len(adminIds) == 0 {
			return fmt.Errorf("无法发送 TG 通知: 未配置管理员 Chat ID")
		}
		// 向所有管理员发送消息
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
	var dbLinkType string // 【新增】: 用于存入数据库的类型标识

	var streamSettings map[string]any
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings)

	// --- 1. 确定链接和协议类型 ---
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
			linkType = "Vless Encryption + XHTTP + TLS" // 协议类型
			dbLinkType = "vless_tls_encryption"
		default:
			return fmt.Errorf("未知的入站 security 类型: %s", security)
		}
	} else {
		return errors.New("无法解析 streamSettings 中的 security 字段")
	}

	if err != nil {
		return err
	}

	// 尝试生成二维码，如果失败，则 qrCodeBytes 为 nil 或空
	qrCodeBytes, err := qrcode.Encode(link, qrcode.Medium, 256)
	if err != nil {
		logger.Warningf("生成二维码失败，将尝试发送纯文本链接: %v", err)
		qrCodeBytes = nil // 确保 qrCodeBytes 为 nil，用于后续判断
	}

	// --- 2. 获取生成时间 ---
	now := time.Now().Format("2006-01-02 15:04:05")

	// --- 3. 构造包含所有信息并严格遵循格式的描述消息 ---
	baseCaption := fmt.Sprintf(
		"入站备注（用户 Email）：\n\n------->>>  `%s`\n\n对应端口号：\n\n---------->>>>>  `%d`\n\n协议类型：\n\n`%s`\n\n设备限制：0（无限制）\n\n生成时间：\n\n`%s`",
		inbound.Remark,
		inbound.Port,
		linkType,
		now,
	)

	var caption string
	if inFromPanel {
		caption = fmt.Sprintf("✅ **面板【一键配置】入站已创建成功！**\n\n%s\n\n👇 **可点击下方链接直接【复制/导入】** 👇", baseCaption)
	} else {
		caption = fmt.Sprintf("✅ **TG端 远程【一键配置】创建成功！**\n\n%s\n\n👇 **可点击下方链接直接【复制/导入】** 👇", baseCaption)
	}
	// 发送主消息（包含描述和二维码）
	if len(qrCodeBytes) > 0 {
		// 尝试发送图片消息
		photoParams := tu.Photo(
			tu.ID(targetChatId),
			tu.FileFromBytes(qrCodeBytes, "qrcode.png"),
		).WithCaption(caption).WithParseMode(telego.ModeMarkdown)

		if _, err := bot.SendPhoto(context.Background(), photoParams); err != nil {
			logger.Warningf("发送带二维码的 TG 消息给 %d 失败: %v", targetChatId, err)
			// 如果图片发送失败，回退到发送纯文本描述
			t.SendMsgToTgbot(targetChatId, caption)
		}
	} else {
		// 如果二维码生成失败，直接发送纯文本描述
		t.SendMsgToTgbot(targetChatId, caption)
	}

	// 链接单独发送，不带任何 Markdown 格式。
	// 这将确保 Telegram 客户端可以将其正确识别为可点击的 vless:// 链接。
	t.SendMsgToTgbot(targetChatId, link)

	// 使用正确的类型保存历史记录
	t.saveLinkToHistory(dbLinkType, link)

	return nil
}

// 【新增辅助函数】: 生成 Reality 链接
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

	// publicKey 在 realitySettings 下的 settings 子对象中
	settingsMap, ok := realitySettings["settings"].(map[string]interface{})
	if !ok {
		return "", errors.New("realitySettings中缺少settings子对象")
	}
	publicKey, ok := settingsMap["publicKey"].(string)
	if !ok {
		// 再次检查，以防结构有变，但主要依赖 settingsMap
		return "", errors.New("publicKey字段缺失或格式错误 (可能在settings子对象中)")
	}

	shortIdsInterface := realitySettings["shortIds"].([]interface{})
	// 确保 shortIdsInterface 不为空，否则可能 panic
	if len(shortIdsInterface) == 0 {
		return "", errors.New("无法生成 Reality 链接：Short IDs 列表为空")
	}
	sid := shortIdsInterface[common.RandomInt(len(shortIdsInterface))].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	// ---------------------- URL 编码 ----------------------
	// 必须对查询参数的值（pbk, sni, sid）
	// Go 标准库中的 net/url.QueryEscape 会处理 Base64 字符串中的 + / 等字符。
	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	return fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F&flow=xtls-rprx-vision#%s-%s",
		uuid, domain, inbound.Port, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

// 【新增辅助函数】: 生成 Reality 链接（支持指定客户端）
func (t *Tgbot) generateRealityLinkWithClient(inbound *model.Inbound, client model.Client) (string, error) {
	uuid := client.ID

	var streamSettings map[string]any
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &streamSettings)
	realitySettings := streamSettings["realitySettings"].(map[string]interface{})
	serverNames := realitySettings["serverNames"].([]interface{})
	sni := serverNames[0].(string)

	// publicKey 在 realitySettings 下的 settings 子对象中
	settingsMap, ok := realitySettings["settings"].(map[string]interface{})
	if !ok {
		return "", errors.New("realitySettings中缺少settings子对象")
	}
	publicKey, ok := settingsMap["publicKey"].(string)
	if !ok {
		// 再次检查，以防结构有变，但主要依赖 settingsMap
		return "", errors.New("publicKey字段缺失或格式错误 (可能在settings子对象中)")
	}

	shortIdsInterface := realitySettings["shortIds"].([]interface{})
	// 确保 shortIdsInterface 不为空，否则可能 panic
	if len(shortIdsInterface) == 0 {
		return "", errors.New("无法生成 Reality 链接：Short IDs 列表为空")
	}
	sid := shortIdsInterface[common.RandomInt(len(shortIdsInterface))].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	// ---------------------- URL 编码 ----------------------
	// 必须对查询参数的值（pbk, sni, sid）
	// Go 标准库中的 net/url.QueryEscape 会处理 Base64 字符串中的 + / 等字符。
	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	return fmt.Sprintf("vless://%s@%s:%d?type=tcp&encryption=none&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F&flow=xtls-rprx-vision#%s-%s",
		uuid, domain, inbound.Port, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

// 【新增辅助函数】: 生成 TLS 链接
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

	// 【修复】: type 应该是 xhttp，不是 tcp；XHTTP 模式不需要 flow 参数
	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=%s&path=%s&security=tls&fp=chrome&alpn=http%%2F1.1&sni=%s#%s-%s",
		uuid, domain, inbound.Port, encryption, url.QueryEscape(path), sni, inbound.Remark, inbound.Remark), nil
}

// 【新增辅助函数】: 生成 TLS 链接（支持指定客户端）
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

	// 【修复】: type 应该是 xhttp，不是 tcp；XHTTP 模式flow 参数根据官方要求可能需要，暂保持现状，仅处理 link
	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=%s&path=%s&security=tls&fp=chrome&alpn=http%%2F1.1&sni=%s#%s-%s",
		uuid, domain, inbound.Port, encryption, url.QueryEscape(path), sni, client.Email, inbound.Remark), nil
}

// 生成 VLESS + XHTTP + Reality 链接的函数
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
		return "", errors.New("无法生成 Reality 链接：Short IDs 列表为空")
	}
	sid := shortIdsInterface[common.RandomInt(len(shortIdsInterface))].(string)

	xhttpSettings, _ := streamSettings["xhttpSettings"].(map[string]interface{})
	path := xhttpSettings["path"].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	// 【中文注释】: 对所有URL查询参数进行编码
	escapedPath := url.QueryEscape(path)
	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	// 【中文注释】: 严格按照最新格式构建链接
	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=none&path=%s&host=&mode=stream-up&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F#%s-%s",
		uuid, domain, inbound.Port, escapedPath, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

// 【新增辅助函数】: 生成 VLESS + XHTTP + Reality 链接（支持指定客户端）
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
		return "", errors.New("无法生成 Reality 链接：Short IDs 列表为空")
	}
	sid := shortIdsInterface[common.RandomInt(len(shortIdsInterface))].(string)

	xhttpSettings, _ := streamSettings["xhttpSettings"].(map[string]interface{})
	path := xhttpSettings["path"].(string)

	domain, err := t.getDomain()
	if err != nil {
		return "", err
	}

	// 【中文注释】: 对所有URL查询参数进行编码
	escapedPath := url.QueryEscape(path)
	escapedPublicKey := url.QueryEscape(publicKey)
	escapedSni := url.QueryEscape(sni)
	escapedSid := url.QueryEscape(sid)
	escapedRemark := url.QueryEscape(inbound.Remark)

	// 【中文注释】: 严格按照最新格式构建链接
	return fmt.Sprintf("vless://%s@%s:%d?type=xhttp&encryption=none&path=%s&host=&mode=stream-up&security=reality&pbk=%s&fp=chrome&sni=%s&sid=%s&spx=%%2F#%s-%s",
		uuid, domain, inbound.Port, escapedPath, escapedPublicKey, escapedSni, escapedSid, escapedRemark, escapedRemark), nil
}

// 【新增方法】: 检查面板更新
func (t *Tgbot) checkPanelUpdate(chatId int64) {
	// 获取当前版本
	currentVersion := config.GetVersion()

	// 获取最新版本
	latestVersion, err := t.serverService.GetPanelLatestVersion()
	if err != nil {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 检查更新失败: %v", err))
		return
	}

	// 比较版本
	if currentVersion == latestVersion {
		t.SendMsgToTgbot(chatId, fmt.Sprintf("✅ 您的面板已经是最新版本！\n\n当前版本: `%s`", currentVersion))
		return
	}

	// 版本不同，显示更新提示
	confirmKeyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ 确认更新").WithCallbackData(t.encodeQuery("confirm_panel_update")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("❌ 取消").WithCallbackData(t.encodeQuery("cancel_panel_update")),
		),
	)

	message := fmt.Sprintf(
		"🔄 **发现新版本！**\n\n"+
			"当前版本: `%s`\n"+
			"最新版本: `%s`\n\n"+
			"⚠️ **注意：** 更新将：\n"+
			"• 自动从 GitHub 拉取最新代码\n"+
			"• 重启面板服务（期间无法访问）\n\n"+
			"是否确认更新？",
		currentVersion, latestVersion)

	t.SendMsgToTgbot(chatId, message, confirmKeyboard)
}

// 【新增辅助函数】: 获取域名（shell 方案）
func (t *Tgbot) getDomain() (string, error) {
	cmd := exec.Command("/usr/local/x-ui/x-ui", "setting", "-getCert", "true")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.New("执行命令获取证书路径失败，请确保已为面板配置 SSL 证书")
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
		return "", errors.New("无法从 x-ui 命令输出中找到证书路径")
	}

	certPath := strings.TrimSpace(strings.TrimPrefix(certLine, "cert:"))
	if certPath == "" {
		return "", errors.New("证书路径为空，请确保已为面板配置 SSL 证书")
	}

	domain := filepath.Base(filepath.Dir(certPath))
	return domain, nil
}

// 【新增辅助函数】: 1:1 复刻自 inbounds.html
func (t *Tgbot) generateShortIds() []string {
	chars := "0123456789abcdef"
	lengths := []int{2, 4, 6, 8, 10, 12, 14, 16}
	shortIds := make([]string, len(lengths))
	for i, length := range lengths {
		shortIds[i] = t.randomString(length, chars)
	}
	return shortIds
}

// 【新增辅助函数】: 随机字符串生成器

// showLogSettings 显示日志设置菜单
func (t *Tgbot) showLogSettings(chatId int64) {
	tgForwardEnabled, err := t.settingService.GetTgLogForwardEnabled()
	if err != nil {
		t.SendMsgToTgbot(chatId, "❌ 获取 TG 转发状态失败")
		return
	}
	localLogEnabled, err := t.settingService.GetLocalLogEnabled()
	if err != nil {
		localLogEnabled = false
	}
	logLevel, err := t.settingService.GetTgLogLevel()
	if err != nil {
		logLevel = "warn"
	}

	tgForwardStatus := "❌"
	if tgForwardEnabled {
		tgForwardStatus = "✅"
	}
	localLogStatus := "❌"
	if localLogEnabled {
		localLogStatus = "✅"
	}

	message := fmt.Sprintf("📝 **日志设置**\n\n"+
		"📤 TG 转发: %s\n"+
		"💾 本地日志: %s\n"+
		"🔧 日志级别: %s\n\n"+
		"选择要切换的设置:",
		tgForwardStatus, localLogStatus, logLevel)

	keyboard := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("📤 TG 转发: %s", tgForwardStatus)).WithCallbackData(t.encodeQuery("toggle_log_forward")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("💾 本地日志: %s", localLogStatus)).WithCallbackData(t.encodeQuery("toggle_local_log")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton(fmt.Sprintf("🔧 日志级别: %s", logLevel)).WithCallbackData(t.encodeQuery("cycle_log_level")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("🔴 仅错误").WithCallbackData(t.encodeQuery("set_log_level error")),
			tu.InlineKeyboardButton("⚠️ 警告及以上").WithCallbackData(t.encodeQuery("set_log_level warn")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("ℹ️ 全部信息").WithCallbackData(t.encodeQuery("set_log_level info")),
		),
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("⬅️ 返回主菜单").WithCallbackData(t.encodeQuery("back_to_main")),
		),
	)

	t.SendMsgToTgbot(chatId, message, keyboard)
}
