package state

import (
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type State struct {
	Name         string
	Reconcile    func() (reconcile.Result, error)
	OnCompletion func() error
}

type transition struct {
	to        State
	predicate func() (bool, error)
}

type Machine struct {
	allTransitions     map[string][]transition
	currentTransitions []transition
	currentState       *State
	logger             *zap.SugaredLogger
	States             map[string]State
}

func NewStateMachine(logger *zap.SugaredLogger) *Machine {
	m := &Machine{
		allTransitions:     map[string][]transition{},
		currentTransitions: []transition{},
		logger:             logger,
		States:             map[string]State{},
	}
	return m
}

func (m *Machine) Reconcile() (reconcile.Result, error) {
	transition, err := m.getTransition()
	if err != nil {
		return reconcile.Result{}, err
	}

	if transition != nil {
		if err := m.SetState(transition.to); err != nil {
			return reconcile.Result{}, err
		}
	}

	if m.currentState == nil {
		panic("no current state!")
	}

	res, err := m.currentState.Reconcile()

	if m.currentState.OnCompletion != nil {
		if err := m.currentState.OnCompletion(); err != nil {
			m.logger.Errorf("error running OnCompletion for state %s: %s", m.currentState.Name, err)
			return reconcile.Result{}, err
		}
	}
	return res, err
}

func (m *Machine) SetState(state State) error {
	if m.currentState != nil && m.currentState.Name == state.Name {
		return nil
	}
	if m.currentState != nil {
		m.logger.Debugf("Transitioning from %s to %s.", m.currentState.Name, state.Name)
	} else {
		m.logger.Debugf("Setting starting state %s.", state.Name)
	}
	m.currentState = &state
	m.currentTransitions = m.allTransitions[m.currentState.Name]
	return nil
}

func (m *Machine) AddTransition(from, to State, predicate func() (bool, error)) {
	_, ok := m.allTransitions[from.Name]
	if !ok {
		m.allTransitions[from.Name] = []transition{}
	}
	m.allTransitions[from.Name] = append(m.allTransitions[from.Name], transition{
		to:        to,
		predicate: predicate,
	})

	m.States[from.Name] = from
	m.States[to.Name] = to

}

func (m *Machine) getTransition() (*transition, error) {
	for _, t := range m.currentTransitions {
		ok, err := t.predicate()
		if err != nil {
			return nil, err
		}
		if ok {
			return &t, nil
		}
	}
	return nil, nil
}