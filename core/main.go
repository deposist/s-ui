package core

import (
	"context"
	"sync"

	"github.com/deposist/s-ui-rus-inst/logger"

	sb "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	_ "github.com/sagernet/sing-box/experimental/clashapi"
	_ "github.com/sagernet/sing-box/experimental/v2rayapi"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	_ "github.com/sagernet/sing-box/transport/v2rayquic"
	"github.com/sagernet/sing/service"
)

var (
	globalCtx = struct {
		sync.RWMutex
		value context.Context
	}{}
)

type Core struct {
	access          sync.RWMutex
	isRunning       bool
	instance        *Box
	inboundManager  adapter.InboundManager
	outboundManager adapter.OutboundManager
	serviceManager  adapter.ServiceManager
	endpointManager adapter.EndpointManager
	router          adapter.Router
	factory         log.Factory
}

type coreRuntime struct {
	ctx             context.Context
	inboundManager  adapter.InboundManager
	outboundManager adapter.OutboundManager
	serviceManager  adapter.ServiceManager
	endpointManager adapter.EndpointManager
	router          adapter.Router
	factory         log.Factory
}

func NewCore() *Core {
	ctx := context.Background()
	ctx = sb.Context(ctx, InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry())
	setGlobalCtx(ctx)
	return &Core{
		isRunning: false,
		instance:  nil,
	}
}

func (c *Core) GetCtx() context.Context {
	return getGlobalCtx()
}

func (c *Core) GetInstance() *Box {
	c.access.RLock()
	defer c.access.RUnlock()
	return c.instance
}

func (c *Core) Start(sbConfig []byte) error {
	var opt option.Options
	ctx := c.GetCtx()
	err := opt.UnmarshalJSONContext(ctx, sbConfig)
	if err != nil {
		logger.Error("Unmarshal config err:", err.Error())
	}

	instance, err := NewBox(Options{
		Context: ctx,
		Options: opt,
	})
	if err != nil {
		return err
	}

	err = instance.Start()
	if err != nil {
		_ = instance.Close()
		return err
	}

	ctx = service.ContextWith(ctx, c)
	setGlobalCtx(ctx)

	c.access.Lock()
	c.instance = instance
	c.isRunning = true
	c.inboundManager = instance.Inbound()
	c.outboundManager = instance.Outbound()
	c.serviceManager = instance.Service()
	c.endpointManager = instance.Endpoint()
	c.router = instance.Router()
	c.factory = instance.LogFactory()
	c.access.Unlock()
	return nil
}

func (c *Core) Stop() error {
	c.access.Lock()
	c.isRunning = false
	if c.instance == nil {
		c.access.Unlock()
		return nil
	}
	instance := c.instance
	c.instance = nil
	c.inboundManager = nil
	c.outboundManager = nil
	c.serviceManager = nil
	c.endpointManager = nil
	c.router = nil
	c.factory = nil
	c.access.Unlock()
	err := instance.Close()
	return err
}

func (c *Core) IsRunning() bool {
	c.access.RLock()
	defer c.access.RUnlock()
	return c.isRunning
}

func getGlobalCtx() context.Context {
	globalCtx.RLock()
	defer globalCtx.RUnlock()
	if globalCtx.value == nil {
		return context.Background()
	}
	return globalCtx.value
}

func setGlobalCtx(ctx context.Context) {
	globalCtx.Lock()
	globalCtx.value = ctx
	globalCtx.Unlock()
}

func (c *Core) runtime() (coreRuntime, bool) {
	c.access.RLock()
	defer c.access.RUnlock()
	if !c.isRunning || c.instance == nil {
		return coreRuntime{}, false
	}
	return coreRuntime{
		ctx:             c.GetCtx(),
		inboundManager:  c.inboundManager,
		outboundManager: c.outboundManager,
		serviceManager:  c.serviceManager,
		endpointManager: c.endpointManager,
		router:          c.router,
		factory:         c.factory,
	}, true
}

func (c *Core) Router() adapter.Router {
	c.access.RLock()
	defer c.access.RUnlock()
	return c.router
}

func (c *Core) OutboundManager() adapter.OutboundManager {
	c.access.RLock()
	defer c.access.RUnlock()
	return c.outboundManager
}
