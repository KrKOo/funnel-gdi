package boltdb

import (
	"fmt"
	"time"

	"github.com/boltdb/bolt"
	"github.com/ohsu-comp-bio/funnel/compute/scheduler"
	"github.com/ohsu-comp-bio/funnel/config"
	"github.com/ohsu-comp-bio/funnel/util/fsutil"
)

// TODO these should probably be unexported names

// TaskBucket defines the name of a bucket which maps
// task ID -> tes.Task struct
var TaskBucket = []byte("tasks")

// TasksQueued defines the name of a bucket which maps
// task ID -> nil
var TasksQueued = []byte("tasks-queued")

// TaskState maps: task ID -> state string
var TaskState = []byte("tasks-state")

// TasksLog defines the name of a bucket which maps
// task ID -> tes.TaskLog struct
var TasksLog = []byte("tasks-log")

// ExecutorLogs maps (task ID + executor index) -> tes.ExecutorLog struct
var ExecutorLogs = []byte("executor-logs")

// ExecutorStdout maps (task ID + executor index) -> tes.ExecutorLog.Stdout string
var ExecutorStdout = []byte("executor-stdout")

// ExecutorStderr maps (task ID + executor index) -> tes.ExecutorLog.Stderr string
var ExecutorStderr = []byte("executor-stderr")

// Nodes maps:
// node ID -> pbs.Node struct
var Nodes = []byte("nodes")

// SysLogs defeines the name of a bucket with maps
// task ID -> tes.TaskLog.SystemLogs
var SysLogs = []byte("system-logs")

// BoltDB provides handlers for gRPC endpoints.
// Data is stored/retrieved from the BoltDB key-value database.
type BoltDB struct {
	scheduler.UnimplementedSchedulerServiceServer
	db *bolt.DB
}

// NewBoltDB returns a new instance of BoltDB, accessing the database at
// the given path, and including the given ServerConfig.
func NewBoltDB(conf config.BoltDB) (*BoltDB, error) {
	if err := fsutil.EnsurePath(conf.Path); err != nil {
		return nil, err
	}
	db, err := bolt.Open(conf.Path, 0600, &bolt.Options{
		Timeout: time.Second * 5,
	})
	if err != nil {
		return nil, err
	}
	return &BoltDB{db: db}, nil
}

// Init creates the required BoltDB buckets
func (taskBolt *BoltDB) Init() error {
	// Check to make sure all the required buckets have been created
	return taskBolt.db.Update(func(tx *bolt.Tx) error {
		ensureBucket(tx, TaskBucket)
		ensureBucket(tx, TasksQueued)
		ensureBucket(tx, TaskState)
		ensureBucket(tx, TasksLog)
		ensureBucket(tx, ExecutorLogs)
		ensureBucket(tx, ExecutorStdout)
		ensureBucket(tx, ExecutorStderr)
		ensureBucket(tx, Nodes)
		ensureBucket(tx, SysLogs)
		return nil
	})
}

func ensureBucket(tx *bolt.Tx, id []byte) {
	if tx.Bucket(id) == nil {
		if _, err := tx.CreateBucket(id); err != nil {
			fmt.Printf("Detected error while creating non-existing "+
				"Bolt bucket [%s]: %s\n", string(id), err)
		}
	}
}

func (taskBolt *BoltDB) Close() {
	taskBolt.db.Close()
}
