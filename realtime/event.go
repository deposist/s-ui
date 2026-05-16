package realtime

type Topic string

const (
	TopicOnlines           Topic = "onlines"
	TopicTrafficDelta      Topic = "traffic_delta"
	TopicCoreState         Topic = "core_state"
	TopicConfigInvalidated Topic = "config_invalidated"
	TopicRestartStatus     Topic = "restart_status"
	TopicNotification      Topic = "notification"
	TopicSecurityEvent     Topic = "security_event"
)

type Scope string

const (
	ScopeAdmin         Scope = "admin"
	ScopeRead          Scope = "read"
	ScopeWrite         Scope = "write"
	ScopeObservability Scope = "observability"
)

type Event struct {
	Type    Topic       `json:"type"`
	Ts      int64       `json:"ts"`
	Payload interface{} `json:"payload,omitempty"`
}

func topicAllowed(topic Topic, scope Scope) bool {
	if topic == TopicSecurityEvent {
		return scope == ScopeAdmin
	}
	return true
}
