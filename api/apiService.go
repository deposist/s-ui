package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/deposist/s-ui-rus-inst/database"
	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/realtime"
	"github.com/deposist/s-ui-rus-inst/service"
	"github.com/deposist/s-ui-rus-inst/util"
	"github.com/deposist/s-ui-rus-inst/util/redact"

	"github.com/gin-gonic/gin"
)

type ApiService struct {
	service.SettingService
	service.UserService
	service.ConfigService
	service.ClientService
	service.TlsService
	service.InboundService
	service.OutboundService
	service.EndpointService
	service.ServicesService
	service.PanelService
	service.StatsService
	service.ServerService
	service.AuditService
	service.ObservabilityService
	service.TelegramService
	service.VersionService
}

const maxDatabaseImportBytes = 64 << 20

func (a *ApiService) LoadData(c *gin.Context) {
	data, err := a.getData(c)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, nil)
}

func (a *ApiService) getData(c *gin.Context) (interface{}, error) {
	data := make(map[string]interface{}, 0)
	lu := c.Query("lu")
	isUpdated, err := a.ConfigService.CheckChanges(lu)
	if err != nil {
		return "", err
	}
	onlines, err := a.StatsService.GetOnlines()

	sysInfo := a.ServerService.GetSingboxInfo()
	if sysInfo["running"] == false {
		logs := a.ServerService.GetLogs("1", "debug")
		if len(logs) > 0 {
			data["lastLog"] = logs[0]
		}
	}

	if err != nil {
		return "", err
	}
	if isUpdated {
		config, err := a.SettingService.GetConfig()
		if err != nil {
			return "", err
		}
		clients, err := a.ClientService.GetAll()
		if err != nil {
			return "", err
		}
		tlsConfigs, err := a.TlsService.GetAll()
		if err != nil {
			return "", err
		}
		inbounds, err := a.InboundService.GetAll()
		if err != nil {
			return "", err
		}
		outbounds, err := a.OutboundService.GetAll()
		if err != nil {
			return "", err
		}
		endpoints, err := a.EndpointService.GetAll()
		if err != nil {
			return "", err
		}
		services, err := a.ServicesService.GetAll()
		if err != nil {
			return "", err
		}
		subURI, err := a.SettingService.GetFinalSubURI(getHostname(c))
		if err != nil {
			return "", err
		}
		subJsonURI, err := a.SettingService.GetSubJsonURI()
		if err != nil {
			return "", err
		}
		subClashURI, err := a.SettingService.GetSubClashURI()
		if err != nil {
			return "", err
		}
		trafficAge, err := a.SettingService.GetTrafficAge()
		if err != nil {
			return "", err
		}
		data["config"] = json.RawMessage(config)
		data["clients"] = clients
		data["tls"] = tlsConfigs
		data["inbounds"] = inbounds
		data["outbounds"] = outbounds
		data["endpoints"] = endpoints
		data["services"] = services
		data["subURI"] = subURI
		if subJsonURI != "" {
			data["subJsonURI"] = subJsonURI
		}
		if subClashURI != "" {
			data["subClashURI"] = subClashURI
		}
		data["enableTraffic"] = trafficAge > 0
		data["onlines"] = onlines
	} else {
		data["onlines"] = onlines
	}

	return data, nil
}

func (a *ApiService) LoadPartialData(c *gin.Context, objs []string) error {
	data := make(map[string]interface{}, 0)
	id := c.Query("id")

	for _, obj := range objs {
		switch obj {
		case "inbounds":
			inbounds, err := a.InboundService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = inbounds
		case "outbounds":
			outbounds, err := a.OutboundService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = outbounds
		case "endpoints":
			endpoints, err := a.EndpointService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = endpoints
		case "services":
			services, err := a.ServicesService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = services
		case "tls":
			tlsConfigs, err := a.TlsService.GetAll()
			if err != nil {
				return err
			}
			data[obj] = tlsConfigs
		case "clients":
			clients, err := a.ClientService.Get(id)
			if err != nil {
				return err
			}
			data[obj] = clients
		case "config":
			config, err := a.SettingService.GetConfig()
			if err != nil {
				return err
			}
			data[obj] = json.RawMessage(config)
		case "settings":
			settings, err := a.SettingService.GetAllSetting()
			if err != nil {
				return err
			}
			data[obj] = settings
		}
	}

	jsonObj(c, data, nil)
	return nil
}

func (a *ApiService) GetUsers(c *gin.Context) {
	users, err := a.UserService.GetUsers()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, *users, nil)
}

func (a *ApiService) GetSettings(c *gin.Context) {
	data, err := a.SettingService.GetAllSetting()
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStats(c *gin.Context) {
	resource := c.Query("resource")
	tag := c.Query("tag")
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 100
	}
	data, err := a.StatsService.GetStats(resource, tag, limit)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	jsonObj(c, data, err)
}

func (a *ApiService) GetStatus(c *gin.Context) {
	request := c.Query("r")
	result := a.ServerService.GetStatus(request)
	jsonObj(c, result, nil)
}

func (a *ApiService) GetOnlines(c *gin.Context) {
	onlines, err := a.StatsService.GetOnlines()
	jsonObj(c, onlines, err)
}

func (a *ApiService) GetLogs(c *gin.Context) {
	count := c.Query("count")
	if count == "" {
		count = c.Query("c")
	}
	level := c.Query("level")
	if level == "" {
		level = c.Query("l")
	}
	logs, err := a.ServerService.GetLogsFiltered(count, level, c.Query("source"), c.Query("filter"))
	if err != nil {
		c.JSON(http.StatusBadRequest, Msg{Success: false, Msg: "logs: " + err.Error()})
		return
	}
	jsonObj(c, logs, nil)
}

func (a *ApiService) CheckChanges(c *gin.Context) {
	actor := c.Query("a")
	chngKey := c.Query("k")
	count := c.Query("c")
	changes := a.ConfigService.GetChanges(actor, chngKey, count)
	jsonObj(c, changes, nil)
}

func (a *ApiService) GetKeypairs(c *gin.Context) {
	kType := c.Query("k")
	options := c.Query("o")
	keypair := a.ServerService.GenKeypair(kType, options)
	jsonObj(c, keypair, nil)
}

func (a *ApiService) GetDb(c *gin.Context) {
	if !a.requireTokenScopeAny(c, "database", "admin") {
		return
	}
	exclude := c.Query("exclude")
	db, err := database.GetDb(exclude)
	if err != nil {
		a.recordAudit(c, requestActor(c), "db_export_failed", "database", service.AuditSeverityWarn, map[string]any{
			"channel": "download",
		})
		jsonMsg(c, "", err)
		return
	}
	a.recordAudit(c, requestActor(c), "db_exported", "database", service.AuditSeverityWarn, map[string]any{
		"channel": "download",
		"exclude": exclude,
	})
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", "attachment; filename=s-ui_"+time.Now().Format("20060102-150405")+".db")
	c.Writer.Write(db)
}

func (a *ApiService) postActions(c *gin.Context) (string, json.RawMessage, error) {
	var data map[string]json.RawMessage
	err := c.ShouldBind(&data)
	if err != nil {
		return "", nil, err
	}
	return string(data["action"]), data["data"], nil
}

func (a *ApiService) Login(c *gin.Context) {
	remoteIP := getRemoteIp(c)
	username := c.Request.FormValue("user")
	if err := checkLoginRateLimit(remoteIP); err != nil {
		a.recordAudit(c, username, "login_blocked", "auth", service.AuditSeverityWarn, map[string]any{
			"reason": "rate_limit",
		})
		jsonMsg(c, "", err)
		return
	}
	loginUser, err := a.UserService.Login(username, c.Request.FormValue("pass"), remoteIP)
	if err != nil {
		recordLoginFailure(remoteIP)
		a.recordAudit(c, username, "login_failed", "auth", service.AuditSeverityWarn, map[string]any{
			"reason": err.Error(),
		})
		a.TelegramService.NotifyTelegramEvent("login_failed", telegramRequestFields(c))
		jsonMsg(c, "", err)
		return
	}
	resetLoginFailures(remoteIP)

	sessionMaxAge, err := a.SettingService.GetSessionMaxAge()
	if err != nil {
		logger.Infof("Unable to get session's max age from DB")
	}

	sessionGeneration, err := a.SettingService.GetSessionGeneration()
	if err != nil {
		logger.Warning("unable to get session generation:", err)
	}

	err = SetLoginUser(c, loginUser, sessionMaxAge, sessionGeneration)
	if err == nil {
		logger.Info("user ", loginUser, " login success")
		a.recordAudit(c, loginUser, "login_success", "auth", service.AuditSeverityInfo, nil)
		a.TelegramService.NotifyTelegramEvent("login_success", map[string]string{
			"user": loginUser,
			"ip":   remoteIP,
		})
	} else {
		logger.Warning("login failed: ", err)
		a.recordAudit(c, loginUser, "login_session_failed", "auth", service.AuditSeverityWarn, map[string]any{
			"reason": err.Error(),
		})
	}

	jsonMsg(c, "", nil)
}

func (a *ApiService) ChangePass(c *gin.Context) {
	id := c.Request.FormValue("id")
	oldPass := c.Request.FormValue("oldPass")
	newUsername := c.Request.FormValue("newUsername")
	newPass := c.Request.FormValue("newPass")
	err := a.UserService.ChangePass(id, oldPass, newUsername, newPass)
	if err == nil {
		logger.Info("change user credentials success")
		a.recordAudit(c, GetLoginUser(c), "admin_credentials_changed", "admin", service.AuditSeverityWarn, map[string]any{
			"targetUserId": id,
			"newUsername":  newUsername,
		})
		jsonMsg(c, "save", nil)
	} else {
		logger.Warning("change user credentials failed:", err)
		jsonMsg(c, "", err)
	}
}

func (a *ApiService) Save(c *gin.Context, loginUser string) {
	hostname := getHostname(c)
	obj := c.Request.FormValue("object")
	act := c.Request.FormValue("action")
	data := c.Request.FormValue("data")
	initUsers := c.Request.FormValue("initUsers")
	objs, err := a.ConfigService.Save(obj, act, json.RawMessage(data), initUsers, loginUser, hostname)
	if err != nil {
		jsonMsg(c, "save", err)
		return
	}
	err = a.LoadPartialData(c, objs)
	if err != nil {
		jsonMsg(c, obj, err)
	}
}

func (a *ApiService) RestartApp(c *gin.Context) {
	err := a.PanelService.RestartPanel(3)
	jsonMsg(c, "restartApp", err)
}

func (a *ApiService) RestartSb(c *gin.Context) {
	err := a.ConfigService.RestartCore()
	if err != nil {
		a.TelegramService.NotifyTelegramEvent("core_restart_failed", coreRestartFailedTelegramFields(c, err))
	} else {
		a.TelegramService.NotifyTelegramEvent("core_restarted", nil)
	}
	jsonMsg(c, "restartSb", err)
}

func telegramRequestFields(c *gin.Context) map[string]string {
	return map[string]string{
		"ip":      getRemoteIp(c),
		"ua_hash": hashUserAgent(c.Request.UserAgent()),
		"ts":      time.Now().UTC().Format(time.RFC3339),
	}
}

func hashUserAgent(userAgent string) string {
	sum := sha256.Sum256([]byte(userAgent))
	return hex.EncodeToString(sum[:])
}

func coreRestartFailedTelegramFields(c *gin.Context, err error) map[string]string {
	fields := telegramRequestFields(c)
	fields["errorClass"] = coreRestartErrorClass(err)
	return fields
}

func coreRestartErrorClass(err error) string {
	if err == nil {
		return ""
	}
	message := strings.ToLower(redact.String(err.Error()))
	switch {
	case strings.Contains(message, "timeout"), strings.Contains(message, "deadline exceeded"):
		return "timeout"
	case strings.Contains(message, "permission"), strings.Contains(message, "access is denied"):
		return "permission"
	case strings.Contains(message, "config"), strings.Contains(message, "parse"), strings.Contains(message, "json"):
		return "config"
	default:
		return "failed"
	}
}

func (a *ApiService) LinkConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, _, err := util.GetOutbound(link, 0)
	jsonObj(c, result, err)
}

func (a *ApiService) SubConvert(c *gin.Context) {
	link := c.Request.FormValue("link")
	result, err := util.GetExternalSub(link)
	jsonObj(c, result, err)
}

func (a *ApiService) ImportDb(c *gin.Context) {
	if !a.requireTokenScopeAny(c, "database", "admin") {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDatabaseImportBytes)
	file, _, err := c.Request.FormFile("db")
	if err != nil {
		a.recordAudit(c, requestActor(c), "db_import_failed", "database", service.AuditSeverityWarn, map[string]any{
			"reason": databaseImportErrorClass(err),
		})
		jsonMsg(c, "", err)
		return
	}
	defer file.Close()
	err = database.ImportDB(file)
	if err != nil {
		a.recordAudit(c, requestActor(c), "db_import_failed", "database", service.AuditSeverityWarn, map[string]any{
			"reason": databaseImportErrorClass(err),
		})
	} else {
		a.recordAudit(c, requestActor(c), "db_imported", "database", service.AuditSeverityWarn, nil)
	}
	jsonMsg(c, "", err)
}

func databaseImportErrorClass(err error) string {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return "too_large"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "format"), strings.Contains(msg, "sqlite"), strings.Contains(msg, "integrity"):
		return "invalid_db"
	default:
		return "failed"
	}
}

func (a *ApiService) RotateSubSecret(c *gin.Context) {
	if !a.requireTokenScopeAny(c, "client", "admin", "write") {
		return
	}
	clientID := c.Query("id")
	clientName, err := a.ClientService.RotateSubSecret(clientID)
	if err == nil {
		a.recordAudit(c, requestActor(c), "sub_secret_rotated", "client", service.AuditSeverityWarn, map[string]any{
			"clientId": clientID,
			"client":   clientName,
		})
		realtime.Publish(realtime.TopicConfigInvalidated, nil)
	}
	jsonMsg(c, "rotateSubSecret", err)
}

func (a *ApiService) Logout(c *gin.Context) {
	loginUser := GetLoginUser(c)
	if loginUser != "" {
		logger.Infof("user %s logout", loginUser)
		a.recordAudit(c, loginUser, "logout", "auth", service.AuditSeverityInfo, nil)
	}
	ClearSession(c)
	jsonMsg(c, "", nil)
}

func (a *ApiService) LogoutAllAdmins(c *gin.Context) {
	loginUser := GetLoginUser(c)
	_, err := a.SettingService.RotateSessionGeneration()
	if err == nil {
		if loginUser != "" {
			logger.Infof("user %s logged out all admin web sessions", loginUser)
		}
		a.recordAudit(c, loginUser, "logout_all_admins", "auth", service.AuditSeverityWarn, nil)
		a.TelegramService.NotifyTelegramEvent("logout_all_admins", map[string]string{
			"user": loginUser,
		})
		ClearSession(c)
	}
	jsonMsg(c, "logoutAllAdmins", err)
}

func (a *ApiService) LoadTokens() ([]byte, error) {
	return a.UserService.LoadTokens()
}

func (a *ApiService) GetTokens(c *gin.Context) {
	loginUser := GetLoginUser(c)
	tokens, err := a.UserService.GetUserTokens(loginUser)
	jsonObj(c, tokens, err)
}

func (a *ApiService) AddToken(c *gin.Context) {
	loginUser := GetLoginUser(c)
	expiry := c.Request.FormValue("expiry")
	expiryInt, err := strconv.ParseInt(expiry, 10, 64)
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	desc := c.Request.FormValue("desc")
	scope := c.DefaultPostForm("scope", "admin")
	token, err := a.UserService.AddToken(loginUser, expiryInt, desc, scope)
	if err == nil {
		a.recordAudit(c, loginUser, "api_token_created", "api_token", service.AuditSeverityWarn, map[string]any{
			"desc":   desc,
			"expiry": expiryInt,
			"scope":  scope,
		})
	}
	jsonObj(c, token, err)
}

func (a *ApiService) DeleteToken(c *gin.Context) {
	tokenId := c.Request.FormValue("id")
	err := a.UserService.DeleteToken(tokenId)
	if err == nil {
		a.recordAudit(c, GetLoginUser(c), "api_token_deleted", "api_token", service.AuditSeverityWarn, map[string]any{
			"id": tokenId,
		})
	}
	jsonMsg(c, "", err)
}

func (a *ApiService) SetTokenEnabled(c *gin.Context) {
	id := c.Request.FormValue("id")
	enabled, err := strconv.ParseBool(c.Request.FormValue("enabled"))
	if err != nil {
		jsonMsg(c, "", err)
		return
	}
	err = a.UserService.SetTokenEnabled(id, enabled)
	if err == nil {
		a.recordAudit(c, GetLoginUser(c), "api_token_enabled_changed", "api_token", service.AuditSeverityWarn, map[string]any{
			"id":      id,
			"enabled": enabled,
		})
	}
	jsonMsg(c, "save", err)
}

func (a *ApiService) GetSingboxConfig(c *gin.Context) {
	rawConfig, err := a.ConfigService.GetConfig("")
	if err != nil {
		c.Status(400)
		c.Writer.WriteString(err.Error())
		return
	}
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", "attachment; filename=config_"+time.Now().Format("20060102-150405")+".json")
	c.Writer.Write(*rawConfig)
}

func (a *ApiService) GetCheckOutbound(c *gin.Context) {
	tag := c.Query("tag")
	link := c.Query("link")
	result := a.ConfigService.CheckOutbound(tag, link)
	jsonObj(c, result, nil)
}
