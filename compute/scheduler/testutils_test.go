package scheduler

import (
	"context"
	"os"
	"time"

	"github.com/ohsu-comp-bio/funnel/config"
	"github.com/ohsu-comp-bio/funnel/logger"
	"github.com/ohsu-comp-bio/funnel/util"
	"github.com/stretchr/testify/mock"
)

// testNode wraps Node with some testing helpers.
type testNode struct {
	*NodeProcess
	Client *MockClient
	done   chan struct{}
}

func newTestNode(conf config.Config) testNode {
	conf.Worker.WorkDir, _ = os.MkdirTemp("", "funnel-test-storage-")
	log := logger.NewLogger("test-node", logger.DebugConfig())

	// A mock scheduler client allows this code to fake/control the worker's
	// communication with a scheduler service.
	res, _ := detectResources(conf.Node, conf.Worker.WorkDir)
	s := new(MockClient)
	n := &NodeProcess{
		conf:      conf,
		client:    s,
		log:       log,
		resources: res,
		workerRun: NoopWorker,
		workers:   newRunSet(),
		timeout:   util.NewIdleTimeout(time.Duration(conf.Node.Timeout)),
		state:     NodeState_ALIVE,
	}

	s.On("PutNode", mock.Anything, mock.Anything).
		Return(nil, nil)
	s.On("PutNode", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	s.On("Close").Return(nil)

	return testNode{
		NodeProcess: n,
		Client:      s,
		done:        make(chan struct{}),
	}
}

func (t *testNode) Start() context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	t.Client.On("GetNode", mock.Anything, mock.Anything, mock.Anything).
		Return(&Node{}, nil)
	go func() {
		t.NodeProcess.Run(ctx)
		close(t.done)
	}()
	return cancel
}

func (t *testNode) Wait() {
	<-t.done
}

func (t *testNode) AddTasks(ids ...string) {
	// Set up the scheduler mock to assign a task to the worker.
	t.Client.On("GetNode", mock.Anything, mock.Anything, mock.Anything).
		Return(&Node{
			TaskIds: ids,
		}, nil).
		Once()

	t.Client.On("GetNode", mock.Anything, mock.Anything, mock.Anything).
		Return(&Node{}, nil)
}
