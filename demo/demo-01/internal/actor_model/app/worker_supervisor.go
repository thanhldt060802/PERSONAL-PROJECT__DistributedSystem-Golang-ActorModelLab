package app

import (
	"fmt"
	"thanhldt060802/internal/actor_model/types"
	"thanhldt060802/internal/repository"

	"ergo.services/ergo/act"
	"ergo.services/ergo/gen"
)

type WorkerSupervisor struct {
	act.Supervisor

	taskRepository         repository.TaskRepository
	numberOfInitialWorkers int

	availableWorkerMap map[string]bool
}

func FactoryWorkerSupervisor() gen.ProcessBehavior {
	return &WorkerSupervisor{}
}

func (workerSupervisor *WorkerSupervisor) Init(args ...any) (act.SupervisorSpec, error) {
	workerSupervisor.taskRepository = args[0].(repository.TaskRepository)
	workerSupervisor.numberOfInitialWorkers = args[1].(int)
	workerSupervisor.availableWorkerMap = map[string]bool{}

	supervisorSpec := act.SupervisorSpec{}
	supervisorSpec.EnableHandleChild = true
	supervisorSpec.DisableAutoShutdown = true
	supervisorSpec.Type = act.SupervisorTypeOneForOne
	supervisorSpec.Restart.Strategy = act.SupervisorStrategyTransient
	supervisorSpec.Restart.Intensity = 100
	supervisorSpec.Restart.Period = 5

	supervisorSpec.Children = []act.SupervisorChildSpec{}
	for i := 1; i <= workerSupervisor.numberOfInitialWorkers; i++ {
		supervisorSpec.Children = append(supervisorSpec.Children, act.SupervisorChildSpec{
			Name:    gen.Atom(fmt.Sprintf("worker_%v", i)),
			Factory: FactoryWorkerActor,
			Options: gen.ProcessOptions{},
			Args:    nil,
		})
		workerSupervisor.availableWorkerMap[fmt.Sprintf("worker_%v", i)] = false
	}

	workerSupervisor.Log().Info("Started worker supervisor %v %v on %v", workerSupervisor.PID(), workerSupervisor.Name(), workerSupervisor.Node().Name())
	return supervisorSpec, nil
}

func (workerSupervisor *WorkerSupervisor) HandleChildStart(childName gen.Atom, pid gen.PID) error {
	workerSupervisor.Log().Info("Actor start with name %v and PID %v", childName, pid)
	return nil
}

func (workerSupervisor *WorkerSupervisor) HandleChildTerminate(name gen.Atom, pid gen.PID, reason error) error {
	if reason.Error() == gen.TerminateReasonNormal.Error() {
		workerName := name.String()
		workerName = workerName[1 : len(workerName)-1]
		workerSupervisor.availableWorkerMap[workerName] = true
	} else {
		workerSupervisor.Log().Error("Actor %v terminated. Panic reason: %v", name, reason.Error())
	}

	return nil
}

func (workerSupervisor *WorkerSupervisor) HandleMessage(from gen.PID, message any) error {
	switch receivedMessage := message.(type) {
	case types.DoStart:
		{
			for _, supervisorChildSpec := range workerSupervisor.Children() {
				workerSupervisor.Send(supervisorChildSpec.Name, types.DoStart{})
			}

			return nil
		}
	case types.GetExistedWorkersMessage:
		{
			workerSupervisor.getExistedWorkers(receivedMessage)
			return nil
		}
	case types.DispatchTaskMessage:
		{
			workerSupervisor.dispatchTask(receivedMessage)
			return nil
		}
	case types.RunTaskMessage:
		{
			workerSupervisor.runTask(receivedMessage)
			return nil
		}
	case types.RunTasksMessage:
		{
			workerSupervisor.runTasks(receivedMessage)
			return nil
		}
	}

	return nil
}

func (workerSupervisor *WorkerSupervisor) getExistedWorkers(message types.GetExistedWorkersMessage) {
	workerNames := []string{}
	running := []string{}
	available := []string{}

	for _, supervisorChildSpec := range workerSupervisor.Children() {
		workerName := supervisorChildSpec.Name.String()
		workerName = workerName[1 : len(workerName)-1]
		workerNames = append(workerNames, workerName)
		if workerSupervisor.availableWorkerMap[workerName] {
			available = append(available, workerName)
		} else {
			running = append(running, workerName)
		}
	}
	message.WorkerNames <- workerNames
	message.Running <- running
	message.Available <- available
}

func (workerSupervisor *WorkerSupervisor) dispatchTask(message types.DispatchTaskMessage) {
	if available, ok := workerSupervisor.availableWorkerMap[message.WorkerName]; ok {
		if available {
			workerSupervisor.Log().Info("Restart existed actor %v", message.WorkerName)
			if err := workerSupervisor.StartChild(gen.Atom(message.WorkerName), workerSupervisor.taskRepository, message.TaskId); err != nil {
				workerSupervisor.Log().Error("Restart existed actor %v failed: %v", message.WorkerName, err.Error())
			}
			workerSupervisor.availableWorkerMap[message.WorkerName] = false
			workerSupervisor.Log().Info("Restart existed actor %v successful", message.WorkerName)
		} else {
			workerSupervisor.Log().Warning("Actor %v is running", message.WorkerName)
		}
	} else {
		workerSupervisor.Log().Info("Start new actor %v", message.WorkerName)
		if err := workerSupervisor.AddChild(act.SupervisorChildSpec{
			Name:    gen.Atom(message.WorkerName),
			Factory: FactoryWorkerActor,
			Options: gen.ProcessOptions{},
			Args:    []any{workerSupervisor.taskRepository, message.TaskId},
		}); err != nil {
			workerSupervisor.Log().Info("Start new actor %v failed: %v", message.WorkerName, err.Error())
		}
		workerSupervisor.availableWorkerMap[message.WorkerName] = false
		workerSupervisor.Log().Info("Start new actor %v successful", message.WorkerName)
	}
}

func (workerSupervisor *WorkerSupervisor) runTask(message types.RunTaskMessage) {
	for workerName := range workerSupervisor.availableWorkerMap {
		if workerSupervisor.availableWorkerMap[workerName] {
			workerSupervisor.Log().Info("Restart existed actor %v", workerName)
			if err := workerSupervisor.StartChild(gen.Atom(workerName), workerSupervisor.taskRepository, message.TaskId); err != nil {
				workerSupervisor.Log().Error("Restart existed actor %v failed: %v", workerName, err.Error())
			}
			workerSupervisor.availableWorkerMap[workerName] = false
			workerSupervisor.Log().Info("Restart existed actor %v successful", workerName)

			return
		}
	}

	var workerName string
	for i := len(workerSupervisor.availableWorkerMap) + 1; ; i++ {
		workerName = fmt.Sprintf("worker_%v", i)
		if _, ok := workerSupervisor.availableWorkerMap[workerName]; !ok {
			break
		}
	}
	workerSupervisor.Log().Info("Start new actor %v", workerName)
	if err := workerSupervisor.AddChild(act.SupervisorChildSpec{
		Name:    gen.Atom(workerName),
		Factory: FactoryWorkerActor,
		Options: gen.ProcessOptions{},
		Args:    []any{workerSupervisor.taskRepository, message.TaskId},
	}); err != nil {
		workerSupervisor.Log().Info("Start new actor %v failed: %v", workerName, err.Error())
	}
	workerSupervisor.availableWorkerMap[workerName] = false
	workerSupervisor.Log().Info("Start new actor %v successful", workerName)
}

func (workerSupervisor *WorkerSupervisor) runTasks(message types.RunTasksMessage) {
	for _, taskId := range message.TaskIds {
		sendingMessage := types.RunTaskMessage{
			TaskId: taskId,
		}
		workerSupervisor.runTask(sendingMessage)
	}
}
