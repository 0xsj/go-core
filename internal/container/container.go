// internal/container/container.go
package container

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

type Container struct {
    services map[reflect.Type]*ServiceInfo
    built    bool
    started  bool
    mu       sync.RWMutex
}

func New() *Container {
    return &Container{
        services: make(map[reflect.Type]*ServiceInfo),
        built:    false,
        started:  false,
    }
}

func RegisterSingleton[T any](c *Container, provider Provider[T]) error {
    return register(c, ScopeSingleton, provider)
}

func RegisterScoped[T any](c *Container, provider Provider[T]) error {
    return register(c, ScopeScoped, provider)
}

func RegisterTransient[T any](c *Container, provider Provider[T]) error {
    return register(c, ScopeTransient, provider)
}

func register[T any](c *Container, scope ServiceScope, provider Provider[T]) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.built {
        return fmt.Errorf("cannot register services after container is built")
    }

    serviceType := reflect.TypeOf((*T)(nil)).Elem()
    
    if _, exists := c.services[serviceType]; exists {
        return fmt.Errorf("service of type %v is already registered", serviceType)
    }

    c.services[serviceType] = &ServiceInfo{
        Type:      serviceType,
        Scope:     scope,
        Provider:  provider,
        CreatedAt: time.Now(),
    }

    return nil
}

func Resolve[T any](c *Container) T {
    serviceType := reflect.TypeOf((*T)(nil)).Elem()
    
    c.mu.RLock()
    defer c.mu.RUnlock()

    if !c.built {
        panic(fmt.Sprintf("container must be built before resolving services. Service type: %v", serviceType))
    }

    service, exists := c.services[serviceType]
    if !exists {
        panic(fmt.Sprintf("service of type %v is not registered", serviceType))
    }

    if service.Scope == ScopeScoped {
        panic(fmt.Sprintf("scoped service of type %v must be resolved with ResolveScoped", serviceType))
    }

    return resolveService[T](c, service)
}

func ResolveScoped[T any](c *Container, ctx context.Context) T {
    serviceType := reflect.TypeOf((*T)(nil)).Elem()
    
    c.mu.RLock()
    defer c.mu.RUnlock()

    if !c.built {
        panic(fmt.Sprintf("container must be built before resolving services. Service type: %v", serviceType))
    }

    service, exists := c.services[serviceType]
    if !exists {
        panic(fmt.Sprintf("service of type %v is not registered", serviceType))
    }

    if service.Scope != ScopeScoped {
        panic(fmt.Sprintf("non-scoped service of type %v cannot be resolved with ResolveScoped", serviceType))
    }

    return resolveService[T](c, service)
}

func resolveService[T any](_ *Container, service *ServiceInfo) T {
    switch service.Scope {
    case ScopeSingleton:
        if service.Instance == nil {
            provider := service.Provider.(Provider[T])
            service.Instance = provider.Provide()
        }
        return service.Instance.(T)
        
    case ScopeScoped, ScopeTransient:
        provider := service.Provider.(Provider[T])
        return provider.Provide()
        
    default:
        panic(fmt.Sprintf("unknown service scope: %v", service.Scope))
    }
}

func (c *Container) Build() error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if c.built {
        return fmt.Errorf("container is already built")
    }

    if err := c.validateServices(); err != nil {
        return fmt.Errorf("service validation failed: %w", err)
    }

    if err := c.buildDependencyGraph(); err != nil {
        return fmt.Errorf("dependency graph construction failed: %w", err)
    }

    if err := c.detectCircularDependencies(); err != nil {
        return fmt.Errorf("circular dependency detected: %w", err)
    }

    if err := c.initializeEagerSingletons(); err != nil {
        return fmt.Errorf("singleton initialization failed: %w", err)
    }

    c.built = true
    return nil
}

func (c *Container) validateServices() error {
    if len(c.services) == 0 {
        return fmt.Errorf("no services registered")
    }

    for serviceType, service := range c.services {
        if service.Provider == nil {
            return fmt.Errorf("service %v has nil provider", serviceType)
        }

        if service.Scope < ScopeSingleton || service.Scope > ScopeTransient {
            return fmt.Errorf("service %v has invalid scope: %v", serviceType, service.Scope)
        }
    }

    return nil
}

func (c *Container) buildDependencyGraph() error {
    for serviceType, service := range c.services {
        deps, err := c.analyzeDependencies(service.Provider)
        if err != nil {
            return fmt.Errorf("failed to analyze dependencies for %v: %w", serviceType, err)
        }
        service.Dependencies = deps
    }
    return nil
}

func (c *Container) analyzeDependencies(provider any) ([]reflect.Type, error) {
    providerType := reflect.TypeOf(provider)
    if providerType.Kind() == reflect.Pointer {
        providerType = providerType.Elem()
    }

    var dependencies []reflect.Type

    if providerType.Kind() == reflect.Struct {
        for i := 0; i < providerType.NumField(); i++ {
            field := providerType.Field(i)
            
            if !field.IsExported() {
                continue
            }

            if _, exists := c.services[field.Type]; exists {
                dependencies = append(dependencies, field.Type)
            }
        }
    }

    return dependencies, nil
}

func (c *Container) detectCircularDependencies() error {
    graph := make(map[reflect.Type][]reflect.Type)
    inDegree := make(map[reflect.Type]int)

    for serviceType, service := range c.services {
        graph[serviceType] = service.Dependencies
        inDegree[serviceType] = 0
    }

    for _, deps := range graph {
        for _, dep := range deps {
            inDegree[dep]++
        }
    }

    queue := make([]reflect.Type, 0)
    processed := 0

    for serviceType, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, serviceType)
        }
    }

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]
        processed++

        for _, neighbor := range graph[current] {
            inDegree[neighbor]--
            if inDegree[neighbor] == 0 {
                queue = append(queue, neighbor)
            }
        }
    }
    if processed != len(c.services) {
        return fmt.Errorf("circular dependency detected in service graph")
    }

    return nil
}

func (c *Container) initializeEagerSingletons() error {
    for serviceType, service := range c.services {
        if service.Scope == ScopeSingleton && len(service.Dependencies) == 0 {
            _ = serviceType 
        }
    }
    return nil
}