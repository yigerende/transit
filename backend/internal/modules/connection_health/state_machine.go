package connection_health

import "time"

// TransitionInput 是状态机做一次决策所需的全部输入：探活前的状态快照 + 本次探活结果 + 所属策略阈值。
// 不依赖任何 IO，纯函数，便于单测覆盖全部分支。
type TransitionInput struct {
	Current              State
	CurrentWeight        int
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	ObservingUntil       *time.Time
	Now                  time.Time
	Result               ResultKey
	Policy               Policy
}

// TransitionOutput 是状态机决策的结果：新状态 + 新权重 + 计数器 + 是否需要触发远端降级/恢复动作。
type TransitionOutput struct {
	NextState            State
	Weight               int
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	CooldownUntil        *time.Time
	ObservingUntil       *time.Time
	TriggerRemoteDegrade bool
	TriggerRemoteRestore bool
}

// Every completed failure follows the same threshold, including transport errors,
// timeouts, authentication errors, unavailable models and server errors.
func isProbeFailure(result ResultKey) bool {
	switch result {
	case ResultServerError, ResultAuth, ResultModelNotFound,
		ResultNetworkFluctuation, ResultRateLimited, ResultInvalidResponse:
		return true
	default:
		return false
	}
}

// Transition changes health/actions only at the configured consecutive thresholds.
// Disabled remains manual. Suspension never changes the probe interval.
func Transition(in TransitionInput) TransitionOutput {
	current := stateWithoutSuspension(ConnectionHealthState{State: in.Current, CurrentWeight: in.CurrentWeight}, in.Policy)
	out := TransitionOutput{
		NextState: current.State, Weight: current.CurrentWeight,
		ConsecutiveFailures: in.ConsecutiveFailures, ConsecutiveSuccesses: in.ConsecutiveSuccesses,
	}
	if current.State == StateDisabled {
		return out
	}
	switch {
	case in.Result == ResultOK:
		out.ConsecutiveFailures = 0
		out.ConsecutiveSuccesses++
		if current.State == StateHealthy || out.ConsecutiveSuccesses >= successThreshold(in.Policy) {
			out.NextState = StateHealthy
			out.Weight = 100
			out.TriggerRemoteRestore = in.Policy.AutoSuspendEnabled && current.State != StateHealthy
		}
	case isProbeFailure(in.Result):
		out.ConsecutiveSuccesses = 0
		out.ConsecutiveFailures++
		if current.State == StateSuspended || out.ConsecutiveFailures >= failureThreshold(in.Policy) {
			out.NextState = StateDegraded
			out.Weight = 100
			if in.Policy.AutoSuspendEnabled {
				out.NextState = StateSuspended
				out.Weight = 0
				out.TriggerRemoteDegrade = current.State != StateSuspended
			}
		}
	}
	return out
}

// Normalize old snapshots so obsolete timers and partial weights cannot delay a
// probe or bypass a threshold. Old observation waits for the success threshold;
// old gradual recovery keeps degraded health until that threshold is reached.
func stateWithoutSuspension(state ConnectionHealthState, policy Policy) ConnectionHealthState {
	state.CooldownUntil = nil
	state.ObservingUntil = nil
	if state.State == StateDisabled {
		state.CurrentWeight = 0
		return state
	}
	switch state.State {
	case StateObserving:
		state.State = StateSuspended
	case StateRecovering:
		state.State = StateDegraded
	}
	if state.State == StateSuspended && !policy.AutoSuspendEnabled {
		state.State = StateDegraded
	}
	state.CurrentWeight = 100
	if state.State == StateSuspended {
		state.CurrentWeight = 0
	}
	return state
}

func successThreshold(p Policy) int {
	return defaultInt(p.SuccessThreshold, 2)
}

func failureThreshold(p Policy) int {
	return defaultInt(p.FailureThreshold, 3)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
