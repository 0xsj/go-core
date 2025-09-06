// internal/container/lifecycle.go
package container

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

func (c *Container) Start(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if !c.built {
        return fmt.Errorf("container must be built before starting")
    }

    if c.started {
        return fmt.Errorf("container is already started")
    }

    startOrder, err := c.getStartupOrder()
    if err != nil {
        return fmt.Errorf("failed to determine startup order: %w", err)
    }

    startedServices := make([]reflect.Type, 0)
    
    for _, serviceType := range startOrder {
        service := c.services[serviceType]
        
        if service.Scope == ScopeSingleton && service.Instance == nil {
            if err := c.createSingletonInstance(service); err != nil {
                c.rollbackStartup(ctx, startedServices)
                return fmt.Errorf("failed to create singleton %v: %w", serviceType, err)
            }
        }

        if err := c.startService(ctx, service); err != nil {
            c.rollbackStartup(ctx, startedServices)
            return fmt.Errorf("failed to start service %v: %w", serviceType, err)
        }

        startedServices = append(startedServices, serviceType)
    }

    c.started = true
    return nil
}

func (c *Container) Stop(ctx context.Context) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if !c.started {
        return nil 
    }

    stopOrder, err := c.getShutdownOrder()
    if err != nil {
        return fmt.Errorf("failed to determine shutdown order: %w", err)
    }

    var errors []error
    var wg sync.WaitGroup
    errorChan := make(chan error, len(stopOrder))

    for _, serviceType := range stopOrder {
        service := c.services[serviceType]
        
        wg.Add(1)
        go func(svc *ServiceInfo, svcType reflect.Type) {
            defer wg.Done()
            
            if err := c.stopService(ctx, svc); err != nil {
                errorChan <- fmt.Errorf("failed to stop service %v: %w", svcType, err)
            }
        }(service, serviceType)
    }

    wg.Wait()
    close(errorChan)

    for err := range errorChan {
        errors = append(errors, err)
    }

    c.started = false

    if len(errors) > 0 {
        return fmt.Errorf("errors during shutdown: %v", errors)
    }

    return nil
}

func (c *Container) getStartupOrder() ([]reflect.Type, error) {
    graph := make(map[reflect.Type][]reflect.Type)
    inDegree := make(map[reflect.Type]int)

    for serviceType, service := range c.services {
        graph[serviceType] = make([]reflect.Type, 0)
        inDegree[serviceType] = len(service.Dependencies)
        
        for _, dep := range service.Dependencies {
            graph[dep] = append(graph[dep], serviceType)
        }
    }

    queue := make([]reflect.Type, 0)
    result := make([]reflect.Type, 0)

    for serviceType, degree := range inDegree {
        if degree == 0 {
            queue = append(queue, serviceType)
        }
    }

    for len(queue) > 0 {
        current := queue[0]
        queue = queue[1:]
        result = append(result, current)

        for _, dependent := range graph[current] {
            inDegree[dependent]--
            if inDegree[dependent] == 0 {
                queue = append(queue, dependent)
            }
        }
    }

    return result, nil
}

func (c *Container) getShutdownOrder() ([]reflect.Type, error) {
    startOrder, err := c.getStartupOrder()
    if err != nil {
        return nil, err
    }

    shutdownOrder := make([]reflect.Type, len(startOrder))
    for i, serviceType := range startOrder {
        shutdownOrder[len(startOrder)-1-i] = serviceType
    }

    return shutdownOrder, nil
}

func (c *Container) createSingletonInstance(service *ServiceInfo) error {
    providerValue := reflect.ValueOf(service.Provider)
    provideMethod := providerValue.MethodByName("Provide")
    
    if !provideMethod.IsValid() {
        return fmt.Errorf("provider does not have Provide method")
    }

    results := provideMethod.Call(nil)
    if len(results) != 1 {
        return fmt.Errorf("provide method should return exactly one value")
    }

    service.Instance = results[0].Interface()
    return nil
}

func (c *Container) startService(ctx context.Context, service *ServiceInfo) error {
    var instance any

    if service.Scope == ScopeSingleton {
        instance = service.Instance
    } else {
        if err := c.createSingletonInstance(service); err != nil {
            return err
        }
        instance = service.Instance
        service.Instance = nil // Clear it since it's not a singleton
    }

    if startable, ok := instance.(Startable); ok {
        startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
        defer cancel()
        
        return startable.Start(startCtx)
    }

    return nil
}

func (c *Container) stopService(ctx context.Context, service *ServiceInfo) error {
    if service.Scope == ScopeSingleton && service.Instance != nil {
        if stoppable, ok := service.Instance.(Stoppable); ok {
            stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
            defer cancel()
            
            return stoppable.Stop(stopCtx)
        }
    }

    return nil
}

func (c *Container) rollbackStartup(ctx context.Context, startedServices []reflect.Type) {
    for i := len(startedServices) - 1; i >= 0; i-- {
        serviceType := startedServices[i]
        service := c.services[serviceType]
        
        if err := c.stopService(ctx, service); err != nil {
            fmt.Printf("Error during startup rollback for service %v: %v\n", serviceType, err)
        }
    }
}