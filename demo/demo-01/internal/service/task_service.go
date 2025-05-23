package service

import (
	"context"
	"fmt"
	"sync"
	"thanhldt060802/internal/actor_model/types"
	"thanhldt060802/internal/dto"
	"thanhldt060802/internal/repository"

	"ergo.services/ergo/gen"
)

type taskService struct {
	taskRepository repository.TaskRepository
	node           gen.Node
	supervisorPID  gen.PID
}

type TaskService interface {
	GetExistedWorkers(ctx context.Context) (*dto.ExistedWorkers, error)
	DispatchTask(ctx context.Context, reqDTO *dto.DispatchTaskRequest) error
	RunTask(ctx context.Context, reqDTO *dto.RunTaskRequest) error
	RunTasks(ctx context.Context, reqDTO *dto.RunTasksRequest) error
}

func NewTaskService(taskRepository repository.TaskRepository, node gen.Node, supervisorPID gen.PID) TaskService {
	return &taskService{
		taskRepository: taskRepository,
		node:           node,
		supervisorPID:  supervisorPID,
	}
}

func (taskService *taskService) GetExistedWorkers(ctx context.Context) (*dto.ExistedWorkers, error) {
	workerNames := make(chan []string)
	running := make(chan []string)
	available := make(chan []string)

	message := types.GetExistedWorkersMessage{
		WorkerNames: workerNames,
		Running:     running,
		Available:   available,
	}

	if err := taskService.node.Send(taskService.supervisorPID, message); err != nil {
		return nil, fmt.Errorf("some thing wrong on actor model: %v", err.Error())
	}

	existedWorkers := &dto.ExistedWorkers{}
	wg := sync.WaitGroup{}
	wg.Add(3)

	go func() {
		existedWorkers.WorkerNames = <-workerNames
		wg.Done()
	}()
	go func() {
		existedWorkers.Running = <-running
		wg.Done()
	}()
	go func() {
		existedWorkers.Available = <-available
		wg.Done()
	}()

	wg.Wait()

	return existedWorkers, nil
}

func (taskService *taskService) DispatchTask(ctx context.Context, reqDTO *dto.DispatchTaskRequest) error {
	message := types.DispatchTaskMessage{
		WorkerName: reqDTO.Body.WorkerName,
		TaskId:     reqDTO.Body.TaskId,
	}
	if err := taskService.node.Send(taskService.supervisorPID, message); err != nil {
		return fmt.Errorf("some thing wrong on actor model: %v", err.Error())
	}

	return nil
}

func (taskService *taskService) RunTask(ctx context.Context, reqDTO *dto.RunTaskRequest) error {
	message := types.RunTaskMessage{
		TaskId: reqDTO.Body.TaskId,
	}
	if err := taskService.node.Send(taskService.supervisorPID, message); err != nil {
		return fmt.Errorf("some thing wrong on actor model: %v", err.Error())
	}

	return nil
}

func (taskService *taskService) RunTasks(ctx context.Context, reqDTO *dto.RunTasksRequest) error {
	message := types.RunTasksMessage{
		TaskIds: reqDTO.Body.TaskIds,
	}
	if err := taskService.node.Send(taskService.supervisorPID, message); err != nil {
		return fmt.Errorf("some thing wrong on actor model: %v", err.Error())
	}

	return nil
}
