package sub

import (
	"strings"

	"github.com/deposist/s-ui-rus-inst/logger"
	"github.com/deposist/s-ui-rus-inst/service"

	"github.com/gin-gonic/gin"
)

type SubHandler struct {
	service.SettingService
	SubService
	JsonService
	ClashService
}

func NewSubHandler(g *gin.RouterGroup) {
	a := &SubHandler{}
	a.initRouter(g)
}

func (s *SubHandler) initRouter(g *gin.RouterGroup) {
	g.Use(rateLimitMiddleware())
	g.GET("/:subid", s.subs)
	g.HEAD("/:subid", s.subHeaders)
	g.GET("/json/:subid", s.json)
	g.HEAD("/json/:subid", s.subHeaders)
	g.GET("/clash/:subid", s.clash)
	g.HEAD("/clash/:subid", s.subHeaders)
}

func (s *SubHandler) subs(c *gin.Context) {
	format, isFormat := c.GetQuery("format")
	if isFormat {
		switch format {
		case "json":
			s.json(c)
		case "clash":
			s.clash(c)
		default:
			c.String(400, "Error!")
		}
		return
	}

	var headers []string
	var result *string
	var err error
	subId := c.Param("subid")
	result, headers, err = s.SubService.GetSubs(subId)
	if err != nil || result == nil {
		logger.Error(err)
		c.String(400, "Error!")
		return
	}

	s.writeResult(c, result, headers)
}

func (s *SubHandler) json(c *gin.Context) {
	result, headers, err := s.JsonService.GetJson(c.Param("subid"), "json")
	if err != nil || result == nil {
		logger.Error(err)
		c.String(400, "Error!")
		return
	}
	s.writeResult(c, result, headers)
}

func (s *SubHandler) clash(c *gin.Context) {
	result, headers, err := s.ClashService.GetClash(c.Param("subid"))
	if err != nil || result == nil {
		logger.Error(err)
		c.String(400, "Error!")
		return
	}
	s.writeResult(c, result, headers)
}

func (s *SubHandler) subHeaders(c *gin.Context) {
	subId := c.Param("subid")
	client, err := s.SubService.getClientBySubId(subId)
	if err != nil {
		logger.Error(err)
		c.String(400, "Error!")
		return
	}

	headers := s.SubService.getClientHeaders(client)
	s.addHeaders(c, headers)

	c.Status(200)
}

func (s *SubHandler) addHeaders(c *gin.Context, headers []string) {
	if len(headers) < 3 {
		return
	}
	c.Writer.Header().Set("Subscription-Userinfo", sanitizeHeaderValue(headers[0]))
	c.Writer.Header().Set("Profile-Update-Interval", sanitizeHeaderValue(headers[1]))
	c.Writer.Header().Set("Profile-Title", sanitizeHeaderValue(headers[2]))
}

func (s *SubHandler) writeResult(c *gin.Context, result *string, headers []string) {
	s.addHeaders(c, headers)
	c.String(200, *result)
}

func sanitizeHeaderValue(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0x7f || r < 0x20 {
			return -1
		}
		return r
	}, value)
}
