package zendesk

import (
	"github.com/rnpridgeon/utils/collections"
	"github.com/rnpridgeon/utils/workerpool"
	"github.com/valyala/fasthttp"
	"sync"
	"sync/atomic"
)

var (
	WG       = &sync.WaitGroup{}
	taskPool = sync.Pool{
		New: func() interface{} { return &Task{} },
	}
)

type DispatcherStruct struct {
	w     *workerpool.WorkerManager
	Stop  chan struct{}
	Latch *sync.WaitGroup
}

func NewDispatcher(requestQueue chan *Task) (stop chan struct{}) {
	w := workerpool.NewWorkerManager(25)
	stop = make(chan struct{})
	go func() {
		for {
			select {
			case req := <-requestQueue:
				req.add()
				WG.Add(1)
				w.Execute(func() {
					req.Process(requestQueue)
				})
			case <-stop:
				return
			}
		}
	}()
	return stop
}

type Task struct {
	req       *fasthttp.Request
	errors    *collections.DEQueue
	refCount  int64
	onSuccess func(interface{})
	onFailure func(error)
}

func (t *Task) add() {
	atomic.AddInt64(&t.refCount, 1)
}

func (t *Task) done() {
	atomic.AddInt64(&t.refCount, -1)
}

func AcquireTask(errorQueue *collections.DEQueue) (t *Task) {
	t = taskPool.Get().(*Task)
	t.errors = errorQueue
	return t
}

func ReleaseTask(t *Task) {
	t.done()
	if atomic.LoadInt64(&t.refCount) <= 0 {
		return
	}
	fasthttp.ReleaseRequest(t.req)
	t.onFailure = nil
	t.onSuccess = nil
	taskPool.Put(t)
}
