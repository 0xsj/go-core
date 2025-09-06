// internal/container/types.go
package container

import (
	"context"
	"reflect"
	"time"
)

type ServiceScope int

const (
	ScopeSingleton ServiceScope = iota
	ScopeScoped
	ScopeTransient
)

func (s ServiceScope) String() string {
	switch s {
	case ScopeSingleton:
		return "Singleton"
	case ScopeScoped:
		return "Scoped"
	case ScopeTransient:
		return "Transient"
	default:
		return "Unknown"
	}
}

type Provider[T any] interface {
	Provide() T
}

type ServiceInfo struct {
	Type         reflect.Type
	Scope        ServiceScope
	Provider     any
	Dependencies []reflect.Type // For dependency graph analysis
	Instance     any            // For singletons
	CreatedAt    time.Time
}

type Startable interface {
	Start(ctx context.Context) error
}

type Stoppable interface {
	Stop(ctx context.Context) error
}

type HealthCheckable interface {
	HealthCheck(ctx context.Context) error
}
