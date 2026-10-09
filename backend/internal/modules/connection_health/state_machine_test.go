package connection_health

import (
	"fmt"
	"testing"
	"time"
)

func testPolicy() Policy {
	return Policy{AutoSuspendEnabled: true, FailureThreshold: 5, SuccessThreshold: 2,
		CooldownSeconds: 300, ObservationSeconds: 300, RecoveryStepPercent: 25}
}

func nextTransitionInput(in TransitionInput, out TransitionOutput) TransitionInput {
	in.Current, in.CurrentWeight = out.NextState, out.Weight
	in.ConsecutiveFailures, in.ConsecutiveSuccesses = out.ConsecutiveFailures, out.ConsecutiveSuccesses
	return in
}

func TestTransition_AllFailuresActOnlyAtThreshold(t *testing.T) {
	for _, result := range []ResultKey{ResultNetworkFluctuation, ResultRateLimited, ResultInvalidResponse, ResultServerError, ResultAuth, ResultModelNotFound} {
		for _, threshold := range []int{1, 5, 8} {
			for _, allowSuspend := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/threshold%d/suspend%t", result, threshold, allowSuspend), func(t *testing.T) {
					policy := testPolicy()
					policy.FailureThreshold, policy.AutoSuspendEnabled = threshold, allowSuspend
					in := TransitionInput{Current: StateHealthy, CurrentWeight: 100, Result: result, Policy: policy}
					for attempt := 1; attempt <= threshold+1; attempt++ {
						out := Transition(in)
						wantState, wantWeight := StateHealthy, 100
						if attempt >= threshold {
							wantState = StateDegraded
							if allowSuspend {
								wantState, wantWeight = StateSuspended, 0
							}
						}
						if out.NextState != wantState || out.Weight != wantWeight || out.ConsecutiveFailures != attempt || out.ConsecutiveSuccesses != 0 {
							t.Fatalf("attempt %d: got %+v, want %s/%d", attempt, out, wantState, wantWeight)
						}
						if out.TriggerRemoteDegrade != (allowSuspend && attempt == threshold) || out.TriggerRemoteRestore || out.CooldownUntil != nil || out.ObservingUntil != nil {
							t.Fatalf("unexpected action or timer on attempt %d: %+v", attempt, out)
						}
						in = nextTransitionInput(in, out)
					}
				})
			}
		}
	}
}

func TestTransition_RecoveryIsImmediateAtSuccessThreshold(t *testing.T) {
	for _, state := range []State{StateSuspended, StateDegraded, StateObserving, StateRecovering} {
		for _, threshold := range []int{1, 3} {
			t.Run(fmt.Sprintf("%s/%d", state, threshold), func(t *testing.T) {
				policy := testPolicy()
				policy.SuccessThreshold = threshold
				future := time.Now().Add(time.Hour)
				in := TransitionInput{Current: state, CurrentWeight: 0, ConsecutiveFailures: 10,
					Result: ResultOK, Policy: policy, ObservingUntil: &future}
				for attempt := 1; attempt <= threshold; attempt++ {
					out := Transition(in)
					if out.ConsecutiveFailures != 0 || out.ConsecutiveSuccesses != attempt || out.CooldownUntil != nil || out.ObservingUntil != nil {
						t.Fatalf("unexpected counters or timer: %+v", out)
					}
					if attempt < threshold {
						if out.NextState == StateHealthy || out.TriggerRemoteRestore {
							t.Fatalf("restored too early: %+v", out)
						}
					} else if out.NextState != StateHealthy || out.Weight != 100 || !out.TriggerRemoteRestore {
						t.Fatalf("must restore fully at threshold: %+v", out)
					}
					in = nextTransitionInput(in, out)
				}
			})
		}
	}
}

func TestTransition_ConsecutiveCountersResetOnOppositeResult(t *testing.T) {
	policy := testPolicy()
	in := TransitionInput{Current: StateHealthy, CurrentWeight: 100, Policy: policy}
	// A success breaks the first four failures; mixed failure types share one counter.
	results := []ResultKey{ResultAuth, ResultServerError, ResultRateLimited, ResultNetworkFluctuation, ResultOK,
		ResultNetworkFluctuation, ResultAuth, ResultServerError, ResultInvalidResponse, ResultModelNotFound}
	for i, result := range results {
		in.Result = result
		out := Transition(in)
		if (out.NextState == StateSuspended) != (i == len(results)-1) {
			t.Fatalf("unexpected threshold at step %d: %+v", i, out)
		}
		if result == ResultOK && out.ConsecutiveFailures != 0 {
			t.Fatalf("success did not clear failures: %+v", out)
		}
		in = nextTransitionInput(in, out)
	}
	for i, result := range []ResultKey{ResultOK, ResultAuth, ResultOK, ResultOK} {
		in.Result = result
		out := Transition(in)
		if (out.NextState == StateHealthy) != (i == 3) {
			t.Fatalf("unexpected recovery at step %d: %+v", i, out)
		}
		if result != ResultOK && out.ConsecutiveSuccesses != 0 {
			t.Fatalf("failure did not clear successes: %+v", out)
		}
		in = nextTransitionInput(in, out)
	}
}

func TestTransition_UnsupportedDoesNotCountAndDisabledStaysManual(t *testing.T) {
	for _, state := range []State{StateHealthy, StateSuspended, StateDisabled} {
		for _, result := range []ResultKey{ResultOK, ResultAuth, ResultUnsupported} {
			in := TransitionInput{Current: state, CurrentWeight: 0, ConsecutiveFailures: 4, ConsecutiveSuccesses: 1, Policy: testPolicy(), Result: result}
			out := Transition(in)
			if state == StateDisabled || result == ResultUnsupported {
				if out.NextState != state || out.ConsecutiveFailures != 4 || out.ConsecutiveSuccesses != 1 || out.TriggerRemoteDegrade || out.TriggerRemoteRestore {
					t.Fatalf("must retain manual state/non-probe counters: %+v", out)
				}
			}
		}
	}
}
