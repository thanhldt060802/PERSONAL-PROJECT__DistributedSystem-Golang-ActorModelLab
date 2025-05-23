package types

type DispatchTaskMessage struct {
	WorkerName string
	TaskId     int64
}

type RunTaskMessage struct {
	TaskId int64
}

type RunTasksMessage struct {
	TaskIds []int64
}

type GetExistedWorkersMessage struct {
	WorkerNames chan []string
	Running     chan []string
	Available   chan []string
}
