package limited_request

import (
	"fmt"
	"io/ioutil"
	"math"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"

	"github.com/cabernety/gopkg/httplib"
	"golang.org/x/sync/errgroup"
)

// Request 在规定的时间间隔内，发送最多指定数量的请求
type Request interface {
	Add(ts []*Task)
	Start() error
	Stop()
	Results() <-chan *Task
	Status() string
	FinishedCount() int
}

type Task struct {
	ID      string
	URL     string
	Temp    map[string]interface{}
	Headers map[string]string
	Method  string
	Params  map[string]string
	Body    []byte
}

type Options struct {
	RequestThresholdPerSecond int
}

func New(opt *Options) Request {
	rtp := opt.RequestThresholdPerSecond
	return &request{
		requestThresholdPerSecond: rtp,
		taskQueue:                 make(chan *Task, 10),
		countCh:                   make(chan int, rtp),
		results:                   make(chan *Task),
		done:                      make(chan struct{}),
		workingTasks:              make(map[string]*Task),
	}
}

type request struct {
	taskQueue chan *Task
	// 用来计数，保证每秒请求数不超过阙值
	countCh                   chan int
	requestThresholdPerSecond int
	results                   chan *Task
	workingTasks              map[string]*Task
	total                     int
	finished                  int
	sync.RWMutex
	done chan struct{}
}

func (r *request) Add(ts []*Task) {
	for _, t := range ts {
		select {
		case <-r.done:
			return
		case r.taskQueue <- t:
			r.total++
		}
	}
}

// func (r *request) doTest(t *Task) {
// 	r.RLock()
// 	_, ok := r.workingTasks[t.ID]
// 	r.RUnlock()
// 	if ok {
// 		logrus.Error("LimitedRequest task id duplicated(%s)", t.ID)
// 		return
// 	}
// 	r.Lock()
// 	r.workingTasks[t.ID] = t
// 	r.Unlock()
// 	time.Sleep(time.Millisecond * time.Duration(rand.Intn(3000)))
// 	r.results <- []byte(fmt.Sprintf("task(%s) test string", t.ID))
// 	r.Lock()
// 	r.finished++
// 	delete(r.workingTasks, t.ID)
// 	r.Unlock()
// }
func (r *request) do(t *Task) {
	r.RLock()
	_, ok := r.workingTasks[t.ID]
	r.RUnlock()
	if ok {
		logrus.Error("LimitedRequest task id duplicated(%s)", t.ID)
		return
	}
	r.Lock()
	r.workingTasks[t.ID] = t
	r.Unlock()
	var req *httplib.Request
	if t.Method == "GET" {
		req = httplib.Get(t.URL)
		for k, v := range t.Params {
			req.Param(k, v)
		}
	} else {
		req = httplib.Post(t.URL)
		req.Body(t.Body)
	}
	req.SetTimeout(time.Second*10, time.Second*10)
	for k, v := range t.Headers {
		req.Header(k, v)
	}
	res, err := req.Response()
	if err != nil {
		// todo
		panic(err)
	}
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		// todo
		panic(err)
	}
	t.Body = b
	r.results <- t
	r.Lock()
	r.finished++
	delete(r.workingTasks, t.ID)
	r.Unlock()
}

func (r *request) Status() string {
	return fmt.Sprintf("tasks(%d/%d)", r.finished, r.total)
}

func (r *request) FinishedCount() int {
	return r.finished
}

func (r *request) Results() <-chan *Task {
	return r.results
}

func (r *request) Start() error {

	var g errgroup.Group

	g.Go(func() error {
		for {
			// ts := time.Second * time.Duration(1000000/r.requestThresholdPerSecond)
			select {
			case <-time.After(time.Millisecond * time.Duration(math.Round(float64(1000/r.requestThresholdPerSecond)))):
				r.countCh <- 1
				// logrus.Debugf("countCh <- 1, cap %d", cap(r.countCh))
			case <-r.done:
				return nil
			}
		}
	})
	g.Go(func() error {
		for {
			t := <-r.taskQueue
			select {
			case <-r.countCh:
				go r.do(t)
			case <-r.done:
				return nil
			}
		}
	})

	return g.Wait()
}
func (r *request) Stop() {
	close(r.done)
}
