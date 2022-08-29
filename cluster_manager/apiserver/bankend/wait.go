package bankend

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
)

type waitTasks struct {
	lock *sync.Mutex

	tasks map[string][]*waitTask
}

func NewWaitTasks() *waitTasks {
	return &waitTasks{
		lock:  new(sync.Mutex),
		tasks: make(map[string][]*waitTask),
	}
}

func (m *waitTasks) NewWaitTask(key string, interval time.Duration, updater updater) *waitTask {
	wt := NewWaitTask(interval, updater)
	wt.SetId(key)

	m.lock.Lock()

	tasks, ok := m.tasks[key]
	if !ok {
		m.tasks[key] = []*waitTask{wt}
	} else {
		tasks = append(tasks, wt)
		m.tasks[key] = tasks
	}

	m.lock.Unlock()

	return wt
}

func (m *waitTasks) cancelTask(key string) {
	tasks, ok := m.tasks[key]
	if ok {
		for i := range tasks {
			if tasks[i].cancel != nil {
				tasks[i].cancel()
			}
		}
	}
}

func (m *waitTasks) CancelTask(key string) {
	m.lock.Lock()

	m.cancelTask(key)

	m.lock.Unlock()
}

func (m *waitTasks) Delete(key string) {
	m.lock.Lock()

	m.cancelTask(key)

	delete(m.tasks, key)

	m.lock.Unlock()
}

//  func() (done bool, err error)
type ConditionFunc = wait.ConditionFunc

type updater func(err error) error

const (
	retryInterval = 10 * time.Second
	retryTimeout  = 30 * time.Second
)

type waitTask struct {
	id       string
	updater  updater
	cancel   context.CancelFunc
	interval time.Duration
}

func NewWaitTask(interval time.Duration, updater updater) *waitTask {
	if interval == 0 {
		interval = retryInterval
	}

	return &waitTask{
		id:       "not-set",
		interval: interval,
		updater:  updater,
	}
}

func NewWaitTaskWithId(id string, interval time.Duration, updater updater) *waitTask {
	if interval == 0 {
		interval = retryInterval
	}

	return &waitTask{
		id:       id,
		interval: interval,
		updater:  updater,
	}
}

func (w *waitTask) SetId(id string) {
	w.id = id
}

func (w *waitTask) AddCancel(cancel context.CancelFunc) {
	w.cancel = cancel
}

func (w *waitTask) WithContext(ctx context.Context, condition ConditionFunc) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithCancel(ctx)

	w.cancel = cancel

	return w.run(condition, ctx.Done())
}

func (w *waitTask) WithTimeout(timeout time.Duration, condition ConditionFunc) error {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	w.cancel = cancel
	defer cancel()

	return w.run(condition, ctx.Done())
}

func (w *waitTask) WithTimeoutAndCancel(timeout time.Duration, ctx context.Context, cancel context.CancelFunc, condition ConditionFunc) error {

	//ctx, cancel := context.WithTimeout(ctx, timeout)

	w.cancel = cancel
	defer cancel()

	return w.run(condition, ctx.Done())
}

func (w *waitTask) Until(stopCh <-chan struct{}, condition ConditionFunc) error {
	ctx, cancel := contextForChannel(stopCh)

	w.cancel = cancel
	defer cancel()

	return w.run(condition, ctx.Done())
}

func contextForChannel(parentCh <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		select {
		case <-parentCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (w *waitTask) run(condition ConditionFunc, stopCh <-chan struct{}) (err error) {
	if condition == nil {
		return nil
	}

	count := 0
	start := time.Now()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)

			klog.Fatalf("%s\n%s", err, debug.Stack())
		}

		if w.updater != nil {
			_err := w.updater(err)
			if _err != nil {
				err = fmt.Errorf("updater:%s;%v", _err, err)
			}
		}

		t := time.Since(start)
		if err == nil {
			klog.Infof("%s Task [%s] Done !!!", t, w.id)
		} else {
			klog.Errorf("%s Task [%s] Failed: %s", t, w.id, err)
		}
	}()

	cond := func() (bool, error) {
		count++

		ok, err := condition()
		if err != nil {
			klog.Errorf("Task [%s] wait %d: ok = %t err = %v", w.id, count, ok, err)
		}

		if ok {
			return ok, err
		}

		return false, err
	}

	err = wait.PollImmediateUntil(w.interval, cond, stopCh)

	return err
}
