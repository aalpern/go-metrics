package metrics

import (
	"reflect"
	"sync"
)

// An implementation of a Registry with read/write locking for
// high-throughput usage.
type ReadWriteRegistry struct {
	metrics map[string]interface{}
	mutex   sync.RWMutex
}

// Create a new registry with read/write locking.
func NewReadWriteRegistry() Registry {
	return &ReadWriteRegistry{metrics: make(map[string]interface{})}
}

// Call the given function for each registered metric.
func (r *ReadWriteRegistry) Each(f func(string, interface{})) {
	for name, i := range r.registered() {
		f(name, i)
	}
}

// Get the metric by the given name or nil if none is registered.
func (r *ReadWriteRegistry) Get(name string) interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.metrics[name]
}

// Gets an existing metric or creates and registers a new one. Threadsafe
// alternative to calling Get and Register on failure.
// The interface can be the metric to register if not found in registry,
// or a function returning the metric for lazy instantiation.
func (r *ReadWriteRegistry) GetOrRegister(name string, i interface{}) interface{} {
	r.mutex.RLock()
	if metric, ok := r.metrics[name]; ok {
		r.mutex.RUnlock()
		return metric
	}
	r.mutex.RUnlock()
	if v := reflect.ValueOf(i); v.Kind() == reflect.Func {
		i = v.Call(nil)[0].Interface()
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.register(name, i)
	return i
}

// Register the given metric under the given name.  Returns a DuplicateMetric
// if a metric by the given name is already registered.
func (r *ReadWriteRegistry) Register(name string, i interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.register(name, i)
}

// Run all registered healthchecks.
func (r *ReadWriteRegistry) RunHealthchecks() {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for _, i := range r.metrics {
		if h, ok := i.(Healthcheck); ok {
			h.Check()
		}
	}
}

// Unregister the metric with the given name.
func (r *ReadWriteRegistry) Unregister(name string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.metrics, name)
}

// Unregister all metrics.  (Mostly for testing.)
func (r *ReadWriteRegistry) UnregisterAll() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	for name, _ := range r.metrics {
		delete(r.metrics, name)
	}
}

func (r *ReadWriteRegistry) register(name string, i interface{}) error {
	if _, ok := r.metrics[name]; ok {
		return DuplicateMetric(name)
	}
	switch i.(type) {
	case Counter, Gauge, GaugeFloat64, Healthcheck, Histogram, Meter, Timer:
		r.metrics[name] = i
	}
	return nil
}

func (r *ReadWriteRegistry) registered() map[string]interface{} {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	metrics := make(map[string]interface{}, len(r.metrics))
	for name, i := range r.metrics {
		metrics[name] = i
	}
	return metrics
}
