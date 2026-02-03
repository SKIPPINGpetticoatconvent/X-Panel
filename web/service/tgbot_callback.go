package service

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"x-ui/config"
	"x-ui/logger"
	"x-ui/util/common"

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
		return "", false, common.NewErrorf("获取内核版本失败: %v", err)
	}
	kernelVersion := strings.TrimSpace(string(kernelOutput))

	// 解析内核版本号
	kernelParts := strings.Split(kernelVersion, ".")
	if len(kernelParts) < 2 {
		return kernelVersion, false, common.NewErrorf("无法解析内核版本: %s", kernelVersion)
	}

	majorVersion, err := strconv.Atoi(kernelParts[0])
	if err != nil {
		return kernelVersion, false, common.NewErrorf("解析主版本号失败: %v", err)
	}

	minorVersion, err := strconv.Atoi(strings.Split(kernelParts[1], "-")[0])
	if err != nil {
		return kernelVersion, false, common.NewErrorf("解析次版本号失败: %v", err)
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
		return kernelVersion, false, common.NewErrorf("检查 BBR 模块失败: %v", err)
	}

	bbrAvailable := strings.TrimSpace(string(modprobeOutput)) == "supported"

	return kernelVersion, bbrAvailable, nil
}

func (t *Tgbot) answerCallback(callbackQuery *telego.CallbackQuery, isAdmin bool) {
	chatId := callbackQuery.Message.GetChat().ID

	// 优先处理对所有用户开放的命令（无需 Admin 权限）
	decodedQueryCommon, err := t.decodeQuery(callbackQuery.Data)
	if err == nil {
		dataArrayCommon := strings.Split(decodedQueryCommon, " ")
		if len(dataArrayCommon) > 0 && dataArrayCommon[0] == "copy_all_links" {
			t.sendCallbackAnswerTgBot(callbackQuery.ID, "📋 正在生成所有客户端链接...")
			err := t.copyAllLinks(chatId)
			if err != nil {
				t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 生成链接失败: %v", err))
			}
			return
		}
	}

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
					t.SendMsgToTgbot(chatId, "📋 <b>最近日志</b>\n\n❌ 未找到符合级别的日志记录")
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
					// 修正参数传递，添加 chatId
					clients, err := t.getInboundClients(chatId, inboundIdInt)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					t.SendMsgToTgbot(chatId, t.I18nBot("tgbot.answers.chooseClient", "Inbound=="+inbound.Remark), clients)
				case "copy_inbound_clients":
					// 处理批量复制回调
					inboundId := dataArray[1]
					inboundIdInt, err := strconv.Atoi(inboundId)
					if err != nil {
						t.sendCallbackAnswerTgBot(callbackQuery.ID, err.Error())
						return
					}
					t.sendCallbackAnswerTgBot(callbackQuery.ID, "📋 正在生成链接...")
					err = t.copyInboundClients(chatId, inboundIdInt)
					if err != nil {
						t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 生成链接失败: %v", err))
					}
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

			// 统一使用 decodedQuery 进行 switch 判断，确保哈希策略变更时的兼容性
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

	// 统一使用 decodedQuery 进行 switch 判断
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

	// ━━━━━━━━━━ 两级菜单回调处理 ━━━━━━━━━━
	case "menu_main":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🏠 主菜单")
		t.SendAnswer(chatId, "🏠 <b>主菜单</b>\n\n请选择功能分类：", true)
	case "menu_monitor":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "📊 系统监控")
		t.showMenuMonitor(chatId, callbackQuery.Message.GetMessageID())
	case "menu_users":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "👥 用户管理")
		t.showMenuUsers(chatId, callbackQuery.Message.GetMessageID())
	case "menu_maintenance":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🛠 系统维护")
		t.showMenuMaintenance(chatId, callbackQuery.Message.GetMessageID())
	case "menu_advanced":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "⚙️ 高级设置")
		t.showMenuAdvanced(chatId, callbackQuery.Message.GetMessageID())

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

	// 处理分层菜单的回调
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

	// 重启面板、VPS推荐
	case "restart_panel":
		// 用户从菜单点击重启，删除主菜单并发送确认消息
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
		// 用户确认重启
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "指令已发送，请稍候...")
		t.SendMsgToTgbot(chatId, "⏳ 【重启命令】已在 VPS 中远程执行，\n\n正在等待面板恢复（约30秒），并进行验证检查...")

		// 在后台协程中执行重启，避免阻塞机器人
		go func() {
			err := t.serverService.RestartPanel()
			// 使用配置的延时，让面板有足够的时间重启
			time.Sleep(config.TelegramPanelRestartWait)
			if err != nil {
				// 如果执行出错，发送失败消息
				t.SendMsgToTgbot(chatId, fmt.Sprintf("❌ 面板重启命令执行失败！\n\n错误信息已记录到日志，请检查命令或权限。\n\n<code>%v</code>", err))
			} else {
				// 执行成功，发送成功消息
				t.SendMsgToTgbot(chatId, "🚀 面板重启成功！服务已成功恢复！")
			}
		}()

	case "restart_panel_cancel":
		// 用户取消重启
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "操作已取消")
		// 发送一个临时消息提示用户，3秒后自动删除
		t.SendMsgToTgbotDeleteAfter(chatId, "已取消重启操作。", 3)

	case "vps_recommend":
		// VPS推荐功能已移除
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "VPS推荐功能已移除")

	// 处理 Xray 版本管理相关回调
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

	// 处理机器优化一键方案相关回调
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

	// 处理防火墙管理相关回调
	case "firewall_menu":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🔥 正在打开防火墙管理菜单...")
		t.sendFirewallMenu(chatId)

	// 处理程序更新相关回调
	case "check_panel_update":
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "🔄 正在检查最新版本...")
		t.checkPanelUpdate(chatId)

	case "confirm_panel_update":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "✅ 更新指令已发送")
		t.SendMsgToTgbot(chatId, "🔄 <b>X-Panel 更新任务已在后台启动</b>\n\n⏳ 请稍候，更新完成后将收到通知...")
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
		text := "🌍 <b>Geo 数据更新确认</b>\n\n" +
			"这将从官方源下载最新的 GeoIP 和 GeoSite 数据，并自动重启 Xray 服务。\n\n" +
			"⚠️ <b>注意：</b>\n" +
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

	// 处理 Geo 数据更新相关回调
	case "update_geodata_confirm":
		t.deleteMessageTgBot(chatId, callbackQuery.Message.GetMessageID())
		t.sendCallbackAnswerTgBot(callbackQuery.ID, "✅ 指令已发送")
		t.SendMsgToTgbot(chatId, "🌍 <b>Geo 数据更新任务已在后台启动</b>\n\n⏳ 请稍候，更新完成后将收到通知...")

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
			t.SendMsgToTgbot(chatId, "📋 <b>最近日志</b>\n\n❌ 未找到符合级别的日志记录")
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

func checkAdmin(tgId int64) bool {
	for _, adminId := range adminIds {
		if adminId == tgId {
			return true
		}
	}
	return false
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
		time.Sleep(config.TelegramMessageDelay)
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

// 新增方法，实现 TelegramService 接口。
// 当设备限制任务需要发送消息时，会调用此方法。
// 该方法内部调用了已有的 SendMsgToTgbotAdmins 函数，将消息发送给所有管理员。
func (t *Tgbot) SendMessage(msg string) error {
	if !t.IsRunning() {
		// 如果 Bot 未运行，返回错误，防止程序出错。
		return common.ErrTelegramNotRunning
	}
	// 调用现有方法将消息发送给所有已配置的管理员。
	t.SendMsgToTgbotAdmins(msg)
	return nil
}
