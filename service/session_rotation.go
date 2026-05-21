package service

import "sync"

var wsTokenInvalidationHooks = struct {
	sync.Mutex
	byName map[string]func() int
}{
	byName: map[string]func() int{},
}

func RegisterWSTokenInvalidationHook(name string, fn func() int) {
	if name == "" {
		return
	}
	wsTokenInvalidationHooks.Lock()
	defer wsTokenInvalidationHooks.Unlock()
	if fn == nil {
		delete(wsTokenInvalidationHooks.byName, name)
		return
	}
	wsTokenInvalidationHooks.byName[name] = fn
}

func invalidateWSTokensForSessionRotation() int {
	wsTokenInvalidationHooks.Lock()
	hooks := make([]func() int, 0, len(wsTokenInvalidationHooks.byName))
	for _, hook := range wsTokenInvalidationHooks.byName {
		hooks = append(hooks, hook)
	}
	wsTokenInvalidationHooks.Unlock()

	total := 0
	for _, hook := range hooks {
		total += hook()
	}
	return total
}
